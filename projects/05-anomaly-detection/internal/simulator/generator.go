package simulator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"strconv"
	"time"

	"github.com/andrey/anomaly-detection/internal/config"
	"github.com/andrey/anomaly-detection/internal/kafka"
	"github.com/andrey/anomaly-detection/internal/models"
)

type profile string

const (
	profileNormal     profile = "normal"
	profileSuspicious profile = "suspicious"
	profileFraud      profile = "fraud"
)

type clientState struct {
	id         string
	prof       profile
	avgAmount  float64
	nextBurst  time.Time
	burstLeft  int
	burstDelay time.Duration
}

type Generator struct {
	cfg       config.SimulatorConfig
	producer  *kafka.Producer
	topic     string

	clients        []*clientState
	counterparties []string
	categories     []string
	channels       []string
	rng            *mrand.Rand
}

func New(cfg config.SimulatorConfig, producer *kafka.Producer, topic string) *Generator {
	g := &Generator{
		cfg:        cfg,
		producer:   producer,
		topic:      topic,
		rng:        mrand.New(mrand.NewSource(time.Now().UnixNano())),
		categories: []string{"groceries", "transport", "entertainment", "utilities", "restaurants", "shopping"},
		channels:   []string{"card", "mobile", "web", "atm"},
	}
	g.seedClients()
	g.seedCounterparties()
	return g
}

func (g *Generator) seedClients() {
	pool := g.cfg.ClientPool
	if pool <= 0 {
		pool = 1000
	}
	suspiciousCount := int(float64(pool) * g.cfg.SuspiciousRatio)
	fraudCount := int(float64(pool) * g.cfg.FraudRatio)

	now := time.Now().UTC()
	g.clients = make([]*clientState, 0, pool)
	for i := 0; i < pool; i++ {
		prof := profileNormal
		if i < fraudCount {
			prof = profileFraud
		} else if i < fraudCount+suspiciousCount {
			prof = profileSuspicious
		}
		avg := 1000 + g.rng.Float64()*15000
		g.clients = append(g.clients, &clientState{
			id:        fmt.Sprintf("client-%05d", i),
			prof:      prof,
			avgAmount: avg,
			nextBurst: now.Add(time.Duration(g.rng.Intn(1800)) * time.Second),
		})
	}
}

func (g *Generator) seedCounterparties() {
	pool := g.cfg.CounterpartyPool
	if pool <= 0 {
		pool = 200
	}
	g.counterparties = make([]string, 0, pool)
	for i := 0; i < pool; i++ {
		g.counterparties = append(g.counterparties, fmt.Sprintf("cp-%04d", i))
	}
}

func (g *Generator) Run(ctx context.Context) error {
	if g.cfg.TPS <= 0 {
		g.cfg.TPS = 50
	}
	interval := time.Second / time.Duration(g.cfg.TPS)
	t := time.NewTicker(interval)
	defer t.Stop()

	var deadline <-chan time.Time
	if g.cfg.DurationSec > 0 {
		deadline = time.After(time.Duration(g.cfg.DurationSec) * time.Second)
	}

	slog.Info("simulator started",
		"tps", g.cfg.TPS,
		"clients", len(g.clients),
		"counterparties", len(g.counterparties),
	)

	var produced uint64
	for {
		select {
		case <-ctx.Done():
			slog.Info("simulator stopped", "produced", produced)
			return nil
		case <-deadline:
			slog.Info("simulator finished", "produced", produced)
			return nil
		case <-t.C:
			g.emitOne()
			produced++
		}
	}
}

func (g *Generator) emitOne() {
	client := g.clients[g.rng.Intn(len(g.clients))]
	tx := g.assemble(client)
	payload, err := json.Marshal(tx)
	if err != nil {
		return
	}
	g.producer.Send(g.topic, []byte(tx.ClientID), payload)
}

func (g *Generator) assemble(c *clientState) models.Transaction {
	now := time.Now().UTC()
	var amount float64
	var cp string

	switch c.prof {
	case profileFraud:
		if c.burstLeft > 0 {
			amount = c.avgAmount * (10 + g.rng.Float64()*15)
			cp = randomID(g.rng, "new-cp")
			c.burstLeft--
		} else if !c.nextBurst.IsZero() && now.After(c.nextBurst) {
			c.burstLeft = 10
			c.burstDelay = 6 * time.Second
			c.nextBurst = now.Add(30 * time.Minute)
			amount = c.avgAmount * (10 + g.rng.Float64()*15)
			cp = randomID(g.rng, "new-cp")
		} else {
			amount = c.avgAmount * (0.5 + g.rng.Float64())
			cp = g.counterparties[g.rng.Intn(len(g.counterparties))]
		}
	case profileSuspicious:
		if c.burstLeft > 0 {
			amount = 100 + g.rng.Float64()*200
			cp = g.counterparties[g.rng.Intn(len(g.counterparties))]
			c.burstLeft--
		} else if !c.nextBurst.IsZero() && now.After(c.nextBurst) {
			c.burstLeft = 20
			c.nextBurst = now.Add(time.Duration(30+g.rng.Intn(30)) * time.Minute)
			amount = 100 + g.rng.Float64()*200
			cp = g.counterparties[g.rng.Intn(len(g.counterparties))]
		} else {
			now = now.Add(time.Duration(g.rng.Intn(6)-3) * time.Hour)
			amount = c.avgAmount * (0.5 + g.rng.Float64())
			cp = g.counterparties[g.rng.Intn(len(g.counterparties))]
		}
	default:
		amount = c.avgAmount * (0.3 + g.rng.Float64()*1.4)
		cp = g.counterparties[g.rng.Intn(len(g.counterparties))]
	}

	return models.Transaction{
		ID:             newTxID(g.rng),
		ClientID:       c.id,
		Amount:         round2(amount),
		CounterpartyID: cp,
		Timestamp:      now,
		Category:       g.categories[g.rng.Intn(len(g.categories))],
		Channel:        g.channels[g.rng.Intn(len(g.channels))],
	}
}

func newTxID(rng *mrand.Rand) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
	}
	return "tx-" + hex.EncodeToString(b[:])
}

func randomID(rng *mrand.Rand, prefix string) string {
	return prefix + "-" + strconv.Itoa(rng.Intn(1_000_000))
}

func round2(x float64) float64 {
	return float64(int64(x*100)) / 100
}
