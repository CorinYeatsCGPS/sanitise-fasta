package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type MappingStore struct {
	db   *sql.DB
	tx   *sql.Tx
	stmt *sql.Stmt
}

func NewMappingStore(storeLocation string, readOnly bool) (*MappingStore, error) {
	if storeLocation == "" {
		storeLocation = "mapping_store.db"
	}

	var db *sql.DB
	var err error
	var tx *sql.Tx
	var stmt *sql.Stmt

	if readOnly {
		db, err = openReadOnlyDB(storeLocation)
	} else {
		db, tx, stmt, err = openWriteDB(storeLocation)
	}

	if err != nil {
		return nil, err
	}

	return &MappingStore{db: db, tx: tx, stmt: stmt}, nil
}

func openWriteDB(storeLocation string) (*sql.DB, *sql.Tx, *sql.Stmt, error) {
	db, err := sql.Open("sqlite3", storeLocation)
	if err != nil {
		return nil, nil, nil, err
	}

	// Performance optimizations for write mode
	optimizations := []string{
		"PRAGMA journal_mode=OFF;",
		"PRAGMA synchronous=OFF;",
		"PRAGMA cache_size=1000000;",
		"PRAGMA locking_mode=EXCLUSIVE;",
		"PRAGMA temp_store=MEMORY;",
	}

	for _, opt := range optimizations {
		_, err = db.Exec(opt)
		if err != nil {
			db.Close()
			return nil, nil, nil, fmt.Errorf("failed to set %s: %v", opt, err)
		}
	}

	// Drop the existing table if it exists and create a new one
	_, err = db.Exec(`DROP TABLE IF EXISTS mappings`)
	if err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("failed to drop existing table: %v", err)
	}

	_, err = db.Exec("CREATE TABLE mappings (new_id TEXT PRIMARY KEY, original_id TEXT)")
	if err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("failed to create table: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("failed to begin transaction: %v", err)
	}

	stmt, err := tx.Prepare("INSERT INTO mappings (new_id, original_id) VALUES (?, ?)")
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, nil, nil, fmt.Errorf("failed to prepare statement: %v", err)
	}

	return db, tx, stmt, nil
}

func openReadOnlyDB(storeLocation string) (*sql.DB, error) {
	// ... (keep this function as it is)
	db, err := sql.Open("sqlite3", storeLocation+"?mode=ro")
	if err != nil {
		return nil, err
	}

	// Performance optimizations for read-only mode
	optimizations := []string{
		"PRAGMA journal_mode=OFF;",
		"PRAGMA synchronous=OFF;",
		"PRAGMA cache_size=1000000;",
		"PRAGMA locking_mode=NORMAL;",
		"PRAGMA temp_store=MEMORY;",
		"PRAGMA mmap_size=52428800;", // 50MB in bytes (50 * 1024 * 1024)
	}

	for _, opt := range optimizations {
		_, err = db.Exec(opt)
		if err != nil {
			err := db.Close()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("failed to set %s: %v", opt, err)
		}
	}

	return db, nil
}

func (ms *MappingStore) Close() error {
	if ms.stmt != nil {
		ms.stmt.Close()
	}
	if ms.tx != nil {
		ms.tx.Rollback()
	}
	return ms.db.Close()
}

func (ms *MappingStore) StorePair(newID, originalHeader string) error {
	if ms.stmt == nil {
		return fmt.Errorf("database not in write mode")
	}

	_, err := ms.stmt.Exec(newID, originalHeader)
	if err != nil {
		return fmt.Errorf("error inserting mapping: %v", err)
	}
	return nil
}

func (ms *MappingStore) ReadAllMappings() (map[string]string, error) {
	mapping := make(map[string]string)
	rows, err := ms.db.Query("SELECT new_id, original_id FROM mappings")
	if err != nil {
		return nil, fmt.Errorf("error querying database: %v", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			err = fmt.Errorf("error closing rows: %v", closeErr)
		}
	}()

	for rows.Next() {
		var newID, originalID string
		err := rows.Scan(&newID, &originalID)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}
		mapping[newID] = originalID
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return mapping, err
}

func (ms *MappingStore) LookupOriginalID(processedID string) (string, error) {
	var originalID string
	err := ms.db.QueryRow("SELECT original_id FROM mappings WHERE new_id = ?", processedID).Scan(&originalID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no mapping found for processed ID: %s", processedID)
		}
		return "", fmt.Errorf("error looking up original ID: %v", err)
	}
	return originalID, nil
}

func (ms *MappingStore) Finalise() error {
	if ms.tx == nil {
		return nil // Nothing to finalize in read-only mode
	}

	if ms.stmt != nil {
		ms.stmt.Close()
		ms.stmt = nil
	}

	err := ms.tx.Commit()
	if err != nil {
		ms.tx.Rollback()
		return fmt.Errorf("error committing transaction: %v", err)
	}
	ms.tx = nil

	// Create index
	_, err = ms.db.Exec("CREATE INDEX IF NOT EXISTS idx_new_id ON mappings(new_id)")
	if err != nil {
		return fmt.Errorf("error creating index: %v", err)
	}

	// Analyze table
	_, err = ms.db.Exec("ANALYZE mappings")
	if err != nil {
		return fmt.Errorf("error analyzing table: %v", err)
	}
	return nil
}
