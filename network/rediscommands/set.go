package rediscommands

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/redcon"
)

// SetArgs struct is used to store the parameters for the SET command
type SetArgs struct {
	Key     string
	Value   string
	NX      bool
	XX      bool
	Get     bool
	EX      *time.Duration
	PX      *time.Duration
	EXAT    *time.Time
	PXAT    *time.Time
	Keepttl bool
}

func ParseSetArgs(cmd redcon.Command) (*SetArgs, error) {
	args := &SetArgs{}
	if len(cmd.Args) < 3 {
		return nil, errors.New("syntax error")
	}
	args.Key = string(cmd.Args[1])
	args.Value = string(cmd.Args[2])

	for i := 3; i < len(cmd.Args); i++ {
		arg := strings.ToLower(string(cmd.Args[i]))
		switch arg {
		case "nx":
			args.NX = true
		case "xx":
			args.XX = true
		case "get":
			args.Get = true
		case "ex":
			i++
			if i >= len(cmd.Args) {
				return nil, errors.New("syntax error")
			}
			seconds, err := strconv.ParseInt(string(cmd.Args[i]), 10, 64)
			if err != nil || seconds <= 0 {
				return nil, errors.New("syntax error")
			}
			duration := time.Second * time.Duration(seconds)
			args.EX = &duration
		case "px":
			i++
			if i >= len(cmd.Args) {
				return nil, errors.New("syntax error")
			}
			milliseconds, err := strconv.ParseInt(string(cmd.Args[i]), 10, 64)
			if err != nil || milliseconds <= 0 {
				return nil, errors.New("syntax error")
			}
			duration := time.Millisecond * time.Duration(milliseconds)
			args.PX = &duration
		case "exat":
			i++
			if i >= len(cmd.Args) {
				return nil, errors.New("syntax error")
			}
			timestamp, err := strconv.ParseInt(string(cmd.Args[i]), 10, 64)
			if err != nil || timestamp <= 0 {
				return nil, errors.New("syntax error")
			}
			t := time.Unix(timestamp, 0)
			args.EXAT = &t
		case "pxat":
			i++
			if i >= len(cmd.Args) {
				return nil, errors.New("syntax error")
			}
			timestamp, err := strconv.ParseInt(string(cmd.Args[i]), 10, 64)
			if err != nil || timestamp <= 0 {
				return nil, errors.New("syntax error")
			}
			t := time.Unix(0, timestamp*int64(time.Millisecond))
			args.PXAT = &t
		case "keepttl":
			args.Keepttl = true
		default:
			return nil, errors.New("syntax error") // or return a more specific error message
		}
	}
	return args, nil
}
