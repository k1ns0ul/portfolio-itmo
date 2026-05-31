package api

import (
	"strconv"

	"github.com/andrey/distributed-kv/internal/raft"
	"github.com/andrey/distributed-kv/internal/store"
)

type StateProvider struct {
	node  *raft.RaftNode
	store *store.KVStore
}

func NewStateProvider(node *raft.RaftNode, kv *store.KVStore) *StateProvider {
	return &StateProvider{node: node, store: kv}
}

func (p *StateProvider) KeyCount() int {
	return p.store.Count()
}

func (p *StateProvider) RaftState() string {
	return p.node.State()
}

func (p *StateProvider) Term() uint64 {
	return statUint(p.node.Stats(), "term")
}

func (p *StateProvider) CommitIndex() uint64 {
	return statUint(p.node.Stats(), "commit_index")
}

func (p *StateProvider) AppliedIndex() uint64 {
	return p.node.AppliedIndex()
}

func statUint(stats map[string]string, key string) uint64 {
	v, err := strconv.ParseUint(stats[key], 10, 64)
	if err != nil {
		return 0
	}
	return v
}
