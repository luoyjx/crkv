package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LogEntry represents a single entry in the append-only log
type LogEntry struct {
	Timestamp int64  `json:"timestamp"`
	Operation string `json:"operation"` // SET, DELETE, MERGE
	Key       string `json:"key"`
	Value     *Value `json:"value,omitempty"`
	Metadata  string `json:"metadata,omitempty"` // Additional metadata if needed
}

// SegmentManager manages append-only log segments for efficient persistence
type SegmentManager struct {
	mu                  sync.RWMutex
	dataDir             string
	currentSegment      *os.File
	currentSegmentID    int64
	maxSegmentSize      int64         // Maximum size per segment in bytes
	compactionThreshold int           // Number of segments before compaction
	segments            []string      // List of segment file paths
	lastCompaction      time.Time     // Last compaction time
	compactionInterval  time.Duration // Minimum interval between compactions
}

// NewSegmentManager creates a new segment manager
func NewSegmentManager(dataDir string) (*SegmentManager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	sm := &SegmentManager{
		dataDir:             dataDir,
		maxSegmentSize:      64 * 1024 * 1024, // 64MB per segment
		compactionThreshold: 10,               // Compact after 10 segments
		compactionInterval:  5 * time.Minute,  // Compact at most every 5 minutes
		lastCompaction:      time.Now(),
	}

	// Load existing segments
	if err := sm.loadSegments(); err != nil {
		return nil, fmt.Errorf("failed to load segments: %v", err)
	}

	// Open current segment for writing
	if err := sm.openCurrentSegment(); err != nil {
		return nil, fmt.Errorf("failed to open current segment: %v", err)
	}

	return sm, nil
}

// loadSegments discovers and loads existing segment files
func (sm *SegmentManager) loadSegments() error {
	files, err := os.ReadDir(sm.dataDir)
	if err != nil {
		return err
	}

	var segmentFiles []string
	var maxID int64 = -1

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if strings.HasPrefix(name, "segment-") && strings.HasSuffix(name, ".log") {
			// Extract segment ID
			idStr := strings.TrimPrefix(name, "segment-")
			idStr = strings.TrimSuffix(idStr, ".log")

			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				continue // Skip invalid segment files
			}

			segmentPath := filepath.Join(sm.dataDir, name)
			segmentFiles = append(segmentFiles, segmentPath)

			if id > maxID {
				maxID = id
			}
		}
	}

	// Sort segments by filename (which includes the ID)
	sort.Strings(segmentFiles)
	sm.segments = segmentFiles

	// Set next segment ID
	sm.currentSegmentID = maxID + 1

	return nil
}

// openCurrentSegment opens or creates the current segment for writing
func (sm *SegmentManager) openCurrentSegment() error {
	segmentPath := filepath.Join(sm.dataDir, fmt.Sprintf("segment-%d.log", sm.currentSegmentID))

	file, err := os.OpenFile(segmentPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	sm.currentSegment = file

	// Add to segments list if not already present
	found := false
	for _, seg := range sm.segments {
		if seg == segmentPath {
			found = true
			break
		}
	}
	if !found {
		sm.segments = append(sm.segments, segmentPath)
	}

	return nil
}

// WriteEntry writes a log entry to the current segment
func (sm *SegmentManager) WriteEntry(entry *LogEntry) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Serialize entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %v", err)
	}

	// Add newline for easier parsing
	data = append(data, '\n')

	// Write to current segment
	_, err = sm.currentSegment.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write entry: %v", err)
	}

	// Sync to disk for durability
	if err := sm.currentSegment.Sync(); err != nil {
		return fmt.Errorf("failed to sync segment: %v", err)
	}

	// Check if we need to rotate segment
	if err := sm.checkSegmentRotation(); err != nil {
		return fmt.Errorf("failed to check segment rotation: %v", err)
	}

	// Check if we need compaction
	sm.checkCompaction()

	return nil
}

// checkSegmentRotation rotates to a new segment if current one is too large
func (sm *SegmentManager) checkSegmentRotation() error {
	stat, err := sm.currentSegment.Stat()
	if err != nil {
		return err
	}

	if stat.Size() >= sm.maxSegmentSize {
		return sm.rotateSegment()
	}

	return nil
}

// rotateSegment closes current segment and opens a new one
func (sm *SegmentManager) rotateSegment() error {
	// Close current segment
	if err := sm.currentSegment.Close(); err != nil {
		return err
	}

	// Increment segment ID and open new segment
	sm.currentSegmentID++
	return sm.openCurrentSegment()
}

