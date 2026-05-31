package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/models"
)

type createInvestorReq struct {
	Name          string `json:"name" binding:"required"`
	Type          string `json:"type" binding:"required"`
	AccountNumber string `json:"account_number" binding:"required"`
}

func (s *Server) createInvestor(c *gin.Context) {
	var req createInvestorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid investor payload: "+err.Error())
		return
	}
	if !models.ValidInvestorType(req.Type) {
		badRequest(c, "type must be individual or legal_entity")
		return
	}
	inv := &models.Investor{
		Name:          req.Name,
		Type:          req.Type,
		AccountNumber: req.AccountNumber,
		Balance:       decimal.Zero,
	}
	if err := s.investors.Create(c.Request.Context(), inv); err != nil {
		failWith(c, err)
		return
	}
	created(c, inv)
}

func (s *Server) getInvestor(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	inv, err := s.investors.Get(c.Request.Context(), id)
	if err != nil {
		failWith(c, err)
		return
	}
	sendOK(c, inv)
}

type depositReq struct {
	Amount string `json:"amount" binding:"required"`
}

func (s *Server) investorDeposit(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	var req depositReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid deposit payload: "+err.Error())
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		badRequest(c, "amount must be a positive number")
		return
	}
	ctx := c.Request.Context()
	if err := s.investors.UpdateBalance(ctx, id, amount); err != nil {
		failWith(c, err)
		return
	}
	payload, _ := json.Marshal(map[string]any{"amount": amount.String()})
	if err := s.events.Append(ctx, nil, &models.EventLog{
		EntityType: models.EntityInvestor,
		EntityID:   id,
		EventType:  models.EventDeposit,
		Payload:    payload,
	}); err != nil {
		s.log.Warn("append deposit event", "err", err)
	}
	if s.cache != nil {
		_ = s.cache.InvalidatePortfolio(ctx, id)
	}
	inv, err := s.investors.Get(ctx, id)
	if err != nil {
		failWith(c, err)
		return
	}
	sendOK(c, inv)
}

func (s *Server) investorEvents(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	limit := queryInt(c, "limit", 100)
	events, err := s.events.ListByEntity(c.Request.Context(), models.EntityInvestor, id, limit)
	if err != nil {
		failWith(c, err)
		return
	}
	sendOK(c, gin.H{"events": events})
}

func (s *Server) investorPortfolio(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	ctx := c.Request.Context()

	if s.cache != nil {
		if cached, hit, err := s.cache.GetPortfolio(ctx, id); err == nil && hit {
			c.Header("X-Cache", "hit")
			sendOK(c, cached)
			return
		}
	}

	inv, err := s.investors.Get(ctx, id)
	if err != nil {
		failWith(c, err)
		return
	}
	positions, err := s.positions.GetByInvestor(ctx, id)
	if err != nil {
		failWith(c, err)
		return
	}
	coupons, err := s.coupons.GetPaidByInvestor(ctx, id)
	if err != nil {
		failWith(c, err)
		return
	}

	summary := &models.PortfolioSummary{
		InvestorID:      id,
		Cash:            inv.Balance,
		CouponsReceived: coupons,
		TotalValue:      inv.Balance,
		TotalPnL:        decimal.Zero,
	}
	for _, p := range positions {
		summary.Positions = append(summary.Positions, *p)
		summary.TotalValue = summary.TotalValue.Add(p.MarketValue)
		summary.TotalPnL = summary.TotalPnL.Add(p.UnrealizedPL)
	}

	if s.cache != nil {
		if err := s.cache.SetPortfolio(ctx, summary); err != nil {
			s.log.Warn("cache portfolio", "err", err)
		}
	}
	c.Header("X-Cache", "miss")
	sendOK(c, summary)
}
