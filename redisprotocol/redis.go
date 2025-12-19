package redisprotocol

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/luoyjx/crdt-redis/redisprotocol/commands"
	"github.com/luoyjx/crdt-redis/server"
	"github.com/luoyjx/crdt-redis/storage"
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
		// Parse the SET command arguments
		setArgs, err := commands.ParseSetArgs(cmd)
		if err != nil {
			// If there's an error parsing the arguments, return an error to the client
			conn.WriteError("ERR " + err.Error())
			return
		}

		// Check if both NX and XX options are provided, which is an error
		if setArgs.NX && setArgs.XX {
			conn.WriteError("ERR NX and XX options are mutually exclusive")
			return
		}

		// Call the server's Set method with the parsed arguments and options
		err = rs.server.Set(setArgs.Key, setArgs.Value, &server.SetOptions{
			NX:      setArgs.NX,
			XX:      setArgs.XX,
			EX:      setArgs.EX,
			PX:      setArgs.PX,
			EXAT:    setArgs.EXAT,
			PXAT:    setArgs.PXAT,
			Keepttl: setArgs.Keepttl,
		})
		if err != nil {
			// If there's an error setting the value, return an error to the client
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

	case "ping":
		if len(cmd.Args) == 1 {
			conn.WriteString("PONG")
		} else if len(cmd.Args) == 2 {
			conn.WriteBulk(cmd.Args[1])
		} else {
			conn.WriteError("ERR wrong number of arguments for 'ping' command")
		}

	case "echo":
		if len(cmd.Args) != 2 {
			conn.WriteError("ERR wrong number of arguments for 'echo' command")
			return
		}
		conn.WriteBulk(cmd.Args[1])

	case "info":
		// Simple INFO response for basic compatibility
		info := "# Server\r\nredis_version:7.0.0-crdt\r\nredis_mode:standalone\r\n# Replication\r\nrole:master\r\n"
		conn.WriteBulk([]byte(info))

	default:
		switch strings.ToLower(string(cmd.Args[0])) {
		case "exists":
			if len(cmd.Args) < 2 {
				conn.WriteError("ERR wrong number of arguments for 'exists' command")
				return
			}
			var keys []string
			for i := 1; i < len(cmd.Args); i++ {
				keys = append(keys, string(cmd.Args[i]))
			}
			n := rs.server.Exists(keys...)
			conn.WriteInt64(n)
		case "ttl":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'ttl' command")
				return
			}
			key := string(cmd.Args[1])
			ttl := rs.server.TTL(key)
			conn.WriteInt64(ttl)
		case "pttl":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'pttl' command")
				return
			}
			key := string(cmd.Args[1])
			ttl := rs.server.PTTL(key)
			conn.WriteInt64(ttl)
		case "getdel":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'getdel' command")
				return
			}
			key := string(cmd.Args[1])
			val, ok, err := rs.server.GetDel(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			if ok {
				conn.WriteBulkString(val)
			} else {
				conn.WriteNull()
			}
		case "expire":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'expire' command")
				return
			}
			key := string(cmd.Args[1])
			seconds, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			n, err := rs.server.Expire(key, seconds)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(n)
		case "pexpire":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'pexpire' command")
				return
			}
			key := string(cmd.Args[1])
			ms, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			n, err := rs.server.PExpire(key, ms)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(n)
		case "expireat":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'expireat' command")
				return
			}
			key := string(cmd.Args[1])
			ts, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			n, err := rs.server.ExpireAt(key, ts)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(n)
		case "del":
			if len(cmd.Args) < 2 {
				conn.WriteError("ERR wrong number of arguments for 'del' command")
				return
			}
			var keys []string
			for i := 1; i < len(cmd.Args); i++ {
				keys = append(keys, string(cmd.Args[i]))
			}
			removed, err := rs.server.Del(keys...)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(removed)
		case "incr":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'incr' command")
				return
			}
			key := string(cmd.Args[1])
			val, err := rs.server.Incr(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(val)
		case "incrby":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'incrby' command")
				return
			}
			key := string(cmd.Args[1])
			delta, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			val, err := rs.server.IncrBy(key, delta)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(val)
		case "decr":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'decr' command")
				return
			}
			key := string(cmd.Args[1])
			val, err := rs.server.Decr(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(val)
		case "decrby":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'decrby' command")
				return
			}
			key := string(cmd.Args[1])
			delta, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			val, err := rs.server.DecrBy(key, delta)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(val)
		case "incrbyfloat":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'incrbyfloat' command")
				return
			}
			key := string(cmd.Args[1])
			deltaStr := string(cmd.Args[2])
			delta, err := strconv.ParseFloat(deltaStr, 64)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR value is not a valid float: %s", deltaStr))
				return
			}
			val, err := rs.server.IncrByFloat(key, delta)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			// Redis returns float values as bulk strings
			conn.WriteBulkString(strconv.FormatFloat(val, 'g', -1, 64))
		case "lpush":
			if len(cmd.Args) < 3 {
				conn.WriteError("ERR wrong number of arguments for 'lpush' command")
				return
			}
			key := string(cmd.Args[1])
			var values []string
			for i := 2; i < len(cmd.Args); i++ {
				values = append(values, string(cmd.Args[i]))
			}
			length, err := rs.server.LPush(key, values...)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(length)
		case "rpush":
			if len(cmd.Args) < 3 {
				conn.WriteError("ERR wrong number of arguments for 'rpush' command")
				return
			}
			key := string(cmd.Args[1])
			var values []string
			for i := 2; i < len(cmd.Args); i++ {
				values = append(values, string(cmd.Args[i]))
			}
			length, err := rs.server.RPush(key, values...)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(length)
		case "lpop":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'lpop' command")
				return
			}
			key := string(cmd.Args[1])
			value, ok, err := rs.server.LPop(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			if ok {
				conn.WriteBulkString(value)
			} else {
				conn.WriteNull()
			}
		case "rpop":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'rpop' command")
				return
			}
			key := string(cmd.Args[1])
			value, ok, err := rs.server.RPop(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			if ok {
				conn.WriteBulkString(value)
			} else {
				conn.WriteNull()
			}
		case "lrange":
			if len(cmd.Args) != 4 {
				conn.WriteError("ERR wrong number of arguments for 'lrange' command")
				return
			}
			key := string(cmd.Args[1])
			start, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			stop, err := commands.ParseInt64(string(cmd.Args[3]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			values, err := rs.server.LRange(key, int(start), int(stop))
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteArray(len(values))
			for _, v := range values {
				conn.WriteBulkString(v)
			}
		case "llen":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'llen' command")
				return
			}
			key := string(cmd.Args[1])
			length, err := rs.server.LLen(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(length)
		case "lindex":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'lindex' command")
				return
			}
			key := string(cmd.Args[1])
			index, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			value, exists, err := rs.server.LIndex(key, int(index))
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			if exists {
				conn.WriteBulkString(value)
			} else {
				conn.WriteNull()
			}
		case "lset":
			if len(cmd.Args) != 4 {
				conn.WriteError("ERR wrong number of arguments for 'lset' command")
				return
			}
			key := string(cmd.Args[1])
			index, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			value := string(cmd.Args[3])
			err = rs.server.LSet(key, int(index), value)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteString("OK")
		case "linsert":
			if len(cmd.Args) != 5 {
				conn.WriteError("ERR wrong number of arguments for 'linsert' command")
				return
			}
			key := string(cmd.Args[1])
			position := strings.ToUpper(string(cmd.Args[2]))
			pivot := string(cmd.Args[3])
			value := string(cmd.Args[4])

			var before bool
			if position == "BEFORE" {
				before = true
			} else if position == "AFTER" {
				before = false
			} else {
				conn.WriteError("ERR syntax error")
				return
			}

			result, err := rs.server.LInsert(key, before, pivot, value)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(result)
		case "ltrim":
			if len(cmd.Args) != 4 {
				conn.WriteError("ERR wrong number of arguments for 'ltrim' command")
				return
			}
			key := string(cmd.Args[1])
			start, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			stop, err := commands.ParseInt64(string(cmd.Args[3]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			err = rs.server.LTrim(key, int(start), int(stop))
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteString("OK")
		case "lrem":
			if len(cmd.Args) != 4 {
				conn.WriteError("ERR wrong number of arguments for 'lrem' command")
				return
			}
			key := string(cmd.Args[1])
			count, err := commands.ParseInt64(string(cmd.Args[2]))
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			value := string(cmd.Args[3])
			removed, err := rs.server.LRem(key, int(count), value)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(removed)
		case "sadd":
			if len(cmd.Args) < 3 {
				conn.WriteError("ERR wrong number of arguments for 'sadd' command")
				return
			}
			key := string(cmd.Args[1])
			var members []string
			for i := 2; i < len(cmd.Args); i++ {
				members = append(members, string(cmd.Args[i]))
			}
			added, err := rs.server.SAdd(key, members...)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(added)
		case "srem":
			if len(cmd.Args) < 3 {
				conn.WriteError("ERR wrong number of arguments for 'srem' command")
				return
			}
			key := string(cmd.Args[1])
			var members []string
			for i := 2; i < len(cmd.Args); i++ {
				members = append(members, string(cmd.Args[i]))
			}
			removed, err := rs.server.SRem(key, members...)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(removed)
		case "smembers":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'smembers' command")
				return
			}
			key := string(cmd.Args[1])
			members, err := rs.server.SMembers(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteArray(len(members))
			for _, member := range members {
				conn.WriteBulkString(member)
			}
		case "scard":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'scard' command")
				return
			}
			key := string(cmd.Args[1])
			cardinality, err := rs.server.SCard(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(cardinality)
		case "sismember":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'sismember' command")
				return
			}
			key := string(cmd.Args[1])
			member := string(cmd.Args[2])
			exists, err := rs.server.SIsMember(key, member)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			if exists {
				conn.WriteInt64(1)
			} else {
				conn.WriteInt64(0)
			}
		case "hset":
			if len(cmd.Args) != 4 {
				conn.WriteError("ERR wrong number of arguments for 'hset' command")
				return
			}
			key := string(cmd.Args[1])
			field := string(cmd.Args[2])
			value := string(cmd.Args[3])
			isNew, err := rs.server.HSet(key, field, value)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(isNew)
		case "hget":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'hget' command")
				return
			}
			key := string(cmd.Args[1])
			field := string(cmd.Args[2])
			value, exists, err := rs.server.HGet(key, field)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			if exists {
				conn.WriteBulkString(value)
			} else {
				conn.WriteNull()
			}
		case "hdel":
			if len(cmd.Args) < 3 {
				conn.WriteError("ERR wrong number of arguments for 'hdel' command")
				return
			}
			key := string(cmd.Args[1])
			var fields []string
			for i := 2; i < len(cmd.Args); i++ {
				fields = append(fields, string(cmd.Args[i]))
			}
			deleted, err := rs.server.HDel(key, fields...)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(deleted)
		case "hgetall":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'hgetall' command")
				return
			}
			key := string(cmd.Args[1])
			fields, err := rs.server.HGetAll(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteArray(len(fields) * 2)
			for field, value := range fields {
				conn.WriteBulkString(field)
				conn.WriteBulkString(value)
			}
		case "hlen":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'hlen' command")
				return
			}
			key := string(cmd.Args[1])
			length, err := rs.server.HLen(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(length)
		case "hincrby":
			if len(cmd.Args) != 4 {
				conn.WriteError("ERR wrong number of arguments for 'hincrby' command")
				return
			}
			key := string(cmd.Args[1])
			field := string(cmd.Args[2])
			deltaStr := string(cmd.Args[3])

			delta, err := strconv.ParseInt(deltaStr, 10, 64)
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}

			newValue, err := rs.server.HIncrBy(key, field, delta)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(newValue)
		case "hincrbyfloat":
			if len(cmd.Args) != 4 {
				conn.WriteError("ERR wrong number of arguments for 'hincrbyfloat' command")
				return
			}
			key := string(cmd.Args[1])
			field := string(cmd.Args[2])
			deltaStr := string(cmd.Args[3])

			delta, err := strconv.ParseFloat(deltaStr, 64)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR value is not a valid float: %s", deltaStr))
				return
			}

			newValue, err := rs.server.HIncrByFloat(key, field, delta)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteBulkString(fmt.Sprintf("%.17g", newValue))
		case "zadd":
			if len(cmd.Args) < 4 || len(cmd.Args)%2 != 0 {
				conn.WriteError("ERR wrong number of arguments for 'zadd' command")
				return
			}
			key := string(cmd.Args[1])

			// Parse score-member pairs
			memberScores := make(map[string]float64)
			for i := 2; i < len(cmd.Args); i += 2 {
				scoreStr := string(cmd.Args[i])
				member := string(cmd.Args[i+1])

				score, err := strconv.ParseFloat(scoreStr, 64)
				if err != nil {
					conn.WriteError(fmt.Sprintf("ERR value is not a valid float: %s", scoreStr))
					return
				}
				memberScores[member] = score
			}

			added, err := rs.server.ZAdd(key, memberScores)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(int64(added))
		case "zrem":
			if len(cmd.Args) < 3 {
				conn.WriteError("ERR wrong number of arguments for 'zrem' command")
				return
			}
			key := string(cmd.Args[1])
			members := make([]string, len(cmd.Args)-2)
			for i := 2; i < len(cmd.Args); i++ {
				members[i-2] = string(cmd.Args[i])
			}

			removed, err := rs.server.ZRem(key, members)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(int64(removed))
		case "zscore":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'zscore' command")
				return
			}
			key := string(cmd.Args[1])
			member := string(cmd.Args[2])

			score, exists, err := rs.server.ZScore(key, member)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			if !exists {
				conn.WriteNull()
			} else {
				conn.WriteBulkString(fmt.Sprintf("%.17g", *score))
			}
		case "zcard":
			if len(cmd.Args) != 2 {
				conn.WriteError("ERR wrong number of arguments for 'zcard' command")
				return
			}
			key := string(cmd.Args[1])

			count, err := rs.server.ZCard(key)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteInt64(int64(count))
		case "zrange":
			if len(cmd.Args) < 4 {
				conn.WriteError("ERR wrong number of arguments for 'zrange' command")
				return
			}
			key := string(cmd.Args[1])

			// Parse arguments
			args := make([]string, len(cmd.Args)-2)
			for i := 2; i < len(cmd.Args); i++ {
				args[i-2] = string(cmd.Args[i])
			}

			start, stop, withScores, err := storage.ParseZRangeArgs(args)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}

			members, scores, err := rs.server.ZRange(key, start, stop, withScores)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}

			if withScores {
				conn.WriteArray(len(members) * 2)
				for i, member := range members {
					conn.WriteBulkString(member)
					conn.WriteBulkString(fmt.Sprintf("%.17g", scores[i]))
				}
			} else {
				conn.WriteArray(len(members))
				for _, member := range members {
					conn.WriteBulkString(member)
				}
			}
		case "zrangebyscore":
			if len(cmd.Args) < 4 {
				conn.WriteError("ERR wrong number of arguments for 'zrangebyscore' command")
				return
			}
			key := string(cmd.Args[1])

			// Parse arguments
			args := make([]string, len(cmd.Args)-2)
			for i := 2; i < len(cmd.Args); i++ {
				args[i-2] = string(cmd.Args[i])
			}

			min, max, withScores, err := storage.ParseZRangeByScoreArgs(args)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}

			members, scores, err := rs.server.ZRangeByScore(key, min, max, withScores)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}

			if withScores {
				conn.WriteArray(len(members) * 2)
				for i, member := range members {
					conn.WriteBulkString(member)
					conn.WriteBulkString(fmt.Sprintf("%.17g", scores[i]))
				}
			} else {
				conn.WriteArray(len(members))
				for _, member := range members {
					conn.WriteBulkString(member)
				}
			}
		case "zrank":
			if len(cmd.Args) != 3 {
				conn.WriteError("ERR wrong number of arguments for 'zrank' command")
				return
			}
			key := string(cmd.Args[1])
			member := string(cmd.Args[2])

			rank, exists, err := rs.server.ZRank(key, member)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			if !exists {
				conn.WriteNull()
			} else {
				conn.WriteInt64(int64(*rank))
			}
		case "zincrby":
			if len(cmd.Args) != 4 {
				conn.WriteError("ERR wrong number of arguments for 'zincrby' command")
				return
			}
			key := string(cmd.Args[1])
			incrementStr := string(cmd.Args[2])
			member := string(cmd.Args[3])

			increment, err := strconv.ParseFloat(incrementStr, 64)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR value is not a valid float: %s", incrementStr))
				return
			}

			newScore, err := rs.server.ZIncrBy(key, member, increment)
			if err != nil {
				conn.WriteError(fmt.Sprintf("ERR %v", err))
				return
			}
			conn.WriteBulkString(fmt.Sprintf("%.17g", newScore))
		default:
			conn.WriteError("ERR unknown command")
		}
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
