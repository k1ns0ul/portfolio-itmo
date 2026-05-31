package metrics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/andrey/portfolio-reports/internal/clickhouse"
	"github.com/andrey/portfolio-reports/internal/models"
)

type Calculator struct {
	repo *clickhouse.Repo
}

func New(repo *clickhouse.Repo) *Calculator {
	return &Calculator{repo: repo}
}

func (c *Calculator) CalculatePortfolio(ctx context.Context, address string) (*models.Portfolio, error) {
	balances, err := c.repo.GetTokenBalances(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("fetch balances: %w", err)
	}
	if len(balances) == 0 {
		return nil, fmt.Errorf("empty portfolio for %s", address)
	}

	pnlList, err := c.repo.GetPnLByToken(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("fetch pnl: %w", err)
	}
	pnlByMint := make(map[string]models.TokenPnL, len(pnlList))
	for _, p := range pnlList {
		pnlByMint[p.Mint] = p
	}

	mints := make([]string, 0, len(balances))
	for _, b := range balances {
		mints = append(mints, b.Mint)
	}
	tokenScores, err := c.repo.GetTokenScores(ctx, mints)
	if err != nil {
		return nil, fmt.Errorf("fetch token scores: %w", err)
	}

	positions, totalValue := c.buildPositions(balances, pnlByMint, tokenScores)

	c.fillShares(positions, totalValue)

	pnl := c.aggregatePnL(positions, pnlByMint)
	div := c.diversification(positions, totalValue)
	risk := c.risk(positions, div)

	return &models.Portfolio{
		Address:         address,
		TotalValue:      totalValue,
		Tokens:          positions,
		PnL:             pnl,
		Diversification: div,
		Risk:            risk,
		GeneratedAt:     time.Now().UTC(),
	}, nil
}

func (c *Calculator) WalletCategory(ctx context.Context, address string) (string, error) {
	ws, err := c.repo.GetWalletScore(ctx, address)
	if err != nil {
		if errors.Is(err, clickhouse.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return ws.Category, nil
}

func (c *Calculator) buildPositions(
	balances []clickhouse.TokenBalance,
	pnlByMint map[string]models.TokenPnL,
	tokenScores map[string]clickhouse.TokenScore,
) ([]models.TokenPosition, float64) {
	positions := make([]models.TokenPosition, 0, len(balances))
	total := 0.0
	for _, b := range balances {
		value := b.Amount * b.LastPrice
		total += value
		_, isStable := models.StablecoinMints[b.Mint]
		_, isBlue := models.BlueChipMints[b.Mint]
		symbol := b.Symbol
		if symbol == "" {
			if s, ok := models.StablecoinMints[b.Mint]; ok {
				symbol = s
			} else if s, ok := models.BlueChipMints[b.Mint]; ok {
				symbol = s
			}
		}
		risk := "legit"
		if s, ok := tokenScores[b.Mint]; ok && s.Category != "" {
			risk = s.Category
		}
		pos := models.TokenPosition{
			Mint:         b.Mint,
			Symbol:       symbol,
			Amount:       b.Amount,
			ValueUSD:     value,
			IsStablecoin: isStable,
			IsBlueChip:   isBlue,
			RiskCategory: risk,
		}
		if p, ok := pnlByMint[b.Mint]; ok {
			pos.PnL = p.RealizedPnL + p.UnrealizedPnL
			cost := p.AvgBuyPrice * p.Quantity
			if cost > 0 {
				pos.PnLPct = pos.PnL / cost * 100.0
			}
		}
		positions = append(positions, pos)
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].ValueUSD > positions[j].ValueUSD })
	return positions, total
}

func (c *Calculator) fillShares(positions []models.TokenPosition, total float64) {
	if total <= 0 {
		return
	}
	for i := range positions {
		positions[i].PctOfPortfolio = positions[i].ValueUSD / total * 100.0
	}
}

func (c *Calculator) aggregatePnL(positions []models.TokenPosition, pnlByMint map[string]models.TokenPnL) models.PnLData {
	var realized, unrealized, cost float64
	for _, p := range pnlByMint {
		realized += p.RealizedPnL
		unrealized += p.UnrealizedPnL
		cost += p.AvgBuyPrice * p.Quantity
	}
	total := realized + unrealized
	pct := 0.0
	if cost > 0 {
		pct = total / cost * 100.0
	}
	return models.PnLData{
		Realized:   realized,
		Unrealized: unrealized,
		Total:      total,
		PctReturn:  pct,
		CostBasis:  cost,
	}
}

func (c *Calculator) diversification(positions []models.TokenPosition, total float64) models.DiversificationData {
	herf := 0.0
	stableValue := 0.0
	blueValue := 0.0
	top := 0.0
	for _, p := range positions {
		share := p.PctOfPortfolio / 100.0
		herf += share * share
		if p.IsStablecoin {
			stableValue += p.ValueUSD
		}
		if p.IsBlueChip {
			blueValue += p.ValueUSD
		}
		if p.PctOfPortfolio > top {
			top = p.PctOfPortfolio
		}
	}
	level := models.ConcentrationHigh
	switch {
	case herf < 0.15:
		level = models.ConcentrationLow
	case herf < 0.40:
		level = models.ConcentrationMedium
	}
	stablePct := 0.0
	bluePct := 0.0
	if total > 0 {
		stablePct = stableValue / total * 100.0
		bluePct = blueValue / total * 100.0
	}
	return models.DiversificationData{
		HerfindahlIndex:    herf,
		TokenCount:         len(positions),
		StablecoinPct:      stablePct,
		BlueChipPct:        bluePct,
		TopTokenPct:        top,
		ConcentrationLevel: level,
	}
}

func (c *Calculator) risk(positions []models.TokenPosition, div models.DiversificationData) models.RiskData {
	if len(positions) == 0 {
		return models.RiskData{Score: 0, Level: models.RiskLow}
	}
	volatile := 0.0
	scam := 0.0
	suspicious := 0.0
	total := 0.0
	for _, p := range positions {
		total += p.ValueUSD
		if !p.IsStablecoin && !p.IsBlueChip {
			volatile += p.ValueUSD
		}
		switch p.RiskCategory {
		case "scam":
			scam += p.ValueUSD
		case "suspicious":
			suspicious += p.ValueUSD
		}
	}
	if total <= 0 {
		total = 1
	}
	volatilePct := volatile / total
	scamPct := (scam + 0.5*suspicious) / total
	concentration := div.HerfindahlIndex
	score := 0.3*concentration + 0.3*volatilePct + 0.4*scamPct
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	var factors []string
	if div.TopTokenPct > 60 {
		factors = append(factors, "высокая концентрация в одном токене")
	}
	if div.ConcentrationLevel == models.ConcentrationHigh {
		factors = append(factors, "низкая диверсификация")
	}
	if scamPct > 0.01 {
		factors = append(factors, "есть подозрительные или скам токены")
	}
	if div.StablecoinPct < 5 {
		factors = append(factors, "нет защитной позиции в стейблкоинах")
	}
	if volatilePct > 0.8 {
		factors = append(factors, "портфель полностью в волатильных активах")
	}

	level := models.RiskLow
	switch {
	case score >= 0.7:
		level = models.RiskCritical
	case score >= 0.45:
		level = models.RiskHigh
	case score >= 0.25:
		level = models.RiskMedium
	}

	return models.RiskData{
		Score:        score * 100,
		Level:        level,
		VolatilePct:  volatilePct * 100,
		ScamTokenPct: scamPct * 100,
		Factors:      factors,
	}
}
