package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type MappingStore struct {
	db *sql.DB
}

func NewMappingStore(filePath ...string) (*MappingStore, error) {
	var dbPath string
	if len(filePath) > 0 {
		// Join all provided path components
		dbPath = filepath.Join(filePath...)
	} else {
		// Get the current working directory
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("error getting current working directory: %v", err)
		}
		dbPath = filepath.Join(currentDir, "fasta_sanitiser_mapping.db")
	}

	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error creating directory: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS mapping (
        new_id TEXT PRIMARY KEY,
        original_header TEXT
    )`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("error creating table: %v", err)
	}

	return &MappingStore{db: db}, nil
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
	defer rows.Close()

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
