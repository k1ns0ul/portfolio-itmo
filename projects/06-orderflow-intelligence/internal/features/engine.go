package features

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/andrey/orderflow-intelligence/internal/clickhouse"
	"github.com/andrey/orderflow-intelligence/internal/models"
	rds "github.com/andrey/orderflow-intelligence/internal/redis"
)

type Engine struct {
	aggregators []*Aggregator
	repo        *clickhouse.Repo

	swapMu     sync.Mutex
	swapBuffer []models.SwapEvent
	batchSize  int
}

func NewEngine(intervals []time.Duration, repo *clickhouse.Repo, cache *rds.Cache, batchSize int) *Engine {
	if batchSize <= 0 {
		batchSize = 256
	}
	aggs := make([]*Aggregator, 0, len(intervals))
	for _, d := range intervals {
		aggs = append(aggs, NewAggregator(d, repo, cache))
	}
	return &Engine{
		aggregators: aggs,
		repo:        repo,
		batchSize:   batchSize,
	}
}

func (e *Engine) Push(s models.SwapEvent) {
	for _, agg := range e.aggregators {
		agg.Add(s)
	}
	e.swapMu.Lock()
	e.swapBuffer = append(e.swapBuffer, s)
	flush := len(e.swapBuffer) >= e.batchSize
	var pending []models.SwapEvent
	if flush {
		pending = e.swapBuffer
		e.swapBuffer = make([]models.SwapEvent, 0, e.batchSize)
	}
	e.swapMu.Unlock()
	if flush {
		e.writeSwaps(pending)
	}
}

func (e *Engine) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, agg := range e.aggregators {
		wg.Add(1)
		go func(a *Aggregator) {
			defer wg.Done()
			a.Run(ctx)
		}(agg)
	}

	swapFlush := time.NewTicker(5 * time.Second)
	defer swapFlush.Stop()

	for {
		select {
		case <-ctx.Done():
			e.flushSwaps()
			wg.Wait()
			return
		case <-swapFlush.C:
			e.flushSwaps()
		}
	}
}

func (e *Engine) flushSwaps() {
	e.swapMu.Lock()
	if len(e.swapBuffer) == 0 {
		e.swapMu.Unlock()
		return
	}
	pending := e.swapBuffer
	e.swapBuffer = make([]models.SwapEvent, 0, e.batchSize)
	e.swapMu.Unlock()
	e.writeSwaps(pending)
}

func (e *Engine) writeSwaps(swaps []models.SwapEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.repo.InsertSwaps(ctx, swaps); err != nil {
		slog.Error("insert swaps", "err", err, "count", len(swaps))
		return
	}
	slog.Debug("swaps written", "count", len(swaps))
}
