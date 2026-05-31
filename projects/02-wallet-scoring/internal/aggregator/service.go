package aggregator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/andrey/wallet-scoring/internal/clickhouse"
	"github.com/andrey/wallet-scoring/internal/config"
	agg "github.com/andrey/wallet-scoring/internal/grpcint"
	"github.com/andrey/wallet-scoring/internal/kafka"
	"github.com/andrey/wallet-scoring/internal/models"
)

type Service struct {
	cfg      config.AggregatorConfig
	txRepo   *clickhouse.TxRepo
	wRepo    *clickhouse.WalletRepo
	producer *kafka.Producer
	scoreTop string

	mu          sync.Mutex
	lastTickEnd time.Time
}

func New(cfg config.AggregatorConfig, txRepo *clickhouse.TxRepo, wRepo *clickhouse.WalletRepo, producer *kafka.Producer, scoreTopic string) *Service {
	return &Service{
		cfg:         cfg,
		txRepo:      txRepo,
		wRepo:       wRepo,
		producer:    producer,
		scoreTop:    scoreTopic,
		lastTickEnd: time.Now().UTC().Add(-cfg.LookbackDur),
	}
}

func (s *Service) Run(ctx context.Context) error {
	t := time.NewTicker(s.cfg.Interval)
	defer t.Stop()
	slog.Info("aggregator running", "interval", s.cfg.Interval.String(), "lookback", s.cfg.LookbackDur.String())
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := s.tick(ctx); err != nil {
				slog.Error("aggregator tick", "err", err)
			}
		}
	}
}

func (s *Service) tick(ctx context.Context) error {
	s.mu.Lock()
	from := s.lastTickEnd
	to := time.Now().UTC()
	s.mu.Unlock()

	stats, prevByWallet, err := s.computeRange(ctx, from, to)
	if err != nil {
		return err
	}
	if len(stats) == 0 {
		s.mu.Lock()
		s.lastTickEnd = to
		s.mu.Unlock()
		return nil
	}

	if err := s.wRepo.UpsertStats(ctx, stats); err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	s.emitScoreUpdates(stats, prevByWallet)
	s.emitHistory(ctx, stats)

	s.mu.Lock()
	s.lastTickEnd = to
	s.mu.Unlock()
	slog.Info("tick", "wallets", len(stats), "from", from, "to", to)
	return nil
}

func (s *Service) computeRange(ctx context.Context, from, to time.Time) ([]models.WalletStats, map[string]models.WalletStats, error) {
	txs, err := s.fetchTransactions(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}
	if len(txs) == 0 {
		return nil, nil, nil
	}

	grouped := groupBySender(txs)
	wallets := make([]string, 0, len(grouped))
	for w := range grouped {
		wallets = append(wallets, w)
	}
	prev := s.fetchPrev(ctx, wallets)

	now := time.Now().UTC()
	out := make([]models.WalletStats, 0, len(grouped))
	for wallet, list := range grouped {
		first, last := TimeRange(list)
		if p, ok := prev[wallet]; ok {
			if !p.FirstSeen.IsZero() && p.FirstSeen.Before(first) {
				first = p.FirstSeen
			}
		}
		s := models.WalletStats{
			Wallet:               wallet,
			TxCount:              TxCount(list),
			FirstSeen:            first,
			LastSeen:             last,
			UniqueCounterparties: UniqueCounterparties(wallet, list),
			AvgTxAmount:          AvgAmount(list),
			MedianTxAmount:       MedianAmount(list),
			HerfindahlIndex:      Herfindahl(wallet, list),
			SmartContractRatio:   SmartContractRatio(list),
			VelocityPerHour:      VelocityPerHour(list, now),
			DormancyDays:         DormancyDays(last, now),
			UpdatedAt:            now,
		}
		s.Score, s.Category = ScoreFromFeatures(s)
		out = append(out, s)
	}
	return out, prev, nil
}

func (s *Service) fetchTransactions(ctx context.Context, from, to time.Time) ([]models.Transaction, error) {
	const pageSize = 5000
	const max = 100000
	var all []models.Transaction
	cursor := ""
	for {
		page, next, err := s.txRepo.GetByTimeRange(ctx, from, to, pageSize, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetch txs: %w", err)
		}
		all = append(all, page...)
		if next == "" || len(all) >= max {
			break
		}
		cursor = next
	}
	return all, nil
}

func (s *Service) fetchPrev(ctx context.Context, wallets []string) map[string]models.WalletStats {
	out := make(map[string]models.WalletStats, len(wallets))
	for _, w := range wallets {
		st, err := s.wRepo.GetStats(ctx, w)
		if err != nil {
			continue
		}
		out[w] = *st
	}
	return out
}

func (s *Service) emitScoreUpdates(stats []models.WalletStats, prev map[string]models.WalletStats) {
	if s.producer == nil || s.scoreTop == "" {
		return
	}
	for _, st := range stats {
		ws := models.WalletScore{
			Wallet:    st.Wallet,
			Score:     st.Score,
			Category:  st.Category,
			UpdatedAt: st.UpdatedAt,
		}
		if p, ok := prev[st.Wallet]; ok {
			ws.Previous = p.Score
			if p.Category != st.Category {
				ws.Reason = fmt.Sprintf("category change: %s -> %s", p.Category, st.Category)
			}
		}
		env, err := models.NewEnvelope(models.EventScoreUpdated, "aggregator", ws)
		if err != nil {
			continue
		}
		payload, err := kafkaEncode(env)
		if err != nil {
			continue
		}
		s.producer.Send(s.scoreTop, []byte(st.Wallet), payload)
	}
}

