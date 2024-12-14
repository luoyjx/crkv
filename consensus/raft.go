package consensus

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// RaftNode manages the Raft consensus
type RaftNode struct {
	raft     *raft.Raft
	dataDir  string
	bindAddr string
}

// NewRaftNode creates a new Raft consensus node
func NewRaftNode(dataDir, bindAddr string) (*RaftNode, error) {
	node := &RaftNode{
		dataDir:  dataDir,
		bindAddr: bindAddr,
	}
	if err := node.initialize(); err != nil {
		return nil, err
	}
	return node, nil
}

// initialize sets up the Raft node
func (n *RaftNode) initialize() error {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(n.getNodeID())

	addr, err := net.ResolveTCPAddr("tcp", n.bindAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %v", err)
	}

	transport, err := raft.NewTCPTransport(n.bindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create transport: %v", err)
	}

	snapshots, err := raft.NewFileSnapshotStore(n.dataDir, 2, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create snapshot store: %v", err)
	}

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(n.dataDir, "raft-log.bolt"))
	if err != nil {
		return fmt.Errorf("failed to create log store: %v", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(n.dataDir, "raft-stable.bolt"))
	if err != nil {
		return fmt.Errorf("failed to create stable store: %v", err)
	}

	ra, err := raft.NewRaft(config, nil, logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("failed to create raft: %v", err)
	}

	n.raft = ra
	return nil
}

// getNodeID returns a unique identifier for this node
func (n *RaftNode) getNodeID() string {
	hostname, _ := os.Hostname()
	return hostname
}

// State returns the current state of the Raft node
func (n *RaftNode) State() raft.RaftState {
	return n.raft.State()
}

// Leader returns the current leader of the cluster
func (n *RaftNode) Leader() string {
	return string(n.raft.Leader())
}

// Close shuts down the Raft node
func (n *RaftNode) Close() error {
	future := n.raft.Shutdown()
	return future.Error()
}
