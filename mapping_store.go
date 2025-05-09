package main

import (
	"fmt"
	"github.com/dgraph-io/badger/v3"
)

type MappingStore struct {
	db *badger.DB
}

func NewMappingStore(location string, readOnly bool) (*MappingStore, error) {
	if location == "" {
		location = "mapping_store"
	}

	opts := badger.DefaultOptions(location)
	opts.ReadOnly = readOnly

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %v", err)
	}

	return &MappingStore{db: db}, nil
}

func (ms *MappingStore) StorePair(newID, originalID string) error {
	return ms.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(newID), []byte(originalID))
	})
}

func (ms *MappingStore) LookupOriginalID(newID string) (string, error) {
	var originalID string
	err := ms.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(newID))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			originalID = string(val)
			return nil
		})
	})
	if err != nil {
		return "", err
	}
	return originalID, nil
}

func (ms *MappingStore) Close() error {
	return ms.db.Close()
}

func (ms *MappingStore) Finalise() error {
	// BadgerDB doesn't require explicit finalization like SQLite
	return nil
}
