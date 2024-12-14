package operation

import (
	"os"
	"testing"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
)

func TestGetReplicaID(t *testing.T) {
	id1 := GetReplicaID()
	id2 := GetReplicaID()

	if id1 != id2 {
		t.Errorf("GetReplicaID() should return same ID. Got %s and %s", id1, id2)
	}

	if id1 == "" {
		t.Error("GetReplicaID() returned empty string")
	}
}

func TestOperationLog(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "oplog-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Create operation log
	opLog, err := NewOperationLog(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create operation log: %v", err)
	}
	defer opLog.Close()

	// Create test operation
	op := &proto.Operation{
		Type:      proto.OperationType_SET,
		Args:      []string{"key1", "value1"},
		Timestamp: time.Now().UnixNano(),
		ReplicaId: GetReplicaID(),
	}

	// Add operation
	if err := opLog.AddOperation(op); err != nil {
		t.Errorf("Failed to add operation: %v", err)
	}

	// Get operations
	ops, err := opLog.GetOperations(0)
	if err != nil {
		t.Errorf("Failed to get operations: %v", err)
	}

	if len(ops) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(ops))
	}

	if ops[0].Type != op.Type {
		t.Errorf("Expected operation type %v, got %v", op.Type, ops[0].Type)
	}
}
