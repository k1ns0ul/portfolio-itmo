package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
)

type ProducerMetrics struct {
	Sent   atomic.Uint64
	Failed atomic.Uint64
}

type Producer struct {
	p       sarama.AsyncProducer
	metrics ProducerMetrics
}

func NewProducer(brokers []string) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Compression = sarama.CompressionSnappy
	cfg.Producer.Flush.Frequency = 100 * time.Millisecond
	cfg.Producer.Flush.Messages = 256
	cfg.Producer.Partitioner = sarama.NewHashPartitioner
	cfg.Producer.Return.Errors = true
	cfg.Producer.Return.Successes = true

	ap, err := sarama.NewAsyncProducer(brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	p := &Producer{p: ap}
	go func() {
		for range ap.Successes() {
			p.metrics.Sent.Add(1)
		}
	}()
	go func() {
		for e := range ap.Errors() {
			p.metrics.Failed.Add(1)
			slog.Error("kafka send", "err", e.Err, "topic", e.Msg.Topic)
		}
	}()
	return p, nil
}

func (p *Producer) Send(topic string, key, value []byte) {
	msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(value)}
	if key != nil {
		msg.Key = sarama.ByteEncoder(key)
	}
	p.p.Input() <- msg
}

func (p *Producer) Metrics() (sent, failed uint64) {
	return p.metrics.Sent.Load(), p.metrics.Failed.Load()
}

func (p *Producer) Close(ctx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- p.p.Close() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
