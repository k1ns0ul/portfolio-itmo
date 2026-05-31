package mockdata

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"strings"

	"github.com/andrey/portfolio-reports/internal/clickhouse"
	"github.com/andrey/portfolio-reports/internal/config"
	"github.com/andrey/portfolio-reports/internal/models"
)

type profile string

const (
	profileWhale     profile = "whale"
	profileTrader    profile = "active_trader"
	profileHodler    profile = "hodler"
	profileDegen     profile = "degen"
	profileNewWallet profile = "new_wallet"
)

type Generator struct {
	cfg  config.MockDataConfig
	repo *clickhouse.Repo

	stableMints []string
	blueMints   []string
	scamMints   []string
	otherMints  []string

	rng *mrand.Rand
}

func New(cfg config.MockDataConfig, repo *clickhouse.Repo) *Generator {
	g := &Generator{
		cfg:  cfg,
		repo: repo,
		rng:  mrand.New(mrand.NewSource(42)),
	}
	g.buildTokenPools()
	return g
}

func (g *Generator) buildTokenPools() {
	for mint := range models.StablecoinMints {
		g.stableMints = append(g.stableMints, mint)
	}
	for mint := range models.BlueChipMints {
		g.blueMints = append(g.blueMints, mint)
	}
	tokenPool := g.cfg.TokenPool
	if tokenPool <= 0 {
		tokenPool = 100
	}
	scamCount := tokenPool / 5
	otherCount := tokenPool - scamCount
	for i := 0; i < scamCount; i++ {
		g.scamMints = append(g.scamMints, "Scam"+randSuffix(g.rng))
	}
	for i := 0; i < otherCount; i++ {
		g.otherMints = append(g.otherMints, "Tk"+randSuffix(g.rng))
	}
}

func (g *Generator) Run(ctx context.Context) error {
	if g.cfg.Wallets <= 0 {
		g.cfg.Wallets = 200
	}
	if err := g.seedTokenScores(ctx); err != nil {
		return fmt.Errorf("seed token scores: %w", err)
	}
	for i := 0; i < g.cfg.Wallets; i++ {
		address := randomAddress(g.rng)
		prof := g.pickProfile()
		balances, pnl := g.makePortfolio(prof)
		if err := g.repo.UpsertTokenBalances(ctx, address, balances); err != nil {
			return fmt.Errorf("upsert balances: %w", err)
		}
		if err := g.repo.UpsertPnL(ctx, address, pnl); err != nil {
			return fmt.Errorf("upsert pnl: %w", err)
		}
		if i%20 == 0 {
			slog.Info("mock progress", "wallets_done", i, "profile", string(prof))
		}
	}
	slog.Info("mock done", "wallets", g.cfg.Wallets)
	return nil
}

func (g *Generator) seedTokenScores(ctx context.Context) error {
	scores := make([]clickhouse.TokenScore, 0, len(g.scamMints)+len(g.blueMints)+len(g.stableMints))
	for _, m := range g.scamMints {
		category := "scam"
		if g.rng.Float64() < 0.3 {
			category = "suspicious"
		}
		scores = append(scores, clickhouse.TokenScore{Mint: m, Category: category})
	}
	for _, m := range g.blueMints {
		scores = append(scores, clickhouse.TokenScore{Mint: m, Category: "legit"})
	}
	for _, m := range g.stableMints {
		scores = append(scores, clickhouse.TokenScore{Mint: m, Category: "legit"})
	}
	return g.repo.UpsertTokenScores(ctx, scores)
}

func (g *Generator) pickProfile() profile {
	r := g.rng.Float64()
	c := g.cfg
	thresholds := []struct {
		p   profile
		cum float64
	}{
		{profileWhale, c.WhalePct},
		{profileTrader, c.WhalePct + c.TraderPct},
		{profileHodler, c.WhalePct + c.TraderPct + c.HodlerPct},
		{profileDegen, c.WhalePct + c.TraderPct + c.HodlerPct + c.DegenPct},
	}
	for _, t := range thresholds {
		if r < t.cum {
			return t.p
		}
	}
	return profileNewWallet
}

func (g *Generator) makePortfolio(p profile) ([]clickhouse.TokenBalance, []models.TokenPnL) {
	switch p {
	case profileWhale:
		return g.whalePortfolio()
	case profileTrader:
		return g.traderPortfolio()
	case profileHodler:
		return g.hodlerPortfolio()
	case profileDegen:
		return g.degenPortfolio()
	default:
		return g.newWalletPortfolio()
	}
}

func (g *Generator) whalePortfolio() ([]clickhouse.TokenBalance, []models.TokenPnL) {
	bs := make([]clickhouse.TokenBalance, 0, 12)
	bs = append(bs, g.balance(g.pickStable(), 50_000+g.rng.Float64()*200_000, 1.0))
	bs = append(bs, g.balance(g.pickBlue(), 30+g.rng.Float64()*100, 120+g.rng.Float64()*40))
	for i := 0; i < 8+g.rng.Intn(3); i++ {
		bs = append(bs, g.balance(g.pickOther(), 100+g.rng.Float64()*10_000, 0.1+g.rng.Float64()*5))
	}
	return bs, g.derivedPnL(bs, 0.7)
}

