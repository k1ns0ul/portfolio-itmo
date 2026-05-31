package session

import "time"

type OpType string

const (
	OpSum        OpType = "sum"
	OpAverage    OpType = "average"
	OpMax        OpType = "max"
	OpComparison OpType = "comparison"
)

func (o OpType) Valid() bool {
	switch o {
	case OpSum, OpAverage, OpMax, OpComparison:
		return true
	default:
		return false
	}
}

type Status string

const (
	StatusCreated    Status = "created"
	StatusNodesReady Status = "nodes_ready"
	StatusComputing  Status = "computing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

type Session struct {
	ID         string    `json:"id"`
	Type       OpType    `json:"type"`
	Status     Status    `json:"status"`
	PartyCount int       `json:"party_count"`
	Threshold  int       `json:"threshold"`
	Result     string    `json:"result,omitempty"`
	ErrorMsg   string    `json:"error_msg,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Round struct {
	SessionID   string     `json:"session_id"`
	Number      int        `json:"number"`
	Phase       string     `json:"phase"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type NodeRecord struct {
	SessionID    string    `json:"session_id"`
	NodeID       int       `json:"node_id"`
	Addr         string    `json:"addr"`
	Status       string    `json:"status"`
	RegisteredAt time.Time `json:"registered_at"`
}
