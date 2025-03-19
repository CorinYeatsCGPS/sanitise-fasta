package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type MappingStore struct {
	db *sql.DB
}

func NewMappingStore(storeLocation string, readOnly bool) (*MappingStore, error) {
	if storeLocation == "" {
		storeLocation = "mapping_store.db"
	}

	var db *sql.DB
	var err error

	if readOnly {
		db, err = openReadOnlyDB(storeLocation)
	} else {
		db, err = openWriteDB(storeLocation)
	}

	if err != nil {
		return nil, err
	}

	return &MappingStore{db: db}, nil
}

func openWriteDB(storeLocation string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", storeLocation)
	if err != nil {
		return nil, err
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
			closeErr := db.Close()
			if closeErr != nil {
				return nil, fmt.Errorf("failed to set %s and close DB: %v, %v", opt, err, closeErr)
			}
			return nil, fmt.Errorf("failed to set %s: %v", opt, err)
		}
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS mappings (
        new_id TEXT PRIMARY KEY,
        original_id TEXT
    )`)
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("failed to create table and close DB: %v, %v", err, closeErr)
		}
		return nil, err
	}

	return db, nil
}

func openReadOnlyDB(storeLocation string) (*sql.DB, error) {
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
	return ms.db.Close()
}

func (ms *MappingStore) StorePair(newID, originalHeader string) error {
	_, err := ms.db.Exec("INSERT INTO mapping (new_id, original_header) VALUES (?, ?)", newID, originalHeader)
	if err != nil {
		return fmt.Errorf("error inserting mapping: %v", err)
	}
	return nil
}

func (ms *MappingStore) ReadAllMappings() (map[string]string, error) {
	mapping := make(map[string]string)
	rows, err := ms.db.Query("SELECT new_id, original_header FROM mapping")
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
		var newID, originalHeader string
		err := rows.Scan(&newID, &originalHeader)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}
		mapping[newID] = originalHeader
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return mapping, nil
}

func (ms *MappingStore) LookupOriginalID(processedID string) (string, error) {
	var originalID string
	err := ms.db.QueryRow("SELECT original_header FROM mapping WHERE new_id = ?", processedID).Scan(&originalID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no mapping found for processed ID: %s", processedID)
		}
		return "", fmt.Errorf("error looking up original ID: %v", err)
	}
	return originalID, nil
}
func (ms *MappingStore) Finalise() error {
	// Create index
	_, err := ms.db.Exec("CREATE INDEX IF NOT EXISTS idx_new_id ON mappings(new_id)")
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
