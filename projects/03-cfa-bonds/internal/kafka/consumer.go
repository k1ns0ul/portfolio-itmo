package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/IBM/sarama"
)

type Handler func(ctx context.Context, key, value []byte) error

type Consumer struct {
	group    sarama.ConsumerGroup
	topics   []string
	handler  Handler
	log      *slog.Logger
	dlq      *Producer
	dlqTopic string
	maxRetry int
}

type ConsumerOptions struct {
	Brokers  []string
	GroupID  string
	ClientID string
	Topics   []string
	DLQ      *Producer
	DLQTopic string
	MaxRetry int
}

func NewConsumer(opts ConsumerOptions, handler Handler, log *slog.Logger) (*Consumer, error) {
	cfg := sarama.NewConfig()
	cfg.ClientID = opts.ClientID
	cfg.Version = sarama.V2_8_0_0
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Return.Errors = true

	group, err := sarama.NewConsumerGroup(opts.Brokers, opts.GroupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer group %s: %w", opts.GroupID, err)
	}
	maxRetry := opts.MaxRetry
	if maxRetry <= 0 {
		maxRetry = 3
	}
	return &Consumer{
		group:    group,
		topics:   opts.Topics,
		handler:  handler,
		log:      log,
		dlq:      opts.DLQ,
		dlqTopic: opts.DLQTopic,
		maxRetry: maxRetry,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	go func() {
		for err := range c.group.Errors() {
			c.log.Error("consumer group error", "err", err)
		}
	}()

	gh := &groupHandler{c: c}
	for {
		if err := c.group.Consume(ctx, c.topics, gh); err != nil {
			if errors.Is(err, sarama.ErrClosedConsumerGroup) {
				return nil
			}
			c.log.Error("consume loop error, retrying", "err", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func (c *Consumer) Close() error {
	return c.group.Close()
}

type groupHandler struct {
	c *Consumer
}

func (h *groupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *groupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *groupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.process(sess.Context(), msg)
			sess.MarkMessage(msg, "")
		case <-sess.Context().Done():
			return nil
		}
	}
}

func (h *groupHandler) process(ctx context.Context, msg *sarama.ConsumerMessage) {
	var lastErr error
	delay := 200 * time.Millisecond
	for attempt := 1; attempt <= h.c.maxRetry; attempt++ {
		if lastErr = h.c.handler(ctx, msg.Key, msg.Value); lastErr == nil {
			return
		}
		h.c.log.Warn("handler failed", "topic", msg.Topic, "attempt", attempt, "err", lastErr)
		if attempt < h.c.maxRetry {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			delay *= 2
		}
	}
	h.deadLetter(ctx, msg, lastErr)
}

func (h *groupHandler) deadLetter(ctx context.Context, msg *sarama.ConsumerMessage, cause error) {
	if h.c.dlq == nil || h.c.dlqTopic == "" {
		h.c.log.Error("dropping message, no DLQ configured", "topic", msg.Topic, "err", cause)
		return
	}
	envelope := map[string]any{
		"original_topic": msg.Topic,
		"key":            string(msg.Key),
		"value":          string(msg.Value),
		"error":          cause.Error(),
	}
	if err := h.c.dlq.Publish(ctx, h.c.dlqTopic, string(msg.Key), envelope); err != nil {
		h.c.log.Error("failed to write to DLQ", "err", err)
	}
}
