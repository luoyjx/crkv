package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/luoyjx/crdt-redis/proto"
	"github.com/luoyjx/crdt-redis/server"
)

// Peer represents a remote node
type Peer struct {
	Address string // http base, e.g. http://127.0.0.1:8083
}

// Config for Syncer
type Config struct {
	SelfAddress string
	Peers       []Peer
	Interval    time.Duration
}

// Syncer performs periodic pull and apply of operations between peers
type Syncer struct {
	cfg        Config
	srv        *server.Server
	httpClient *http.Client
	mu         sync.Mutex
	lastSent   int64               // watermark for outbound since
	lastPull   map[string]int64    // per-peer last pull timestamp
	seen       map[string]struct{} // op-id dedupe (best-effort)
}

func New(cfg Config, srv *server.Server) *Syncer {
	return &Syncer{
		cfg:        cfg,
		srv:        srv,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		lastSent:   0,
		lastPull:   make(map[string]int64),
		seen:       make(map[string]struct{}),
	}
}

// Start launches periodic replication in background
func (s *Syncer) Start(stop <-chan struct{}) {
	ticker := time.NewTicker(s.cfg.Interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.replicateOnce()
			case <-stop:
				return
			}
		}
	}()
}

// Stop gracefully stops the syncer (for future use if needed)
func (s *Syncer) Stop() {
	// Currently, stopping is handled via the stop channel in Start()
	// This method is a placeholder for future cleanup logic
}

// replicateOnce pulls from peers and pushes our ops
func (s *Syncer) replicateOnce() {
	// Pull from peers
	for _, p := range s.cfg.Peers {
		s.pullFromPeer(p)
	}
	// Push to peers
	s.pushToPeers()
}

func (s *Syncer) pullFromPeer(p Peer) {
	since := s.lastPull[p.Address]
	url := fmt.Sprintf("%s/ops?since=%d", p.Address, since)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var batch proto.OperationBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		return
	}
	for _, op := range batch.Operations {
		if op == nil || op.OperationId == "" {
			continue
		}
		if _, ok := s.seen[op.OperationId]; ok {
			continue
		}
		_ = s.srv.HandleOperation(context.Background(), op)
		s.seen[op.OperationId] = struct{}{}
		if op.Timestamp > s.lastPull[p.Address] {
			s.lastPull[p.Address] = op.Timestamp
		}
	}
}

func (s *Syncer) pushToPeers() {
	// Export ops since lastSent
	ops, err := s.srv.OpLog().GetOperations(s.lastSent)
	if err != nil || len(ops) == 0 {
		return
	}
	batch := proto.OperationBatch{Operations: ops}
	data, _ := json.Marshal(batch)
	for _, p := range s.cfg.Peers {
		url := fmt.Sprintf("%s/apply", p.Address)
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}
}

// no-op