func (s *Service) emitHistory(ctx context.Context, stats []models.WalletStats) {
	pts := make([]models.WalletScore, 0, len(stats))
	for _, st := range stats {
		pts = append(pts, models.WalletScore{
			Wallet:    st.Wallet,
			Score:     st.Score,
			Category:  st.Category,
			UpdatedAt: st.UpdatedAt,
		})
	}
	if err := s.wRepo.RecordHistory(ctx, pts); err != nil {
		slog.Error("record history", "err", err)
	}
}

func (s *Service) WalletStats(ctx context.Context, addr string) (*agg.WalletStatsResponse, error) {
	st, err := s.wRepo.GetStats(ctx, addr)
	if err != nil {
		if errors.Is(err, clickhouse.ErrNotFound) {
			return &agg.WalletStatsResponse{Wallet: addr, Found: false}, nil
		}
		return nil, err
	}
	return toProto(st), nil
}

func (s *Service) TopWallets(ctx context.Context, limit, offset uint32) (*agg.TopWalletsResponse, error) {
	list, err := s.wRepo.GetTopByScore(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	out := &agg.TopWalletsResponse{Items: make([]*agg.WalletStatsResponse, 0, len(list))}
	for i := range list {
		out.Items = append(out.Items, toProto(&list[i]))
	}
	return out, nil
}

func (s *Service) RefreshWallet(ctx context.Context, addr string) error {
	to := time.Now().UTC()
	from := to.Add(-s.cfg.LookbackDur)
	stats, prev, err := s.computeForWallet(ctx, addr, from, to)
	if err != nil {
		return err
	}
	if stats == nil {
		return nil
	}
	if err := s.wRepo.UpsertStats(ctx, []models.WalletStats{*stats}); err != nil {
		return err
	}
	s.emitScoreUpdates([]models.WalletStats{*stats}, prev)
	return nil
}

func (s *Service) computeForWallet(ctx context.Context, addr string, from, to time.Time) (*models.WalletStats, map[string]models.WalletStats, error) {
	var all []models.Transaction
	cursor := ""
	for {
		page, next, err := s.txRepo.GetByWallet(ctx, addr, 1000, cursor)
		if err != nil {
			return nil, nil, err
		}
		filtered := page[:0]
		for _, t := range page {
			if t.BlockTime.Before(from) || t.BlockTime.After(to) {
				continue
			}
			filtered = append(filtered, t)
		}
		all = append(all, filtered...)
		if next == "" || len(all) > 50000 {
			break
		}
		cursor = next
	}
	if len(all) == 0 {
		return nil, nil, nil
	}
	now := time.Now().UTC()
	first, last := TimeRange(all)
	prev := s.fetchPrev(ctx, []string{addr})
	if p, ok := prev[addr]; ok && !p.FirstSeen.IsZero() && p.FirstSeen.Before(first) {
		first = p.FirstSeen
	}
	st := models.WalletStats{
		Wallet:               addr,
		TxCount:              TxCount(all),
		FirstSeen:            first,
		LastSeen:             last,
		UniqueCounterparties: UniqueCounterparties(addr, all),
		AvgTxAmount:          AvgAmount(all),
		MedianTxAmount:       MedianAmount(all),
		HerfindahlIndex:      Herfindahl(addr, all),
		SmartContractRatio:   SmartContractRatio(all),
		VelocityPerHour:      VelocityPerHour(all, now),
		DormancyDays:         DormancyDays(last, now),
		UpdatedAt:            now,
	}
	st.Score, st.Category = ScoreFromFeatures(st)
	return &st, prev, nil
}

func groupBySender(txs []models.Transaction) map[string][]models.Transaction {
	out := make(map[string][]models.Transaction, len(txs)/4)
	for _, t := range txs {
		if t.Sender == "" {
			continue
		}
		out[t.Sender] = append(out[t.Sender], t)
	}
	return out
}

func toProto(s *models.WalletStats) *agg.WalletStatsResponse {
	return &agg.WalletStatsResponse{
		Wallet:               s.Wallet,
		TxCount:              s.TxCount,
		FirstSeenUnix:        s.FirstSeen.Unix(),
		LastSeenUnix:         s.LastSeen.Unix(),
		UniqueCounterparties: s.UniqueCounterparties,
		AvgTxAmount:          s.AvgTxAmount,
		MedianTxAmount:       s.MedianTxAmount,
		HerfindahlIndex:      s.HerfindahlIndex,
		SmartContractRatio:   s.SmartContractRatio,
		VelocityPerHour:      s.VelocityPerHour,
		DormancyDays:         s.DormancyDays,
		Score:                s.Score,
		Category:             string(s.Category),
		UpdatedAtUnix:        s.UpdatedAt.Unix(),
		Found:                true,
	}
}

func kafkaEncode(env models.Envelope) ([]byte, error) {
	return env.Encode()
}
