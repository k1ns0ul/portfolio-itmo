package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/andrey/cfa-bonds/internal/repo"
)

func repoIssueFilter(c *gin.Context, limit, offset int) repo.IssueFilter {
	f := repo.IssueFilter{Status: c.Query("status"), Limit: limit, Offset: offset}
	if raw := c.Query("issuer_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			f.IssuerID = id
		}
	}
	return f
}

func repoTradeFilter(c *gin.Context, limit, offset int) repo.TradeFilter {
	f := repo.TradeFilter{Status: c.Query("status"), Limit: limit, Offset: offset}
	if raw := c.Query("investor_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			f.InvestorID = id
		}
	}
	if raw := c.Query("issue_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			f.IssueID = id
		}
	}
	return f
}
