package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/andrey/cfa-bonds/internal/repo"
)

var (
	SettlementDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cfa_settlement_duration_seconds",
		Help:    "Time spent settling a single trade",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})
	SettlementOutcomes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cfa_settlement_outcomes_total",
		Help: "Settlement attempts by outcome",
	}, []string{"outcome"})
)

func ObserveSettlement(dur time.Duration, ok bool) {
	SettlementDuration.Observe(dur.Seconds())
	outcome := "settled"
	if !ok {
		outcome = "failed"
	}
	SettlementOutcomes.WithLabelValues(outcome).Inc()
}

type Collector struct {
	issues    *repo.IssueRepo
	trades    *repo.TradeRepo
	investors *repo.InvestorRepo
	coupons   *repo.CouponRepo
	events    *repo.EventRepo
	log       *slog.Logger
	timeout   time.Duration

	issuesByStatus *prometheus.Desc
	tradesTotal    *prometheus.Desc
	tradesVolume   *prometheus.Desc
	investorsTotal *prometheus.Desc
	couponsPaid    *prometheus.Desc
	couponsAmount  *prometheus.Desc
	eventsByType   *prometheus.Desc
}

type CollectorDeps struct {
	Issues    *repo.IssueRepo
	Trades    *repo.TradeRepo
	Investors *repo.InvestorRepo
	Coupons   *repo.CouponRepo
	Events    *repo.EventRepo
	Log       *slog.Logger
}

func NewCollector(d CollectorDeps) *Collector {
	return &Collector{
		issues:    d.Issues,
		trades:    d.Trades,
		investors: d.Investors,
		coupons:   d.Coupons,
		events:    d.Events,
		log:       d.Log,
		timeout:   5 * time.Second,
		issuesByStatus: prometheus.NewDesc("cfa_issues_total",
			"Number of bond issues by status", []string{"status"}, nil),
		tradesTotal: prometheus.NewDesc("cfa_trades_total",
			"Total number of settled trades", nil, nil),
		tradesVolume: prometheus.NewDesc("cfa_trades_volume_rub",
			"Total settled trade volume in rubles", nil, nil),
		investorsTotal: prometheus.NewDesc("cfa_investors_total",
			"Number of registered investors", nil, nil),
		couponsPaid: prometheus.NewDesc("cfa_coupons_paid_total",
			"Number of coupon payments made", nil, nil),
		couponsAmount: prometheus.NewDesc("cfa_coupons_paid_amount_rub",
			"Total coupon amount paid in rubles", nil, nil),
		eventsByType: prometheus.NewDesc("cfa_events_total",
			"Event log entries by type over the last 24h", []string{"type"}, nil),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.issuesByStatus
	ch <- c.tradesTotal
	ch <- c.tradesVolume
	ch <- c.investorsTotal
	ch <- c.couponsPaid
	ch <- c.couponsAmount
	ch <- c.eventsByType
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	if byStatus, err := c.issues.CountByStatus(ctx); err != nil {
		c.log.Warn("collect issues by status", "err", err)
	} else {
		for status, count := range byStatus {
			ch <- prometheus.MustNewConstMetric(c.issuesByStatus, prometheus.GaugeValue, float64(count), status)
		}
	}

	if cnt, vol, err := c.trades.CountSettled(ctx); err != nil {
		c.log.Warn("collect trade totals", "err", err)
	} else {
		volF, _ := vol.Float64()
		ch <- prometheus.MustNewConstMetric(c.tradesTotal, prometheus.CounterValue, float64(cnt))
		ch <- prometheus.MustNewConstMetric(c.tradesVolume, prometheus.CounterValue, volF)
	}

	if n, err := c.investors.CountAll(ctx); err != nil {
		c.log.Warn("collect investor count", "err", err)
	} else {
		ch <- prometheus.MustNewConstMetric(c.investorsTotal, prometheus.GaugeValue, float64(n))
	}

	if cnt, amt, err := c.coupons.PaidTotals(ctx); err != nil {
		c.log.Warn("collect coupon totals", "err", err)
	} else {
		amtF, _ := amt.Float64()
		ch <- prometheus.MustNewConstMetric(c.couponsPaid, prometheus.CounterValue, float64(cnt))
		ch <- prometheus.MustNewConstMetric(c.couponsAmount, prometheus.CounterValue, amtF)
	}

	since := time.Now().Add(-24 * time.Hour)
	if byType, err := c.events.CountByType(ctx, since); err != nil {
		c.log.Warn("collect events by type", "err", err)
	} else {
		for typ, count := range byType {
			ch <- prometheus.MustNewConstMetric(c.eventsByType, prometheus.CounterValue, float64(count), typ)
		}
	}
}
