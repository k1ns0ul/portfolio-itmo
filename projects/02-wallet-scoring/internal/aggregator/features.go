package aggregator

import (
	"math"
	"sort"
	"time"

	"github.com/andrey/wallet-scoring/internal/models"
	"github.com/andrey/wallet-scoring/internal/solana"
)

func TxCount(txs []models.Transaction) uint64 { return uint64(len(txs)) }

func TimeRange(txs []models.Transaction) (first, last time.Time) {
	if len(txs) == 0 {
		return
	}
	first = txs[0].BlockTime
	last = txs[0].BlockTime
	for _, t := range txs[1:] {
		if t.BlockTime.Before(first) {
			first = t.BlockTime
		}
		if t.BlockTime.After(last) {
			last = t.BlockTime
		}
	}
	return
}

func UniqueCounterparties(addr string, txs []models.Transaction) uint32 {
	set := make(map[string]struct{}, len(txs))
	for _, t := range txs {
		cp := t.Counterparty(addr)
		if cp != "" && cp != addr {
			set[cp] = struct{}{}
		}
	}
	return uint32(len(set))
}

func AvgAmount(txs []models.Transaction) float64 {
	if len(txs) == 0 {
		return 0
	}
	var sum uint64
	var n int
	for _, t := range txs {
		if t.Amount == 0 {
			continue
		}
		sum += t.Amount
		n++
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

func MedianAmount(txs []models.Transaction) float64 {
	if len(txs) == 0 {
		return 0
	}
	xs := make([]uint64, 0, len(txs))
	for _, t := range txs {
		if t.Amount > 0 {
			xs = append(xs, t.Amount)
		}
	}
	if len(xs) == 0 {
		return 0
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	mid := len(xs) / 2
	if len(xs)%2 == 1 {
		return float64(xs[mid])
	}
	return (float64(xs[mid-1]) + float64(xs[mid])) / 2
}

func Herfindahl(addr string, txs []models.Transaction) float64 {
	totals := make(map[string]uint64)
	var total uint64
	for _, t := range txs {
		cp := t.Counterparty(addr)
		if cp == "" || cp == addr || t.Amount == 0 {
			continue
		}
		totals[cp] += t.Amount
		total += t.Amount
	}
	if total == 0 || len(totals) == 0 {
		return 0
	}
	var hi float64
	for _, v := range totals {
		share := float64(v) / float64(total)
		hi += share * share
	}
	return hi
}

func SmartContractRatio(txs []models.Transaction) float32 {
	if len(txs) == 0 {
		return 0
	}
	var sc int
	for _, t := range txs {
		if solana.IsSmartContract(t.ProgramID) {
			sc++
		}
	}
	return float32(sc) / float32(len(txs))
}

func VelocityPerHour(txs []models.Transaction, now time.Time) float64 {
	if len(txs) == 0 {
		return 0
	}
	from := now.Add(-24 * time.Hour)
	var n int
	for _, t := range txs {
		if !t.BlockTime.Before(from) {
			n++
		}
	}
	return float64(n) / 24.0
}

func DormancyDays(last time.Time, now time.Time) float64 {
	if last.IsZero() {
		return 0
	}
	d := now.Sub(last)
	if d < 0 {
		return 0
	}
	return d.Hours() / 24.0
}

func ScoreFromFeatures(s models.WalletStats) (float32, models.Category) {
	score := 100.0
	score -= 30 * math.Min(1, s.HerfindahlIndex)
	score -= 25 * math.Min(1, s.VelocityPerHour/10)
	score -= 20 * float64(s.SmartContractRatio)
	score -= 15 * math.Min(1, s.DormancyDays/365)
	score += 10 * math.Min(1, float64(s.UniqueCounterparties)/100)

	if s.TxCount < 5 {
		score -= 10
	}
	score = math.Max(0, math.Min(100, score))

	cat := models.CategoryLegit
	switch {
	case score < 25:
		cat = models.CategoryScam
	case score < 55:
		cat = models.CategorySuspicious
	}
	return float32(score), cat
}