// checkCompaction checks if compaction should be performed
func (sm *SegmentManager) checkCompaction() {
	// Don't compact too frequently
	if time.Since(sm.lastCompaction) < sm.compactionInterval {
		return
	}

	// Don't compact if we don't have enough segments
	if len(sm.segments) < sm.compactionThreshold {
		return
	}

	// Perform compaction in background
	go func() {
		if err := sm.performCompaction(); err != nil {
			// Log error but don't fail the write operation
			fmt.Printf("Compaction failed: %v\n", err)
		}
	}()
}

// performCompaction merges old segments and removes duplicates
func (sm *SegmentManager) performCompaction() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Don't compact the current segment
	segmentsToCompact := sm.segments[:len(sm.segments)-1]
	if len(segmentsToCompact) < 2 {
		return nil // Nothing to compact
	}

	// Load all entries from segments to compact
	allEntries := make(map[string]*LogEntry) // Key -> Latest entry
	var orderedKeys []string

	for _, segmentPath := range segmentsToCompact {
		entries, err := sm.readSegment(segmentPath)
		if err != nil {
			return fmt.Errorf("failed to read segment %s: %v", segmentPath, err)
		}

		for _, entry := range entries {
			// Keep only the latest entry for each key
			if existing, exists := allEntries[entry.Key]; !exists || entry.Timestamp > existing.Timestamp {
				if !exists {
					orderedKeys = append(orderedKeys, entry.Key)
				}
				allEntries[entry.Key] = entry
			}
		}
	}

	// Write compacted data to new segment
	compactedPath := filepath.Join(sm.dataDir, fmt.Sprintf("segment-%d-compacted.log", time.Now().UnixNano()))
	compactedFile, err := os.Create(compactedPath)
	if err != nil {
		return fmt.Errorf("failed to create compacted segment: %v", err)
	}
	defer compactedFile.Close()

	// Write entries in order
	for _, key := range orderedKeys {
		entry := allEntries[key]
		// Skip deleted entries (no value)
		if entry.Operation == "DELETE" {
			continue
		}

		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal compacted entry: %v", err)
		}
		data = append(data, '\n')

		if _, err := compactedFile.Write(data); err != nil {
			return fmt.Errorf("failed to write compacted entry: %v", err)
		}
	}

	if err := compactedFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync compacted segment: %v", err)
	}

	// Remove old segments
	for _, segmentPath := range segmentsToCompact {
		if err := os.Remove(segmentPath); err != nil {
			fmt.Printf("Warning: failed to remove old segment %s: %v\n", segmentPath, err)
		}
	}

	// Update segments list
	newSegments := []string{compactedPath}
	if len(sm.segments) > len(segmentsToCompact) {
		// Keep the current segment
		newSegments = append(newSegments, sm.segments[len(segmentsToCompact):]...)
	}
	sm.segments = newSegments

	sm.lastCompaction = time.Now()
	return nil
}

// readSegment reads all entries from a segment file
func (sm *SegmentManager) readSegment(segmentPath string) ([]*LogEntry, error) {
	file, err := os.Open(segmentPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []*LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip invalid entries but log the error
			fmt.Printf("Warning: failed to unmarshal entry in %s: %v\n", segmentPath, err)
			continue
		}

		entries = append(entries, &entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading segment: %v", err)
	}

	return entries, nil
}

// LoadAllEntries loads all entries from all segments
func (sm *SegmentManager) LoadAllEntries() (map[string]*Value, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]*Value)

	// Read all segments in order
	for _, segmentPath := range sm.segments {
		entries, err := sm.readSegment(segmentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read segment %s: %v", segmentPath, err)
		}

		// Apply entries in order
		for _, entry := range entries {
			switch entry.Operation {
			case "SET", "MERGE":
				if entry.Value != nil {
					result[entry.Key] = entry.Value
				}
			case "DELETE":
				delete(result, entry.Key)
			}
		}
	}

	return result, nil
}

// Close closes the segment manager and current segment
func (sm *SegmentManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentSegment != nil {
		return sm.currentSegment.Close()
	}

	return nil
}

// GetStats returns statistics about the segment manager
func (sm *SegmentManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_segments":       len(sm.segments),
		"current_segment_id":   sm.currentSegmentID,
		"max_segment_size":     sm.maxSegmentSize,
		"compaction_threshold": sm.compactionThreshold,
		"last_compaction":      sm.lastCompaction,
	}

	// Calculate total size
	var totalSize int64
	for _, segmentPath := range sm.segments {
		if stat, err := os.Stat(segmentPath); err == nil {
			totalSize += stat.Size()
		}
	}
	stats["total_size_bytes"] = totalSize

	return stats
}
