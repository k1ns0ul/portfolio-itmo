package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/kafka"
	"github.com/andrey/cfa-bonds/internal/models"
)

type createTradeReq struct {
	IssueID  string `json:"issue_id" binding:"required"`
	SellerID string `json:"seller_id" binding:"required"`
	BuyerID  string `json:"buyer_id" binding:"required"`
	Quantity int64  `json:"quantity" binding:"required"`
	Price    string `json:"price" binding:"required"`
}

func (s *Server) createTrade(c *gin.Context) {
	var req createTradeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid trade payload: "+err.Error())
		return
	}
	issueID, err := uuid.Parse(req.IssueID)
	if err != nil {
		badRequest(c, "issue_id is not a valid uuid")
		return
	}
	sellerID, err := uuid.Parse(req.SellerID)
	if err != nil {
		badRequest(c, "seller_id is not a valid uuid")
		return
	}
	buyerID, err := uuid.Parse(req.BuyerID)
	if err != nil {
		badRequest(c, "buyer_id is not a valid uuid")
		return
	}
	if sellerID == buyerID {
		badRequest(c, "seller and buyer must differ")
		return
	}
	if req.Quantity <= 0 {
		badRequest(c, "quantity must be positive")
		return
	}
	price, err := decimal.NewFromString(req.Price)
	if err != nil || price.LessThanOrEqual(decimal.Zero) {
		badRequest(c, "price must be a positive number")
		return
	}

	trade := &models.Trade{
		IssueID:         issueID,
		SellerID:        sellerID,
		BuyerID:         buyerID,
		Quantity:        req.Quantity,
		Price:           price,
		AccruedInterest: decimal.Zero,
		TotalAmount:     price.Mul(decimal.NewFromInt(req.Quantity)),
		Status:          models.TradeSubmitted,
	}

	ctx := c.Request.Context()
	if err := s.trades.Create(ctx, trade); err != nil {
		failWith(c, err)
		return
	}

	payload, _ := json.Marshal(trade)
	_ = s.events.Append(ctx, nil, &models.EventLog{
		EntityType: models.EntityTrade,
		EntityID:   trade.ID,
		EventType:  models.EventTradeSubmitted,
		Payload:    payload,
	})

	if s.producer != nil {
		if err := s.producer.Publish(ctx, kafka.TopicTradeSubmitted, trade.ID.String(), trade); err != nil {
			s.log.Warn("publish trade.submitted failed", "trade", trade.ID, "err", err)
		}
	}

	created(c, trade)
}

func (s *Server) getTrade(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	trade, err := s.trades.Get(c.Request.Context(), id)
	if err != nil {
		failWith(c, err)
		return
	}
	sendOK(c, trade)
}

func (s *Server) listTrades(c *gin.Context) {
	limit, offset := parsePaging(c)
	f := repoTradeFilter(c, limit, offset)
	trades, err := s.trades.List(c.Request.Context(), f)
	if err != nil {
		failWith(c, err)
		return
	}
	listResponse(c, trades, pageMeta{Limit: limit, Offset: offset})
}
