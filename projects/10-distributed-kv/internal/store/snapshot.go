package store

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/raft"
)

type KVSnapshot struct {
	data map[string][]byte
}

func (s *KVSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		encoded, err := json.Marshal(s.data)
		if err != nil {
			return fmt.Errorf("encode snapshot: %w", err)
		}
		if _, err := sink.Write(encoded); err != nil {
			return fmt.Errorf("write snapshot sink: %w", err)
		}
		return sink.Close()
	}()
	if err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (s *KVSnapshot) Release() {}
