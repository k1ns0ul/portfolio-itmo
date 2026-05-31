package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	EntityTrade    = "trade"
	EntityIssue    = "issue"
	EntityInvestor = "investor"
	EntityCoupon   = "coupon"
)

const (
	EventTradeSubmitted = "trade.submitted"
	EventTradeSettled   = "trade.settled"
	EventTradeFailed    = "trade.failed"
	EventCouponPaid     = "coupon.paid"
	EventIssueMatured   = "issue.matured"
	EventIssuePlaced    = "issue.placed"
	EventIssueCreated   = "issue.created"
	EventStatusChanged  = "issue.status_changed"
	EventDeposit        = "investor.deposit"
)

type EventLog struct {
	ID         uuid.UUID       `json:"id"`
	EntityType string          `json:"entity_type"`
	EntityID   uuid.UUID       `json:"entity_id"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
	CreatedAt  time.Time       `json:"created_at"`
}
