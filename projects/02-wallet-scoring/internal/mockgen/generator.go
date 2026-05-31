package mockgen

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	mrand "math/rand"
	"sync"
	"time"

	"github.com/andrey/wallet-scoring/internal/config"
	"github.com/andrey/wallet-scoring/internal/kafka"
	"github.com/andrey/wallet-scoring/internal/models"
	"github.com/andrey/wallet-scoring/internal/solana"
)

type Generator struct {
	cfg      config.MockGenConfig
	producer *kafka.Producer
	topic    string

	wallets        []string
	scamWallets    []string
	pumpDumpPools  []poolGroup
	washTradeRings [][]int

	rng *mrand.Rand
	mu  sync.Mutex
}

type poolGroup struct {
	pool    string
	buyers  []int
	dumpers []int
}

func New(cfg config.MockGenConfig, producer *kafka.Producer, topic string) *Generator {
	g := &Generator{
		cfg:      cfg,
		producer: producer,
		topic:    topic,
		rng:      mrand.New(mrand.NewSource(time.Now().UnixNano())),
	}
	g.seedWallets()
	g.seedPumpPools()
	g.seedWashRings()
	return g
}

func (g *Generator) seedWallets() {
	g.wallets = make([]string, g.cfg.WalletPoolSize)
	for i := range g.wallets {
		g.wallets[i] = solana.Base58Encode(randBytes(g.rng, 32))
	}
	scamCount := int(float64(g.cfg.WalletPoolSize) * g.cfg.ScamRatio)
	g.scamWallets = make([]string, 0, scamCount)
	for i := 0; i < scamCount; i++ {
		idx := g.rng.Intn(len(g.wallets))
		g.scamWallets = append(g.scamWallets, g.wallets[idx])
	}
}

func (g *Generator) seedPumpPools() {
	const pools = 10
	g.pumpDumpPools = make([]poolGroup, pools)
	for i := 0; i < pools; i++ {
		pool := solana.Base58Encode(randBytes(g.rng, 32))
		group := poolGroup{pool: pool}
		for j := 0; j < 40; j++ {
			group.buyers = append(group.buyers, g.rng.Intn(len(g.wallets)))
		}
		for j := 0; j < 10; j++ {
			group.dumpers = append(group.dumpers, g.rng.Intn(len(g.wallets)))
		}
		g.pumpDumpPools[i] = group
	}
}

func (g *Generator) seedWashRings() {
	const rings = 20
	g.washTradeRings = make([][]int, rings)
	for i := 0; i < rings; i++ {
		size := 3 + g.rng.Intn(3)
		ring := make([]int, size)
		for j := 0; j < size; j++ {
			ring[j] = g.rng.Intn(len(g.wallets))
		}
		g.washTradeRings[i] = ring
	}
}

func (g *Generator) Run(ctx context.Context) error {
	if g.cfg.TPS <= 0 {
		g.cfg.TPS = 100
	}
	tick := time.Second / time.Duration(g.cfg.TPS)
	t := time.NewTicker(tick)
	defer t.Stop()

	var deadline <-chan time.Time
	if g.cfg.DurationSec > 0 {
		deadline = time.After(time.Duration(g.cfg.DurationSec) * time.Second)
	}

	slog.Info("mockgen running",
		"tps", g.cfg.TPS,
		"wallets", g.cfg.WalletPoolSize,
		"scam_ratio", g.cfg.ScamRatio,
		"suspicious_ratio", g.cfg.SuspiciousRate,
		"duration_sec", g.cfg.DurationSec,
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
			tx := g.nextTransaction()
			payload, err := g.encode(tx)
			if err != nil {
				continue
			}
			g.producer.Send(g.topic, []byte(tx.Signature), payload)
			produced++
		}
	}
}

func (g *Generator) nextTransaction() models.Transaction {
	roll := g.rng.Float64()
	switch {
	case roll < g.cfg.ScamRatio:
		return g.scamTx()
	case roll < g.cfg.ScamRatio+g.cfg.SuspiciousRate:
		return g.suspiciousTx()
	default:
		return g.normalTx()
	}
}

func (g *Generator) normalTx() models.Transaction {
	from := g.wallets[g.rng.Intn(len(g.wallets))]
	to := g.wallets[g.rng.Intn(len(g.wallets))]
	for from == to {
		to = g.wallets[g.rng.Intn(len(g.wallets))]
	}
	amount := uint64(math.Max(1, g.rng.NormFloat64()*1_000_000+5_000_000))
	return g.assembleTransfer(from, to, amount, solana.SystemProgram, false)
}

func (g *Generator) suspiciousTx() models.Transaction {
	ring := g.washTradeRings[g.rng.Intn(len(g.washTradeRings))]
	if len(ring) < 2 {
		return g.normalTx()
	}
	a := g.wallets[ring[g.rng.Intn(len(ring))]]
	b := g.wallets[ring[g.rng.Intn(len(ring))]]
	for a == b {
		b = g.wallets[ring[g.rng.Intn(len(ring))]]
	}
	amount := uint64(g.rng.Intn(2_000_000) + 100_000)
	return g.assembleTransfer(a, b, amount, solana.TokenProgram, false)
}