func (g *Generator) traderPortfolio() ([]clickhouse.TokenBalance, []models.TokenPnL) {
	bs := make([]clickhouse.TokenBalance, 0, 8)
	bs = append(bs, g.balance(g.pickStable(), 5_000+g.rng.Float64()*20_000, 1.0))
	bs = append(bs, g.balance(g.pickBlue(), 5+g.rng.Float64()*30, 120+g.rng.Float64()*40))
	for i := 0; i < 4+g.rng.Intn(4); i++ {
		bs = append(bs, g.balance(g.pickOther(), 50+g.rng.Float64()*3_000, 0.05+g.rng.Float64()*3))
	}
	if g.rng.Float64() < 0.4 {
		bs = append(bs, g.balance(g.pickScam(), 1_000+g.rng.Float64()*5_000, 0.01))
	}
	return bs, g.derivedPnL(bs, 0.5)
}

func (g *Generator) hodlerPortfolio() ([]clickhouse.TokenBalance, []models.TokenPnL) {
	bs := make([]clickhouse.TokenBalance, 0, 4)
	bs = append(bs, g.balance(g.pickBlue(), 1+g.rng.Float64()*15, 120+g.rng.Float64()*40))
	bs = append(bs, g.balance(g.pickStable(), 500+g.rng.Float64()*5_000, 1.0))
	if g.rng.Float64() < 0.5 {
		bs = append(bs, g.balance(g.pickBlue(), 0.5+g.rng.Float64()*5, 120+g.rng.Float64()*40))
	}
	return bs, g.derivedPnL(bs, 0.85)
}

func (g *Generator) degenPortfolio() ([]clickhouse.TokenBalance, []models.TokenPnL) {
	bs := make([]clickhouse.TokenBalance, 0, 12)
	for i := 0; i < 8+g.rng.Intn(5); i++ {
		mint := g.pickOther()
		if g.rng.Float64() < 0.4 {
			mint = g.pickScam()
		}
		bs = append(bs, g.balance(mint, 100+g.rng.Float64()*5_000, 0.001+g.rng.Float64()))
	}
	return bs, g.derivedPnL(bs, 0.2)
}

func (g *Generator) newWalletPortfolio() ([]clickhouse.TokenBalance, []models.TokenPnL) {
	bs := []clickhouse.TokenBalance{
		g.balance(g.pickStable(), 50+g.rng.Float64()*500, 1.0),
	}
	if g.rng.Float64() < 0.6 {
		bs = append(bs, g.balance(g.pickBlue(), 0.1+g.rng.Float64()*2, 120+g.rng.Float64()*40))
	}
	return bs, g.derivedPnL(bs, 0.6)
}

func (g *Generator) balance(mint string, amount, price float64) clickhouse.TokenBalance {
	return clickhouse.TokenBalance{
		Mint:      mint,
		Symbol:    g.symbolFor(mint),
		Amount:    round4(amount),
		LastPrice: round4(price),
	}
}

func (g *Generator) symbolFor(mint string) string {
	if s, ok := models.StablecoinMints[mint]; ok {
		return s
	}
	if s, ok := models.BlueChipMints[mint]; ok {
		return s
	}
	if strings.HasPrefix(mint, "Scam") {
		return "SCM"
	}
	return "TK"
}

func (g *Generator) derivedPnL(bs []clickhouse.TokenBalance, winRate float64) []models.TokenPnL {
	out := make([]models.TokenPnL, 0, len(bs))
	for _, b := range bs {
		buy := b.LastPrice * (0.6 + g.rng.Float64()*0.6)
		quantity := b.Amount * (0.9 + g.rng.Float64()*0.2)
		realized := (b.LastPrice - buy) * quantity * 0.3
		unrealized := (b.LastPrice - buy) * quantity * 0.7
		if g.rng.Float64() > winRate {
			realized = -abs(realized)
			unrealized = -abs(unrealized)
		}
		out = append(out, models.TokenPnL{
			Mint:          b.Mint,
			AvgBuyPrice:   round4(buy),
			CurrentPrice:  b.LastPrice,
			Quantity:      quantity,
			RealizedPnL:   round4(realized),
			UnrealizedPnL: round4(unrealized),
		})
	}
	return out
}

func (g *Generator) pickStable() string { return g.stableMints[g.rng.Intn(len(g.stableMints))] }
func (g *Generator) pickBlue() string   { return g.blueMints[g.rng.Intn(len(g.blueMints))] }
func (g *Generator) pickScam() string   { return g.scamMints[g.rng.Intn(len(g.scamMints))] }
func (g *Generator) pickOther() string  { return g.otherMints[g.rng.Intn(len(g.otherMints))] }

func randomAddress(rng *mrand.Rand) string {
	var b [22]byte
	if _, err := rand.Read(b[:]); err != nil {
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
	}
	out := hex.EncodeToString(b[:])
	if len(out) < 32 {
		out += "0000000000"
	}
	return out[:44]
}

func randSuffix(rng *mrand.Rand) string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
	}
	return hex.EncodeToString(b[:])
}

func round4(x float64) float64 {
	return float64(int64(x*10000)) / 10000
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
