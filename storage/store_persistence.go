package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// loadFromSegments loads CRDT state from segment manager
func (s *Store) loadFromSegments() error {
	// Try to load from segments first
	items, err := s.segmentManager.LoadAllEntries()
	if err != nil {
		return fmt.Errorf("failed to load from segments: %v", err)
	}

	// If no segments exist, try legacy load
	if len(items) == 0 {
		return s.loadLegacy()
	}

	// Load items into memory
	for key, value := range items {
		s.items[key] = value
	}

	return nil
}

// loadLegacy loads from the legacy JSON file format
func (s *Store) loadLegacy() error {
	data, err := os.ReadFile(s.dataPath)
	if os.IsNotExist(err) {
		return nil // No existing data
	}
	if err != nil {
		return fmt.Errorf("failed to read data file: %v", err)
	}

	if err := json.Unmarshal(data, &s.items); err != nil {
		return fmt.Errorf("failed to unmarshal data: %v", err)
	}

	// Migrate to segments
	return s.migrateToSegments()
}

// migrateToSegments migrates legacy data to segment format
func (s *Store) migrateToSegments() error {
	timestamp := time.Now().UnixNano()

	for key, value := range s.items {
		entry := &LogEntry{
			Timestamp: timestamp,
			Operation: "SET",
			Key:       key,
			Value:     value,
		}

		if err := s.segmentManager.WriteEntry(entry); err != nil {
			return fmt.Errorf("failed to migrate key %s: %v", key, err)
		}
	}

	return nil
}

// persistSet writes a SET operation to the segment log
func (s *Store) persistSet(key string, value *Value) error {
	entry := &LogEntry{
		Timestamp: time.Now().UnixNano(),
		Operation: "SET",
		Key:       key,
		Value:     value,
	}

	return s.segmentManager.WriteEntry(entry)
}

// persistDelete writes a DELETE operation to the segment log
func (s *Store) persistDelete(key string) error {
	entry := &LogEntry{
		Timestamp: time.Now().UnixNano(),
		Operation: "DELETE",
		Key:       key,
	}

	return s.segmentManager.WriteEntry(entry)
}

// GetPersistenceStats returns statistics about the persistence layer
func (s *Store) GetPersistenceStats() map[string]interface{} {
	return s.segmentManager.GetStats()
}
