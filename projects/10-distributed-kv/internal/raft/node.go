package raft

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	hraft "github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"

	"github.com/andrey/distributed-kv/internal/store"
)

const snapshotRetain = 3

type RaftNode struct {
	raft      *hraft.Raft
	transport *hraft.NetworkTransport
	store     *boltdb.BoltStore
	id        string
}

type Config struct {
	ID       string
	RaftAddr string
	DataDir  string
}

func NewRaftNode(cfg Config, fsm hraft.FSM) (*RaftNode, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir %s: %w", cfg.DataDir, err)
	}

	raftCfg := hraft.DefaultConfig()
	raftCfg.LocalID = hraft.ServerID(cfg.ID)
	raftCfg.SnapshotThreshold = 1024

	logStore, err := boltdb.NewBoltStore(filepath.Join(cfg.DataDir, "raft-log.bolt"))
	if err != nil {
		return nil, fmt.Errorf("open bolt log store: %w", err)
	}

	snapshots, err := hraft.NewFileSnapshotStore(cfg.DataDir, snapshotRetain, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("create snapshot store: %w", err)
	}

	transport, err := newTransport(cfg.RaftAddr)
	if err != nil {
		return nil, err
	}

	r, err := hraft.NewRaft(raftCfg, fsm, logStore, logStore, snapshots, transport)
	if err != nil {
		return nil, fmt.Errorf("create raft instance: %w", err)
	}

	return &RaftNode{raft: r, transport: transport, store: logStore, id: cfg.ID}, nil
}

func (n *RaftNode) Bootstrap(servers []hraft.Server) error {
	future := n.raft.BootstrapCluster(hraft.Configuration{Servers: servers})
	if err := future.Error(); err != nil {
		if err == hraft.ErrCantBootstrap {
			return nil
		}
		return fmt.Errorf("bootstrap cluster: %w", err)
	}
	return nil
}

func ServerID(id string) hraft.ServerID {
	return hraft.ServerID(id)
}

func (n *RaftNode) Apply(cmd store.Command, timeout time.Duration) error {
	data, err := cmd.Marshal()
	if err != nil {
		return err
	}
	future := n.raft.Apply(data, timeout)
	if err := future.Error(); err != nil {
		return fmt.Errorf("apply %s on %s: %w", cmd.Op, cmd.Key, err)
	}
	if resp := future.Response(); resp != nil {
		if applyErr, ok := resp.(error); ok && applyErr != nil {
			return fmt.Errorf("fsm rejected command: %w", applyErr)
		}
	}
	return nil
}

func (n *RaftNode) VerifyLeader() error {
	if err := n.raft.VerifyLeader().Error(); err != nil {
		return fmt.Errorf("verify leader: %w", err)
	}
	return nil
}

func (n *RaftNode) Leader() string {
	addr, _ := n.raft.LeaderWithID()
	return string(addr)
}

func (n *RaftNode) IsLeader() bool {
	return n.raft.State() == hraft.Leader
}

func (n *RaftNode) State() string {
	return n.raft.State().String()
}

func (n *RaftNode) AddVoter(id, addr string) error {
	if !n.IsLeader() {
		return fmt.Errorf("cannot add voter %s: node is not leader", id)
	}
	future := n.raft.AddVoter(hraft.ServerID(id), hraft.ServerAddress(addr), 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("add voter %s (%s): %w", id, addr, err)
	}
	return nil
}

func (n *RaftNode) RemoveServer(id string) error {
	if !n.IsLeader() {
		return fmt.Errorf("cannot remove %s: node is not leader", id)
	}
	future := n.raft.RemoveServer(hraft.ServerID(id), 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("remove server %s: %w", id, err)
	}
	return nil
}

func (n *RaftNode) Servers() ([]hraft.Server, error) {
	future := n.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	return future.Configuration().Servers, nil
}

func (n *RaftNode) Stats() map[string]string {
	return n.raft.Stats()
}

func (n *RaftNode) AppliedIndex() uint64 {
	return n.raft.AppliedIndex()
}

func (n *RaftNode) ID() string {
	return n.id
}

func (n *RaftNode) Shutdown() error {
	if err := n.raft.Shutdown().Error(); err != nil {
		return fmt.Errorf("shutdown raft: %w", err)
	}
	if err := n.transport.Close(); err != nil {
		return fmt.Errorf("close transport: %w", err)
	}
	if err := n.store.Close(); err != nil {
		return fmt.Errorf("close log store: %w", err)
	}
	return nil
}
