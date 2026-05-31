package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/IBM/sarama"
)

const (
	TopicTradeSubmitted = "trade.submitted"
	TopicTradeSettled   = "trade.settled"
	TopicTradeFailed    = "trade.failed"
	TopicCouponPaid     = "coupon.paid"
	TopicIssueMatured   = "issue.matured"
)

type Producer struct {
	async sarama.AsyncProducer
	log   *slog.Logger
}

func NewProducer(brokers []string, clientID string, log *slog.Logger) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.ClientID = clientID
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Compression = sarama.CompressionSnappy
	cfg.Producer.Return.Successes = false
	cfg.Producer.Return.Errors = true
	cfg.Version = sarama.V2_8_0_0

	ap, err := sarama.NewAsyncProducer(brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("create async producer: %w", err)
	}
	p := &Producer{async: ap, log: log}
	go p.drainErrors()
	return p, nil
}

func (p *Producer) drainErrors() {
	for e := range p.async.Errors() {
		p.log.Error("kafka publish failed", "topic", e.Msg.Topic, "err", e.Err)
	}
}

func (p *Producer) Publish(ctx context.Context, topic, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal message for %s: %w", topic, err)
	}
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(body),
	}
	select {
	case p.async.Input() <- msg:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("publish to %s cancelled: %w", topic, ctx.Err())
	}
}

func (p *Producer) Close() error {
	p.async.AsyncClose()
	return nil
}
