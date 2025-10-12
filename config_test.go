package main

import (
	"os"
	"testing"
	"time"

	"github.com/luoyjx/crdt-redis/config"
)

func TestConfigDefaults(t *testing.T) {
	cfg := config.DefaultConfig()

	// Test default values
	if cfg.ServerPort != 6379 {
		t.Errorf("Expected default server port 6379, got %d", cfg.ServerPort)
	}

	if cfg.HTTPPort != 8080 {
		t.Errorf("Expected default HTTP port 8080, got %d", cfg.HTTPPort)
	}

	if cfg.DataDir != "./data" {
		t.Errorf("Expected default data dir './data', got %s", cfg.DataDir)
	}

	if cfg.SyncInterval != 5*time.Second {
		t.Errorf("Expected default sync interval 5s, got %s", cfg.SyncInterval)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", cfg.LogLevel)
	}

	if cfg.ClusterName != "crdt-redis-cluster" {
		t.Errorf("Expected default cluster name 'crdt-redis-cluster', got %s", cfg.ClusterName)
	}
}

func TestConfigValidation(t *testing.T) {
	// Test valid config
	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Test invalid server port
	cfg.ServerPort = 0
	if err := cfg.Validate(); err == nil {
		t.Error("Invalid server port should fail validation")
	}

	cfg.ServerPort = 70000
	if err := cfg.Validate(); err == nil {
		t.Error("Server port > 65535 should fail validation")
	}

	// Reset and test invalid HTTP port
	cfg = config.DefaultConfig()
	cfg.HTTPPort = -1
	if err := cfg.Validate(); err == nil {
		t.Error("Negative HTTP port should fail validation")
	}

	// Test empty replica ID
	cfg = config.DefaultConfig()
	cfg.ReplicaID = ""
	if err := cfg.Validate(); err == nil {
		t.Error("Empty replica ID should fail validation")
	}

	// Test invalid Redis DB
	cfg = config.DefaultConfig()
	cfg.RedisDB = 16
	if err := cfg.Validate(); err == nil {
		t.Error("Redis DB > 15 should fail validation")
	}

	// Test invalid discovery mode
	cfg = config.DefaultConfig()
	cfg.DiscoveryMode = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("Invalid discovery mode should fail validation")
	}

	// Test invalid log level
	cfg = config.DefaultConfig()
	cfg.LogLevel = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("Invalid log level should fail validation")
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// Create a temporary config file
	tempFile := "/tmp/test-config.json"
	defer os.Remove(tempFile)

	// Create and save config
	originalConfig := config.DefaultConfig()
	originalConfig.ServerPort = 9999
	originalConfig.ReplicaID = "test-replica"
	originalConfig.LogLevel = "debug"
	originalConfig.Peers = []string{"http://peer1:8080", "http://peer2:8080"}

	err := originalConfig.SaveToFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load config
	loadedConfig, err := config.LoadFromFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Compare values
	if loadedConfig.ServerPort != originalConfig.ServerPort {
		t.Errorf("Server port mismatch: expected %d, got %d", originalConfig.ServerPort, loadedConfig.ServerPort)
	}

	if loadedConfig.ReplicaID != originalConfig.ReplicaID {
		t.Errorf("Replica ID mismatch: expected %s, got %s", originalConfig.ReplicaID, loadedConfig.ReplicaID)
	}

	if loadedConfig.LogLevel != originalConfig.LogLevel {
		t.Errorf("Log level mismatch: expected %s, got %s", originalConfig.LogLevel, loadedConfig.LogLevel)
	}

	if len(loadedConfig.Peers) != len(originalConfig.Peers) {
		t.Errorf("Peers count mismatch: expected %d, got %d", len(originalConfig.Peers), len(loadedConfig.Peers))
	}

	for i, peer := range originalConfig.Peers {
		if i < len(loadedConfig.Peers) && loadedConfig.Peers[i] != peer {
			t.Errorf("Peer %d mismatch: expected %s, got %s", i, peer, loadedConfig.Peers[i])
		}
	}
}

func TestConfigEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("CRDT_REDIS_PORT", "7777")
	os.Setenv("CRDT_REPLICA_ID", "env-replica")
	os.Setenv("CRDT_LOG_LEVEL", "debug")
	os.Setenv("CRDT_PEERS", "http://env-peer1:8080,http://env-peer2:8080")

	defer func() {
		os.Unsetenv("CRDT_REDIS_PORT")
		os.Unsetenv("CRDT_REPLICA_ID")
		os.Unsetenv("CRDT_LOG_LEVEL")
		os.Unsetenv("CRDT_PEERS")
	}()

	// Load default config and apply env vars
	cfg := config.DefaultConfig()
	config.LoadFromEnv(cfg)

	// Check that env vars were applied
	if cfg.ServerPort != 7777 {
		t.Errorf("Environment variable not applied: expected port 7777, got %d", cfg.ServerPort)
	}

	if cfg.ReplicaID != "env-replica" {
		t.Errorf("Environment variable not applied: expected replica ID 'env-replica', got %s", cfg.ReplicaID)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Environment variable not applied: expected log level 'debug', got %s", cfg.LogLevel)
	}

	expectedPeers := []string{"http://env-peer1:8080", "http://env-peer2:8080"}
	if len(cfg.Peers) != len(expectedPeers) {
		t.Errorf("Environment peers not applied: expected %d peers, got %d", len(expectedPeers), len(cfg.Peers))
	}

	for i, expectedPeer := range expectedPeers {
		if i < len(cfg.Peers) && cfg.Peers[i] != expectedPeer {
			t.Errorf("Environment peer %d mismatch: expected %s, got %s", i, expectedPeer, cfg.Peers[i])
		}
	}
}

func TestConfigHelperMethods(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ServerPort = 9999
	cfg.HTTPPort = 8888

	// Test GetAddress
	expectedAddr := ":9999"
	if addr := cfg.GetAddress(); addr != expectedAddr {
		t.Errorf("GetAddress() mismatch: expected %s, got %s", expectedAddr, addr)
	}

	// Test GetHTTPAddress
	expectedHTTPAddr := ":8888"
	if addr := cfg.GetHTTPAddress(); addr != expectedHTTPAddr {
		t.Errorf("GetHTTPAddress() mismatch: expected %s, got %s", expectedHTTPAddr, addr)
	}

	// Test GetOpLogPath with relative path
	cfg.DataDir = "/test/data"
	cfg.OpLogPath = "oplog.json"
	expectedOpLogPath := "/test/data/oplog.json"
	if path := cfg.GetOpLogPath(); path != expectedOpLogPath {
		t.Errorf("GetOpLogPath() mismatch: expected %s, got %s", expectedOpLogPath, path)
	}

	// Test GetOpLogPath with absolute path
	cfg.OpLogPath = "/absolute/path/oplog.json"
	if path := cfg.GetOpLogPath(); path != cfg.OpLogPath {
		t.Errorf("GetOpLogPath() should return absolute path unchanged: expected %s, got %s", cfg.OpLogPath, path)
	}

	// Test GetPersistencePath
	expectedPersistencePath := "/test/data/store.json"
	if path := cfg.GetPersistencePath(); path != expectedPersistencePath {
		t.Errorf("GetPersistencePath() mismatch: expected %s, got %s", expectedPersistencePath, path)
	}
}

func TestConfigNonExistentFile(t *testing.T) {
	// Try to load a non-existent file
	_, err := config.LoadFromFile("/non/existent/file.json")
	if err == nil {
		t.Error("Loading non-existent file should return error")
	}
}

func TestConfigString(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ReplicaID = "test-string"

	str := cfg.String()
	if str == "" {
		t.Error("Config String() should not be empty")
	}

	// Should contain the replica ID
	if !contains(str, "test-string") {
		t.Error("Config String() should contain the replica ID")
	}

	t.Logf("Config string representation:\n%s", str)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr ||
		(len(s) > len(substr) && contains(s[1:], substr))
}

