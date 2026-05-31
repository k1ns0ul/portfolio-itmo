package models

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventPromoActivated    EventType = "promo.activated"
	EventPurchaseCreated   EventType = "purchase.created"
	EventPurchaseConfirmed EventType = "purchase.confirmed"
	EventReferralCredited  EventType = "referral.credited"
	EventCFASettlement     EventType = "cfa.settlement.created"
)

type KafkaEvent struct {
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"ts"`
	Source    string          `json:"source,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

func NewEvent(t EventType, source string, payload any) (KafkaEvent, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return KafkaEvent{}, err
	}
	return KafkaEvent{Type: t, Timestamp: time.Now().UTC(), Source: source, Payload: b}, nil
}

func (e KafkaEvent) Decode(out any) error { return json.Unmarshal(e.Payload, out) }
func (e KafkaEvent) Marshal() ([]byte, error) { return json.Marshal(e) }
