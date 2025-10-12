package main

import (
	"testing"

	"github.com/luoyjx/crdt-redis/storage"
)

func TestVectorClockBasicOperations(t *testing.T) {
	// Test creation and increment
	vc := storage.NewVectorClock()
	vc.Increment("server1")
	vc.Increment("server1")
	vc.Increment("server2")

	if vc.GetTime("server1") != 2 {
		t.Errorf("Expected server1 time to be 2, got %d", vc.GetTime("server1"))
	}
	if vc.GetTime("server2") != 1 {
		t.Errorf("Expected server2 time to be 1, got %d", vc.GetTime("server2"))
	}

	// Test string representation
	str := vc.String()
	t.Logf("Vector clock string: %s", str)
	if str != "{server1:2,server2:1}" {
		t.Errorf("Unexpected string representation: %s", str)
	}
}

func TestVectorClockComparison(t *testing.T) {
	// Create two vector clocks
	vc1 := storage.NewVectorClock()
	vc1.Increment("server1")
	vc1.Increment("server2")

	vc2 := storage.NewVectorClock()
	vc2.Increment("server1")

	// vc1 should happen after vc2
	if !vc1.HappensAfter(vc2) {
		t.Error("vc1 should happen after vc2")
	}
	if !vc2.HappensBefore(vc1) {
		t.Error("vc2 should happen before vc1")
	}

	// Test concurrent clocks
	vc3 := storage.NewVectorClock()
	vc3.Increment("server2")
	vc3.Increment("server3")

	if !vc2.IsConcurrent(vc3) {
		t.Error("vc2 and vc3 should be concurrent")
	}

	// Test equality
	vc4 := storage.NewVectorClock()
	vc4.Increment("server1")

	if !vc2.Equal(vc4) {
		t.Error("vc2 and vc4 should be equal")
	}
}

func TestVectorClockMerge(t *testing.T) {
	vc1 := storage.NewVectorClock()
	vc1.Increment("server1")
	vc1.Increment("server1")
	vc1.Increment("server2")

	vc2 := storage.NewVectorClock()
	vc2.Increment("server1")
	vc2.Increment("server3")
	vc2.Increment("server3")

	// Merge vc2 into vc1
	vc1.Update(vc2)

	// Check merged result
	if vc1.GetTime("server1") != 2 { // max(2, 1) = 2
		t.Errorf("Expected server1 time to be 2, got %d", vc1.GetTime("server1"))
	}
	if vc1.GetTime("server2") != 1 { // max(1, 0) = 1
		t.Errorf("Expected server2 time to be 1, got %d", vc1.GetTime("server2"))
	}
	if vc1.GetTime("server3") != 2 { // max(0, 2) = 2
		t.Errorf("Expected server3 time to be 2, got %d", vc1.GetTime("server3"))
	}

	t.Logf("Merged vector clock: %s", vc1.String())
}

func TestVectorClockCopy(t *testing.T) {
	original := storage.NewVectorClock()
	original.Increment("server1")
	original.Increment("server2")

	copy := original.Copy()

	// Modify original
	original.Increment("server1")

	// Copy should not be affected
	if copy.GetTime("server1") != 1 {
		t.Errorf("Copy should not be affected by original modification")
	}
	if original.GetTime("server1") != 2 {
		t.Errorf("Original should be modified")
	}
}

func TestVectorClockSerialization(t *testing.T) {
	vc := storage.NewVectorClock()
	vc.Increment("server1")
	vc.Increment("server2")
	vc.Increment("server2")

	// Serialize to JSON
	data, err := vc.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize vector clock: %v", err)
	}

	// Deserialize from JSON
	newVC := storage.NewVectorClock()
	err = newVC.FromJSON(data)
	if err != nil {
		t.Fatalf("Failed to deserialize vector clock: %v", err)
	}

	// Check equality
	if !vc.Equal(newVC) {
		t.Error("Deserialized vector clock should equal original")
	}

	t.Logf("Original: %s, Deserialized: %s", vc.String(), newVC.String())
}

func TestVectorClockCausalityScenarios(t *testing.T) {
	// Scenario 1: Sequential operations on same replica
	vc1 := storage.NewVectorClock()
	vc1.Increment("server1") // {server1:1}

	vc2 := vc1.Copy()
	vc2.Increment("server1") // {server1:2}

	if !vc1.HappensBefore(vc2) {
		t.Error("Sequential operations should have happens-before relationship")
	}

	// Scenario 2: Operations on different replicas (concurrent)
	vc3 := storage.NewVectorClock()
	vc3.Increment("server1") // {server1:1}

	vc4 := storage.NewVectorClock()
	vc4.Increment("server2") // {server2:1}

	if !vc3.IsConcurrent(vc4) {
		t.Error("Operations on different replicas should be concurrent")
	}

	// Scenario 3: After synchronization
	vc5 := vc3.Copy()
	vc5.Update(vc4)          // {server1:1, server2:1}
	vc5.Increment("server1") // {server1:2, server2:1}

	if !vc5.HappensAfter(vc3) || !vc5.HappensAfter(vc4) {
		t.Error("After sync and increment, should happen after both original clocks")
	}

	t.Logf("Causality test results:")
	t.Logf("  vc1: %s", vc1.String())
	t.Logf("  vc2: %s", vc2.String())
	t.Logf("  vc3: %s", vc3.String())
	t.Logf("  vc4: %s", vc4.String())
	t.Logf("  vc5: %s", vc5.String())
}

