package models

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventRawTransaction EventType = "raw_transaction"
	EventScoreUpdated   EventType = "score_updated"
	EventTokenScored    EventType = "token_scored"
	EventSwapDetected   EventType = "swap_detected"
	EventAlertRaised    EventType = "alert_raised"
)

type Envelope struct {
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"ts"`
	Source    string          `json:"source,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

func NewEnvelope(t EventType, source string, payload any) (Envelope, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Type:      t,
		Timestamp: time.Now().UTC(),
		Source:    source,
		Payload:   b,
	}, nil
}

func (e Envelope) Decode(out any) error { return json.Unmarshal(e.Payload, out) }

func (e Envelope) Encode() ([]byte, error) { return json.Marshal(e) }
