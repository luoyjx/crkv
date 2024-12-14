package network

import (
	"fmt"
	"strings"

	"github.com/luoyjx/crdt-redis/server"
	"github.com/tidwall/redcon"
)

// RedisServer handles Redis protocol communication
type RedisServer struct {
	server *server.Server
}

// NewRedisServer creates a new Redis protocol server
func NewRedisServer(server *server.Server) *RedisServer {
	return &RedisServer{
		server: server,
	}
}

// Start starts the Redis protocol server
func (rs *RedisServer) Start(addr string) error {
	return redcon.ListenAndServe(addr,
		rs.handleCommand,
		rs.handleConnect,
		rs.handleDisconnect,
	)
}

// handleCommand processes Redis commands
func (rs *RedisServer) handleCommand(conn redcon.Conn, cmd redcon.Command) {
	switch strings.ToLower(string(cmd.Args[0])) {
	case "set":
		if len(cmd.Args) != 3 {
			conn.WriteError("ERR wrong number of arguments for 'set' command")
			return
		}
		key := string(cmd.Args[1])
		value := string(cmd.Args[2])
		if err := rs.server.Set(key, value); err != nil {
			conn.WriteError(fmt.Sprintf("ERR %v", err))
			return
		}
		conn.WriteString("OK")

	case "get":
		if len(cmd.Args) != 2 {
			conn.WriteError("ERR wrong number of arguments for 'get' command")
			return
		}
		key := string(cmd.Args[1])
		value, exists := rs.server.Get(key)
		if exists {
			conn.WriteBulk([]byte(value))
		} else {
			conn.WriteNull()
		}

	default:
		conn.WriteError("ERR unknown command")
	}
}

// handleConnect handles new connections
func (rs *RedisServer) handleConnect(conn redcon.Conn) bool {
	return true
}

// handleDisconnect handles client disconnections
func (rs *RedisServer) handleDisconnect(conn redcon.Conn, err error) {
	// Handle cleanup if needed
}
