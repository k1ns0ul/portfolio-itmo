package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/models"
)

type createIssueReq struct {
	IssuerID        string `json:"issuer_id" binding:"required"`
	Name            string `json:"name" binding:"required"`
	ISIN            string `json:"isin" binding:"required"`
	Nominal         string `json:"nominal" binding:"required"`
	CouponRate      string `json:"coupon_rate" binding:"required"`
	CouponFrequency int    `json:"coupon_frequency" binding:"required"`
	IssueDate       string `json:"issue_date" binding:"required"`
	MaturityDate    string `json:"maturity_date" binding:"required"`
	TotalQuantity   int64  `json:"total_quantity" binding:"required"`
}

func (s *Server) createIssue(c *gin.Context) {
	var req createIssueReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid issue payload: "+err.Error())
		return
	}
	issuerID, err := uuid.Parse(req.IssuerID)
	if err != nil {
		badRequest(c, "issuer_id is not a valid uuid")
		return
	}
	if !models.ValidCouponFrequency(req.CouponFrequency) {
		badRequest(c, "coupon_frequency must be 1, 2 or 4")
		return
	}
	nominal, err := decimal.NewFromString(req.Nominal)
	if err != nil || nominal.LessThanOrEqual(decimal.Zero) {
		badRequest(c, "nominal must be a positive number")
		return
	}
	rate, err := decimal.NewFromString(req.CouponRate)
	if err != nil || rate.IsNegative() {
		badRequest(c, "coupon_rate must be a non-negative number")
		return
	}
	issueDate, err := time.Parse("2006-01-02", req.IssueDate)
	if err != nil {
		badRequest(c, "issue_date must be YYYY-MM-DD")
		return
	}
	maturityDate, err := time.Parse("2006-01-02", req.MaturityDate)
	if err != nil {
		badRequest(c, "maturity_date must be YYYY-MM-DD")
		return
	}
	if !maturityDate.After(issueDate) {
		badRequest(c, "maturity_date must be after issue_date")
		return
	}
	if req.TotalQuantity <= 0 {
		badRequest(c, "total_quantity must be positive")
		return
	}

	issue := &models.BondIssue{
		IssuerID:        issuerID,
		Name:            req.Name,
		ISIN:            req.ISIN,
		Nominal:         nominal,
		CouponRate:      rate,
		CouponFrequency: req.CouponFrequency,
		IssueDate:       issueDate,
		MaturityDate:    maturityDate,
		TotalQuantity:   req.TotalQuantity,
		Status:          models.IssueDraft,
	}

	ctx := c.Request.Context()
	if err := s.issues.Create(ctx, issue); err != nil {
		failWith(c, err)
		return
	}

	schedule := buildSchedule(issue)
	if err := s.coupons.CreateBatch(ctx, s.pool, schedule); err != nil {
		failWith(c, err)
		return
	}

	payload, _ := json.Marshal(map[string]any{"isin": issue.ISIN, "total_quantity": issue.TotalQuantity})
	_ = s.events.Append(ctx, nil, &models.EventLog{
		EntityType: models.EntityIssue,
		EntityID:   issue.ID,
		EventType:  models.EventIssueCreated,
		Payload:    payload,
	})

	created(c, gin.H{"issue": issue, "coupon_schedule": schedule})
}

func buildSchedule(issue *models.BondIssue) []*models.CouponSchedule {
	perPeriod := issue.Nominal.Mul(issue.CouponRate).Div(decimal.NewFromInt(int64(issue.CouponFrequency))).Round(6)
	monthsStep := 12 / issue.CouponFrequency
	var schedule []*models.CouponSchedule
	seq := 1
	for d := issue.IssueDate.AddDate(0, monthsStep, 0); !d.After(issue.MaturityDate); d = d.AddDate(0, monthsStep, 0) {
		schedule = append(schedule, &models.CouponSchedule{
			IssueID:     issue.ID,
			SequenceNum: seq,
			PaymentDate: d,
			Amount:      perPeriod,
			Status:      models.CouponScheduled,
		})
		seq++
	}
	if len(schedule) == 0 {
		schedule = append(schedule, &models.CouponSchedule{
			IssueID:     issue.ID,
			SequenceNum: 1,
			PaymentDate: issue.MaturityDate,
			Amount:      perPeriod,
			Status:      models.CouponScheduled,
		})
	}
	return schedule
}

func (s *Server) getIssue(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	ctx := c.Request.Context()
	issue, err := s.issues.Get(ctx, id)
	if err != nil {
		failWith(c, err)
		return
	}
	schedule, err := s.coupons.GetByIssue(ctx, id)
	if err != nil {
		failWith(c, err)
		return
	}
	sendOK(c, gin.H{"issue": issue, "coupon_schedule": schedule})
}

func (s *Server) listIssues(c *gin.Context) {
	limit, offset := parsePaging(c)
	f := repoIssueFilter(c, limit, offset)
	items, total, err := s.issues.List(c.Request.Context(), f)
	if err != nil {
		failWith(c, err)
		return
	}
	listResponse(c, items, pageMeta{Limit: limit, Offset: offset, Total: total})
}

type updateStatusReq struct {
	Status string `json:"status" binding:"required"`
}

func (s *Server) updateIssueStatus(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	var req updateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid status payload: "+err.Error())
		return
	}
	ctx := c.Request.Context()
	issue, err := s.issues.Get(ctx, id)
	if err != nil {
		failWith(c, err)
		return
	}
	if !models.CanTransition(issue.Status, req.Status) {
		c.JSON(http.StatusConflict, gin.H{"error": "transition " + issue.Status + " -> " + req.Status + " not allowed"})
		return
	}
	if err := s.issues.UpdateStatus(ctx, id, issue.Status, req.Status); err != nil {
		failWith(c, err)
		return
	}
	payload, _ := json.Marshal(map[string]any{"from": issue.Status, "to": req.Status})
	_ = s.events.Append(ctx, nil, &models.EventLog{
		EntityType: models.EntityIssue,
		EntityID:   id,
		EventType:  models.EventStatusChanged,
		Payload:    payload,
	})
	sendOK(c, gin.H{"id": id, "status": req.Status})
}

func (s *Server) issueHolders(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	holders, err := s.positions.GetByIssue(c.Request.Context(), nil, id)
	if err != nil {
		failWith(c, err)
		return
	}
	sendOK(c, gin.H{"holders": holders})
}

func (s *Server) issueTrades(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	limit, offset := parsePaging(c)
	trades, err := s.trades.ListByIssue(c.Request.Context(), id, limit, offset)
	if err != nil {
		failWith(c, err)
		return
	}
	listResponse(c, trades, pageMeta{Limit: limit, Offset: offset})
}

func (s *Server) issueEvents(c *gin.Context) {
	id, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	limit := queryInt(c, "limit", 100)
	events, err := s.events.ListByEntity(c.Request.Context(), models.EntityIssue, id, limit)
	if err != nil {
		failWith(c, err)
		return
	}
	sendOK(c, gin.H{"events": events})
}
