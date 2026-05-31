package mockgen

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math"
	mrand "math/rand"
	"strconv"
	"time"

	"github.com/andrey/orderflow-intelligence/internal/config"
	"github.com/andrey/orderflow-intelligence/internal/kafka"
	"github.com/andrey/orderflow-intelligence/internal/models"
)

type pairState struct {
	name      string
	price     float64
	trend     float64
	trendEnd  time.Time
	priceUnit float64
}

type Generator struct {
	cfg      config.MockGenConfig
	producer *kafka.Producer
	topic    string
	rng      *mrand.Rand
	pairs    []*pairState
}

func New(cfg config.MockGenConfig, producer *kafka.Producer, topic string) *Generator {
	g := &Generator{
		cfg:      cfg,
		producer: producer,
		topic:    topic,
		rng:      mrand.New(mrand.NewSource(time.Now().UnixNano())),
	}
	g.seedPairs()
	return g
}

func (g *Generator) seedPairs() {
	defs := []struct {
		name      string
		basePrice float64
	}{
		{"SOL-USDC", 150.0},
		{"SOL-USDT", 150.2},
		{"RAY-SOL", 0.018},
		{"ORCA-SOL", 0.012},
		{"JUP-SOL", 0.0042},
	}
	now := time.Now().UTC()
	for _, d := range defs {
		g.pairs = append(g.pairs, &pairState{
			name:      d.name,
			price:     d.basePrice,
			trend:     g.pickTrend(),
			trendEnd:  now.Add(g.nextTrendDuration()),
			priceUnit: d.basePrice,
		})
	}
}

func (g *Generator) Run(ctx context.Context) error {
	if g.cfg.TPS <= 0 {
		g.cfg.TPS = 20
	}
	interval := time.Second / time.Duration(g.cfg.TPS)
	t := time.NewTicker(interval)
	defer t.Stop()

	var deadline <-chan time.Time
	if g.cfg.DurationSec > 0 {
		deadline = time.After(time.Duration(g.cfg.DurationSec) * time.Second)
	}

	slog.Info("mockgen started",
		"tps", g.cfg.TPS,
		"pairs", len(g.pairs),
	)
	var produced uint64
	for {
		select {
		case <-ctx.Done():
			slog.Info("mockgen stopped", "produced", produced)
			return nil
		case <-deadline:
			slog.Info("mockgen finished", "produced", produced)
			return nil
		case <-t.C:
			g.emit()
			produced++
		}
	}
}

func (g *Generator) emit() {
	pair := g.pairs[g.rng.Intn(len(g.pairs))]
	now := time.Now().UTC()
	g.maybeRotateTrend(pair, now)
	g.updatePrice(pair)

	direction := models.DirBuy
	threshold := 0.55 + pair.trend*0.20
	if g.rng.Float64() > threshold {
		direction = models.DirSell
	}

	amountIn := uint64(g.lognormal(8.0, 1.0))
	if g.rng.Float64() < 0.02 {
		amountIn *= 5
	}
	amountOut := uint64(float64(amountIn) * pair.price)

	swap := models.SwapEvent{
		Signature:   newSig(g.rng),
		Slot:        uint64(now.Unix()),
		BlockTime:   now,
		Dex:         g.pickDEX(),
		PoolAddress: "pool-" + pair.name,
		Pair:        pair.name,
		TokenIn:     pair.name + "-base",
		TokenOut:    pair.name + "-quote",
		AmountIn:    amountIn,
		AmountOut:   amountOut,
		Price:       pair.price,
		Direction:   direction,
		Sender:      "wallet-" + strconv.Itoa(g.rng.Intn(2000)),
	}
	payload, err := json.Marshal(swap)
	if err != nil {
		return
	}
	g.producer.Send(g.topic, []byte(pair.name), payload)
}

func (g *Generator) updatePrice(p *pairState) {
	step := g.rng.NormFloat64() * 0.0008
	step += p.trend * 0.0006
	p.price *= 1.0 + step
	if p.price < p.priceUnit*0.2 {
		p.price = p.priceUnit * 0.2
	}
	if p.price > p.priceUnit*5 {
		p.price = p.priceUnit * 5
	}
}

func (g *Generator) maybeRotateTrend(p *pairState, now time.Time) {
	if now.Before(p.trendEnd) {
		return
	}
	p.trend = g.pickTrend()
	p.trendEnd = now.Add(g.nextTrendDuration())
}

func (g *Generator) pickTrend() float64 {
	r := g.rng.Float64()
	if r < 0.3 {
		return 1.0
	}
	if r < 0.6 {
		return -1.0
	}
	return 0.0
}

func (g *Generator) nextTrendDuration() time.Duration {
	return time.Duration(5+g.rng.Intn(11)) * time.Minute
}

func (g *Generator) pickDEX() string {
	options := []string{"raydium-v4", "orca-whirlpool", "jupiter-v6"}
	return options[g.rng.Intn(len(options))]
}

func (g *Generator) lognormal(mu, sigma float64) float64 {
	return math.Exp(mu + sigma*g.rng.NormFloat64())
}

func newSig(rng *mrand.Rand) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
	}
	return "sig-" + hex.EncodeToString(b[:])
}
