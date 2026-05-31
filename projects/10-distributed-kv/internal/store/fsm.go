package store

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/hashicorp/raft"
)

type KVStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewKVStore() *KVStore {
	return &KVStore{data: make(map[string][]byte)}
}

func (s *KVStore) Apply(log *raft.Log) interface{} {
	cmd, err := UnmarshalCommand(log.Data)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch cmd.Op {
	case OpSet:
		stored := make([]byte, len(cmd.Value))
		copy(stored, cmd.Value)
		s.data[cmd.Key] = stored
		return nil
	case OpDelete:
		delete(s.data, cmd.Key)
		return nil
	default:
		return fmt.Errorf("unknown operation %q at index %d", cmd.Op, log.Index)
	}
}

func (s *KVStore) Snapshot() (raft.FSMSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := make(map[string][]byte, len(s.data))
	for k, v := range s.data {
		dup := make([]byte, len(v))
		copy(dup, v)
		clone[k] = dup
	}
	return &KVSnapshot{data: clone}, nil
}

func (s *KVStore) Restore(reader io.ReadCloser) error {
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}
	restored := make(map[string][]byte)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &restored); err != nil {
			return fmt.Errorf("decode snapshot: %w", err)
		}
	}

	s.mu.Lock()
	s.data = restored
	s.mu.Unlock()
	return nil
}

func (s *KVStore) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	if !ok {
		return nil, false
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, true
}

func (s *KVStore) Keys(prefix string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0)
	for k := range s.data {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys
}

func (s *KVStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}
