package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config represents the server configuration
type Config struct {
	// Server settings
	ServerPort int    `json:"server_port" yaml:"server_port"`
	HTTPPort   int    `json:"http_port" yaml:"http_port"`
	ReplicaID  string `json:"replica_id" yaml:"replica_id"`

	// Data storage settings
	DataDir   string `json:"data_dir" yaml:"data_dir"`
	OpLogPath string `json:"oplog_path" yaml:"oplog_path"`

	// Redis settings
	RedisAddr string `json:"redis_addr" yaml:"redis_addr"`
	RedisDB   int    `json:"redis_db" yaml:"redis_db"`

	// Replication settings
	Peers         []string      `json:"peers" yaml:"peers"`
	SyncInterval  time.Duration `json:"sync_interval" yaml:"sync_interval"`
	SyncTimeout   time.Duration `json:"sync_timeout" yaml:"sync_timeout"`
	MaxRetries    int           `json:"max_retries" yaml:"max_retries"`
	RetryInterval time.Duration `json:"retry_interval" yaml:"retry_interval"`

	// Cluster discovery settings
	DiscoveryMode     string        `json:"discovery_mode" yaml:"discovery_mode"` // "static", "consul", "etcd"
	DiscoveryAddr     string        `json:"discovery_addr" yaml:"discovery_addr"`
	DiscoveryInterval time.Duration `json:"discovery_interval" yaml:"discovery_interval"`
	ClusterName       string        `json:"cluster_name" yaml:"cluster_name"`

	// Performance settings
	MaxConnections   int           `json:"max_connections" yaml:"max_connections"`
	ReadTimeout      time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout     time.Duration `json:"write_timeout" yaml:"write_timeout"`
	KeepAliveTimeout time.Duration `json:"keepalive_timeout" yaml:"keepalive_timeout"`
	MaxMemory        int64         `json:"max_memory" yaml:"max_memory"` // in bytes
	GCInterval       time.Duration `json:"gc_interval" yaml:"gc_interval"`

	// Logging settings
	LogLevel  string `json:"log_level" yaml:"log_level"`
	LogFile   string `json:"log_file" yaml:"log_file"`
	LogFormat string `json:"log_format" yaml:"log_format"` // "json", "text"

	// Security settings
	EnableTLS   bool   `json:"enable_tls" yaml:"enable_tls"`
	TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file" yaml:"tls_key_file"`
	AuthToken   string `json:"auth_token" yaml:"auth_token"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	return &Config{
		// Server settings
		ServerPort: 6379,
		HTTPPort:   8080,
		ReplicaID:  fmt.Sprintf("%s-%d", hostname, os.Getpid()),

		// Data storage settings
		DataDir:   "./data",
		OpLogPath: "./data/oplog.json",

		// Redis settings
		RedisAddr: "localhost:6379",
		RedisDB:   0,

		// Replication settings
		Peers:         []string{},
		SyncInterval:  5 * time.Second,
		SyncTimeout:   30 * time.Second,
		MaxRetries:    3,
		RetryInterval: 1 * time.Second,

		// Cluster discovery settings
		DiscoveryMode:     "static",
		DiscoveryAddr:     "",
		DiscoveryInterval: 30 * time.Second,
		ClusterName:       "crdt-redis-cluster",

		// Performance settings
		MaxConnections:   1000,
		ReadTimeout:      30 * time.Second,
		WriteTimeout:     30 * time.Second,
		KeepAliveTimeout: 300 * time.Second,
		MaxMemory:        1024 * 1024 * 1024, // 1GB
		GCInterval:       60 * time.Second,

		// Logging settings
		LogLevel:  "info",
		LogFile:   "",
		LogFormat: "text",

		// Security settings
		EnableTLS:   false,
		TLSCertFile: "",
		TLSKeyFile:  "",
		AuthToken:   "",
	}
}

// LoadFromFile loads configuration from a JSON or YAML file
func LoadFromFile(filename string) (*Config, error) {
	config := DefaultConfig()

	if filename == "" {
		return config, nil
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return config, fmt.Errorf("config file does not exist: %s", filename)
	}

	// Read file content
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Determine file format and parse
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		if err := json.Unmarshal(content, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %v", err)
		}
	case ".yaml", ".yml":
		// For now, we'll parse YAML as JSON (simplified)
		if err := json.Unmarshal(content, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %v", err)
		}
	default:
		// Try to parse as JSON by default
		if err := json.Unmarshal(content, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file (unknown format): %v", err)
		}
	}

	return config, nil
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv(config *Config) {
	if val := os.Getenv("CRDT_REDIS_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.ServerPort = port
		}
	}

	if val := os.Getenv("CRDT_HTTP_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.HTTPPort = port
		}
	}

	if val := os.Getenv("CRDT_REPLICA_ID"); val != "" {
		config.ReplicaID = val
	}

	if val := os.Getenv("CRDT_DATA_DIR"); val != "" {
		config.DataDir = val
	}

	if val := os.Getenv("CRDT_OPLOG_PATH"); val != "" {
		config.OpLogPath = val
	}

	if val := os.Getenv("CRDT_REDIS_ADDR"); val != "" {
		config.RedisAddr = val
	}

	if val := os.Getenv("CRDT_REDIS_DB"); val != "" {
		if db, err := strconv.Atoi(val); err == nil {
			config.RedisDB = db
		}
	}

	if val := os.Getenv("CRDT_PEERS"); val != "" {
		config.Peers = strings.Split(val, ",")
	}

	if val := os.Getenv("CRDT_SYNC_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.SyncInterval = duration
		}
	}

	if val := os.Getenv("CRDT_CLUSTER_NAME"); val != "" {
		config.ClusterName = val
	}

	if val := os.Getenv("CRDT_LOG_LEVEL"); val != "" {
		config.LogLevel = val
	}

	if val := os.Getenv("CRDT_LOG_FILE"); val != "" {
		config.LogFile = val
	}

	if val := os.Getenv("CRDT_AUTH_TOKEN"); val != "" {
		config.AuthToken = val
	}
}

// SaveToFile saves the configuration to a JSON file
func (c *Config) SaveToFile(filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Marshal to JSON with indentation
	content, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write to file
	if err := ioutil.WriteFile(filename, content, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ServerPort <= 0 || c.ServerPort > 65535 {
		return fmt.Errorf("invalid server port: %d", c.ServerPort)
	}

	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.HTTPPort)
	}

	if c.ReplicaID == "" {
		return fmt.Errorf("replica ID cannot be empty")
	}

	if c.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	if c.RedisDB < 0 || c.RedisDB > 15 {
		return fmt.Errorf("invalid Redis DB: %d (must be 0-15)", c.RedisDB)
	}

	if c.SyncInterval <= 0 {
		return fmt.Errorf("sync interval must be positive")
	}

	if c.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}

	// Validate discovery mode
	validModes := []string{"static", "consul", "etcd"}
	validMode := false
	for _, mode := range validModes {
		if c.DiscoveryMode == mode {
			validMode = true
			break
		}
	}
	if !validMode {
		return fmt.Errorf("invalid discovery mode: %s (valid: %v)", c.DiscoveryMode, validModes)
	}

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "error"}
	validLevel := false
	for _, level := range validLevels {
		if c.LogLevel == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid log level: %s (valid: %v)", c.LogLevel, validLevels)
	}

	return nil
}

// GetAddress returns the server address
func (c *Config) GetAddress() string {
	return fmt.Sprintf(":%d", c.ServerPort)
}

// GetHTTPAddress returns the HTTP server address
func (c *Config) GetHTTPAddress() string {
	return fmt.Sprintf(":%d", c.HTTPPort)
}

// GetOpLogPath returns the absolute path to the operation log
func (c *Config) GetOpLogPath() string {
	if filepath.IsAbs(c.OpLogPath) {
		return c.OpLogPath
	}
	return filepath.Join(c.DataDir, "oplog.json")
}

// GetPersistencePath returns the absolute path to the persistence file
func (c *Config) GetPersistencePath() string {
	return filepath.Join(c.DataDir, "store.json")
}

// String returns a string representation of the config
func (c *Config) String() string {
	content, _ := json.MarshalIndent(c, "", "  ")
	return string(content)
}

