package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/models"
)

type placeReq struct {
	InvestorID string `json:"investor_id" binding:"required"`
	Quantity   int64  `json:"quantity" binding:"required"`
	Price      string `json:"price" binding:"required"`
}

func (s *Server) placeIssue(c *gin.Context) {
	issueID, valid := pathUUID(c, "id")
	if !valid {
		return
	}
	var req placeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "invalid placement payload: "+err.Error())
		return
	}
	investorID, err := uuid.Parse(req.InvestorID)
	if err != nil {
		badRequest(c, "investor_id is not a valid uuid")
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

	ctx := c.Request.Context()
	issue, err := s.issues.Get(ctx, issueID)
	if err != nil {
		failWith(c, err)
		return
	}
	if issue.Status != models.IssuePlacement {
		c.JSON(http.StatusConflict, gin.H{"error": "issue must be in placement, current status " + issue.Status})
		return
	}

	cost := price.Mul(decimal.NewFromInt(req.Quantity))
	investor, err := s.investors.Get(ctx, investorID)
	if err != nil {
		failWith(c, err)
		return
	}
	if investor.Balance.LessThan(cost) {
		c.JSON(http.StatusConflict, gin.H{"error": "insufficient balance for placement"})
		return
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		failWith(c, err)
		return
	}
	defer tx.Rollback(ctx)

	placed, err := s.issues.IncrementPlaced(ctx, tx, issueID, req.Quantity)
	if err != nil {
		failWith(c, err)
		return
	}
	if err := s.investors.UpdateBalanceTx(ctx, tx, investorID, cost.Neg()); err != nil {
		failWith(c, err)
		return
	}

	existing, err := s.positions.GetByInvestorAndIssue(ctx, tx, investorID, issueID)
	pos := &models.Position{InvestorID: investorID, IssueID: issueID, Quantity: req.Quantity, AvgPrice: price}
	if err == nil {
		newQty := existing.Quantity + req.Quantity
		prevCost := existing.AvgPrice.Mul(decimal.NewFromInt(existing.Quantity))
		addCost := price.Mul(decimal.NewFromInt(req.Quantity))
		pos.ID = existing.ID
		pos.Quantity = newQty
		pos.AvgPrice = prevCost.Add(addCost).Div(decimal.NewFromInt(newQty)).Round(8)
	}
	if err := s.positions.Upsert(ctx, tx, pos); err != nil {
		failWith(c, err)
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"investor_id": investorID,
		"quantity":    req.Quantity,
		"price":       price.String(),
		"cost":        cost.String(),
	})
	if err := s.events.Append(ctx, tx, &models.EventLog{
		EntityType: models.EntityIssue,
		EntityID:   issueID,
		EventType:  models.EventIssuePlaced,
		Payload:    payload,
	}); err != nil {
		failWith(c, err)
		return
	}

	activated := false
	if placed >= issue.TotalQuantity {
		if _, err := tx.Exec(ctx, `UPDATE bond_issues SET status=$1, updated_at=now() WHERE id=$2 AND status=$3`,
			models.IssueActive, issueID, models.IssuePlacement); err != nil {
			failWith(c, err)
			return
		}
		activated = true
	}

	if err := tx.Commit(ctx); err != nil {
		failWith(c, err)
		return
	}

	if s.cache != nil {
		_ = s.cache.InvalidatePortfolio(ctx, investorID)
	}

	created(c, gin.H{
		"issue_id":        issueID,
		"investor_id":     investorID,
		"placed_quantity": placed,
		"total_quantity":  issue.TotalQuantity,
		"activated":       activated,
	})
}
