package operation

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
)

// OperationLog represents a log of operations
type OperationLog struct {
	mu       sync.RWMutex
	path     string
	ops      []*proto.Operation
	lastSync int64
}

// NewOperationLog creates a new operation log
func NewOperationLog(path string) (*OperationLog, error) {
	opLog := &OperationLog{
		path:     path,
		ops:      make([]*proto.Operation, 0),
		lastSync: time.Now().UnixNano(),
	}

	// Load existing operations
	if err := opLog.load(); err != nil {
		return nil, fmt.Errorf("failed to load operation log: %v", err)
	}

	return opLog, nil
}

// AddOperation adds an operation to the log
func (o *OperationLog) AddOperation(op *proto.Operation) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Add operation to memory
	o.ops = append(o.ops, op)

	// Save to disk
	return o.save()
}

// GetOperations returns all operations since the given timestamp
func (o *OperationLog) GetOperations(since int64) ([]*proto.Operation, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var ops []*proto.Operation
	for _, op := range o.ops {
		if op.Timestamp > since {
			ops = append(ops, op)
		}
	}

	return ops, nil
}

// load reads operations from disk
func (o *OperationLog) load() error {
	data, err := ioutil.ReadFile(o.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize with empty array
			if err := ioutil.WriteFile(o.path, []byte("[]"), 0644); err != nil {
				return fmt.Errorf("failed to initialize log file: %v", err)
			}
			return nil
		}
		return fmt.Errorf("failed to read log file: %v", err)
	}

	// if data is not empty, unmarshal
	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, &o.ops); err != nil {
		return fmt.Errorf("failed to unmarshal operations: %v", err)
	}

	return nil
}

// save writes operations to disk
func (o *OperationLog) save() error {
	data, err := json.Marshal(o.ops)
	if err != nil {
		return fmt.Errorf("failed to marshal operations: %v", err)
	}

	if err := ioutil.WriteFile(o.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write log file: %v", err)
	}

	return nil
}

// Close closes the operation log
func (o *OperationLog) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.save()
}

var replicaID string
var replicaOnce sync.Once

// GetReplicaID returns a unique replica ID
func GetReplicaID() string {
	replicaOnce.Do(func() {
		// Use hostname and timestamp to generate a unique ID
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		replicaID = fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())
	})
	return replicaID
}
