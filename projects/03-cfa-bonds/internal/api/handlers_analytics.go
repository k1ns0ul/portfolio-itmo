package api

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/andrey/cfa-bonds/internal/analytics"
	"github.com/andrey/cfa-bonds/internal/models"
)

func (s *Server) analyticsOverview(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now()

	byStatus, err := s.issues.CountByStatus(ctx)
	if err != nil {
		failWith(c, err)
		return
	}
	vol24, cnt24, err := s.trades.GlobalVolumeSince(ctx, now.Add(-24*time.Hour))
	if err != nil {
		failWith(c, err)
		return
	}
	vol7, cnt7, err := s.trades.GlobalVolumeSince(ctx, now.AddDate(0, 0, -7))
	if err != nil {
		failWith(c, err)
		return
	}
	vol30, cnt30, err := s.trades.GlobalVolumeSince(ctx, now.AddDate(0, 0, -30))
	if err != nil {
		failWith(c, err)
		return
	}
	investors, err := s.investors.CountAll(ctx)
	if err != nil {
		failWith(c, err)
		return
	}
	upcoming, err := s.coupons.UpcomingPayments(ctx, 10)
	if err != nil {
		failWith(c, err)
		return
	}

	sendOK(c, gin.H{
		"issues_by_status": byStatus,
		"investors_total":  investors,
		"trade_volume": gin.H{
			"24h": gin.H{"volume": vol24.String(), "count": cnt24},
			"7d":  gin.H{"volume": vol7.String(), "count": cnt7},
			"30d": gin.H{"volume": vol30.String(), "count": cnt30},
		},
		"upcoming_coupons": upcoming,
	})
}

func (s *Server) analyticsIssue(c *gin.Context) {
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

	vol30, cnt30, err := s.trades.GetVolumeByIssue(ctx, id, time.Now().AddDate(0, 0, -30))
	if err != nil {
		failWith(c, err)
		return
	}

	lastPrice, hasPrice, err := s.trades.LastPrice(ctx, id)
	if err != nil {
		failWith(c, err)
		return
	}
	priceForYield := lastPrice
	if !hasPrice {
		priceForYield = issue.Nominal
	}

	remaining := remainingPeriods(issue, time.Now())
	resp := gin.H{
		"issue_id":         id,
		"status":           issue.Status,
		"volume_30d":       vol30.String(),
		"trade_count_30d":  cnt30,
		"last_price":       priceForYield.String(),
		"has_market_price": hasPrice,
	}

	if cy, err := analytics.CurrentYield(issue.Nominal, issue.CouponRate, priceForYield); err == nil {
		resp["current_yield"] = cy.String()
	}
	if remaining > 0 {
		if ytm, err := analytics.CalculateYTM(issue.Nominal, priceForYield, issue.CouponRate, issue.CouponFrequency, remaining); err == nil {
			resp["ytm"] = ytm.String()
		}
		resp["remaining_periods"] = remaining
	}

	sendOK(c, resp)
}

func remainingPeriods(issue *models.BondIssue, now time.Time) int {
	if issue.Status == models.IssueMatured || !issue.MaturityDate.After(now) {
		return 0
	}
	monthsStep := 12 / issue.CouponFrequency
	count := 0
	for d := issue.MaturityDate; d.After(now); d = d.AddDate(0, -monthsStep, 0) {
		count++
		if count > 1000 {
			break
		}
	}
	if count == 0 {
		count = 1
	}
	return count
}