func (g *Generator) scamTx() models.Transaction {
	if g.rng.Float64() < 0.5 {
		pool := g.pumpDumpPools[g.rng.Intn(len(g.pumpDumpPools))]
		if g.rng.Float64() < 0.7 {
			user := g.wallets[pool.buyers[g.rng.Intn(len(pool.buyers))]]
			return g.assembleSwap(user, pool.pool, solana.RaydiumAMMv4, true)
		}
		user := g.wallets[pool.dumpers[g.rng.Intn(len(pool.dumpers))]]
		return g.assembleSwap(user, pool.pool, solana.RaydiumAMMv4, false)
	}
	from := g.scamWallets[g.rng.Intn(len(g.scamWallets))]
	to := g.wallets[g.rng.Intn(len(g.wallets))]
	amount := uint64(g.rng.Intn(10_000_000) + 1)
	return g.assembleTransfer(from, to, amount, solana.TokenProgram, true)
}

func (g *Generator) assembleTransfer(from, to string, amount uint64, program string, scam bool) models.Transaction {
	sig := solana.Base58Encode(randBytes(g.rng, 64))
	data := transferInstructionData(amount, program)
	ix := models.Instruction{
		ProgramID: program,
		Accounts:  []string{from, to},
		Data:      base64.StdEncoding.EncodeToString(data),
		Kind:      string(solana.KindTransfer),
	}
	tx := models.Transaction{
		Signature:    sig,
		Slot:         uint64(time.Now().Unix()),
		BlockTime:    time.Now().UTC(),
		Fee:          5000,
		Success:      true,
		Accounts:     []string{from, to},
		Instructions: []models.Instruction{ix},
		Sender:       from,
		Receiver:     to,
		Amount:       amount,
		ProgramID:    program,
		RawData:      ix.Data,
	}
	if scam {
		tx.SwapKind = "wash"
	}
	return tx
}

func (g *Generator) assembleSwap(user, pool, dex string, buy bool) models.Transaction {
	sig := solana.Base58Encode(randBytes(g.rng, 64))
	amountIn := uint64(g.rng.Intn(50_000_000) + 1_000_000)
	minOut := amountIn * uint64(g.rng.Intn(100)+50) / 100
	data := swapInstructionData(amountIn, minOut, buy)
	accounts := makeSwapAccounts(g.rng, user, pool)
	ix := models.Instruction{
		ProgramID: dex,
		Accounts:  accounts,
		Data:      base64.StdEncoding.EncodeToString(data),
		Kind:      string(solana.KindSwap),
	}
	swapKind := "buy"
	if !buy {
		swapKind = "sell"
	}
	return models.Transaction{
		Signature:    sig,
		Slot:         uint64(time.Now().Unix()),
		BlockTime:    time.Now().UTC(),
		Fee:          5000,
		Success:      true,
		Accounts:     accounts,
		Instructions: []models.Instruction{ix},
		Sender:       user,
		Receiver:     pool,
		Amount:       amountIn,
		ProgramID:    dex,
		SwapKind:     swapKind,
		RawData:      ix.Data,
	}
}

func (g *Generator) encode(tx models.Transaction) ([]byte, error) {
	env, err := models.NewEnvelope(models.EventRawTransaction, "mockgen", tx)
	if err != nil {
		return nil, fmt.Errorf("envelope: %w", err)
	}
	return env.Encode()
}

func transferInstructionData(amount uint64, program string) []byte {
	switch program {
	case solana.SystemProgram:
		buf := make([]byte, 12)
		binary.LittleEndian.PutUint32(buf[:4], 2)
		binary.LittleEndian.PutUint64(buf[4:], amount)
		return buf
	default:
		buf := make([]byte, 9)
		buf[0] = 3
		binary.LittleEndian.PutUint64(buf[1:], amount)
		return buf
	}
}

func swapInstructionData(amountIn, minOut uint64, buy bool) []byte {
	buf := make([]byte, 17)
	if buy {
		buf[0] = 9
	} else {
		buf[0] = 11
	}
	binary.LittleEndian.PutUint64(buf[1:9], amountIn)
	binary.LittleEndian.PutUint64(buf[9:17], minOut)
	return buf
}

func makeSwapAccounts(rng *mrand.Rand, user, pool string) []string {
	out := make([]string, 17)
	for i := range out {
		out[i] = solana.Base58Encode(randBytes(rng, 32))
	}
	out[1] = pool
	out[16] = user
	return out
}

func randBytes(rng *mrand.Rand, n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err == nil {
		return b
	}
	for i := range b {
		b[i] = byte(rng.Intn(256))
	}
	return b
}
