package crdt

import "time"

type Value struct {
	Value     string
	Timestamp int64
	TTL       *int64    // Time-to-live in seconds, pointer to allow nil (no TTL)
	ExpireAt  time.Time // Expiration time
}

func (v *Value) String() string {
	return v.Value
}
