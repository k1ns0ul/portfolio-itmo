package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
)

type Handler func(ctx context.Context, msg *sarama.ConsumerMessage) error

type ConsumerMetrics struct {
	Consumed atomic.Uint64
	Retried  atomic.Uint64
	Dropped  atomic.Uint64
}

type Consumer struct {
	group   sarama.ConsumerGroup
	dlq     *Producer
	dlqName string
	topic   string
	metrics ConsumerMetrics
}

type ConsumerOptions struct {
	Brokers []string
	GroupID string
	Topic   string
	DLQ     *Producer
	DLQName string
}

func NewConsumer(opts ConsumerOptions) (*Consumer, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Consumer.Group.Session.Timeout = 30 * time.Second
	cfg.Consumer.Group.Heartbeat.Interval = 3 * time.Second
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Return.Errors = true

	g, err := sarama.NewConsumerGroup(opts.Brokers, opts.GroupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer group: %w", err)
	}

	c := &Consumer{
		group:   g,
		dlq:     opts.DLQ,
		dlqName: opts.DLQName,
		topic:   opts.Topic,
	}

	go func() {
		for err := range g.Errors() {
			slog.Error("kafka consumer", "err", err)
		}
	}()

	return c, nil
}

func (c *Consumer) Subscribe(ctx context.Context, h Handler) error {
	gh := &groupHandler{h: h, c: c}
	for {
		if err := c.group.Consume(ctx, []string{c.topic}, gh); err != nil {
			if errors.Is(err, sarama.ErrClosedConsumerGroup) {
				return nil
			}
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("consume cycle", "err", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func (c *Consumer) Metrics() (consumed, retried, dropped uint64) {
	return c.metrics.Consumed.Load(), c.metrics.Retried.Load(), c.metrics.Dropped.Load()
}

func (c *Consumer) Close() error { return c.group.Close() }

type groupHandler struct {
	h Handler
	c *Consumer
}

func (g *groupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (g *groupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (g *groupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			g.handleWithRetry(sess.Context(), msg)
			sess.MarkMessage(msg, "")
		case <-sess.Context().Done():
			return nil
		}
	}
}

func (g *groupHandler) handleWithRetry(ctx context.Context, msg *sarama.ConsumerMessage) {
	const attempts = 3
	delay := 200 * time.Millisecond
	var last error
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return
		}
		if err := g.h(ctx, msg); err == nil {
			g.c.metrics.Consumed.Add(1)
			return
		} else {
			last = err
			if i < attempts-1 {
				g.c.metrics.Retried.Add(1)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
				delay *= 2
			}
		}
	}
	g.c.metrics.Dropped.Add(1)
	slog.Error("handler exhausted", "err", last, "topic", msg.Topic, "offset", msg.Offset)
	if g.c.dlq != nil && g.c.dlqName != "" {
		g.c.dlq.Send(g.c.dlqName, msg.Key, msg.Value)
	}
}
