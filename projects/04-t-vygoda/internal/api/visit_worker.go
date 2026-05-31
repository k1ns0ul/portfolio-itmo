package api

import (
	"context"
	"log/slog"
	"sync"
	"time"

	rds "github.com/andrey/t-vygoda/internal/redis"
)

func StartVisitWorker(ctx context.Context, streaks *rds.Streaks, in <-chan int64, batchInterval time.Duration) {
	if batchInterval <= 0 {
		batchInterval = 2 * time.Second
	}
	go func() {
		pending := make(map[int64]struct{}, 256)
		var mu sync.Mutex
		t := time.NewTicker(batchInterval)
		defer t.Stop()

		flush := func() {
			mu.Lock()
			snapshot := pending
			pending = make(map[int64]struct{}, 256)
			mu.Unlock()
			if len(snapshot) == 0 {
				return
			}
			fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for uid := range snapshot {
				if _, err := streaks.RecordVisit(fctx, uid); err != nil {
					slog.Debug("record visit", "user_id", uid, "err", err)
				}
			}
		}

		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case uid, ok := <-in:
				if !ok {
					flush()
					return
				}
				mu.Lock()
				pending[uid] = struct{}{}
				mu.Unlock()
			case <-t.C:
				flush()
			}
		}
	}()
}
