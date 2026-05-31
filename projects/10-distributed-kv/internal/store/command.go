package store

import (
	"encoding/json"
	"fmt"
)

const (
	OpSet    = "set"
	OpDelete = "delete"
)

type Command struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value []byte `json:"value,omitempty"`
}

func (c Command) Marshal() ([]byte, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal command %s/%s: %w", c.Op, c.Key, err)
	}
	return data, nil
}

func UnmarshalCommand(data []byte) (Command, error) {
	var c Command
	if err := json.Unmarshal(data, &c); err != nil {
		return Command{}, fmt.Errorf("unmarshal command: %w", err)
	}
	return c, nil
}
