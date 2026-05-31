package solana

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"github.com/andrey/wallet-scoring/internal/common"
	"github.com/andrey/wallet-scoring/internal/models"
	pb "github.com/andrey/wallet-scoring/internal/proto"
)

type Filter struct {
	AccountIncludes []string
	AccountExcludes []string
	Commitment      pb.CommitmentLevel
}

type GeyserMetrics struct {
	Received   atomic.Uint64
	Reconnects atomic.Uint64
	Errors     atomic.Uint64
}

type GeyserClient struct {
	endpoint string
	token    string
	conn     *grpc.ClientConn
	metrics  GeyserMetrics
}

func NewGeyser(ctx context.Context, endpoint, token string) (*GeyserClient, error) {
	if endpoint == "" {
		return nil, errors.New("geyser: empty endpoint")
	}
	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.ForceCodec(common.Codec{}),
			grpc.MaxCallRecvMsgSize(64*1024*1024),
		),
	}
	if isPlaintext(endpoint) {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, hostPort(endpoint), opts...)
	if err != nil {
		return nil, fmt.Errorf("geyser dial: %w", err)
	}
	return &GeyserClient{endpoint: endpoint, token: token, conn: conn}, nil
}

func (c *GeyserClient) Close() error { return c.conn.Close() }

func (c *GeyserClient) Metrics() (received, reconnects, errs uint64) {
	return c.metrics.Received.Load(), c.metrics.Reconnects.Load(), c.metrics.Errors.Load()
}

func (c *GeyserClient) Subscribe(ctx context.Context, filter Filter) <-chan *models.Transaction {
	out := make(chan *models.Transaction, 10000)
	go c.runSubscribe(ctx, filter, out)
	return out
}

var subscribeDesc = &grpc.StreamDesc{
	StreamName:    "Subscribe",
	ServerStreams: true,
	ClientStreams: true,
}

func (c *GeyserClient) runSubscribe(ctx context.Context, filter Filter, out chan<- *models.Transaction) {
	defer close(out)
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.runStream(ctx, filter, out)
		if ctx.Err() != nil {
			return
		}
		c.metrics.Reconnects.Add(1)
		c.metrics.Errors.Add(1)
		slog.Warn("geyser stream broken", "err", err, "backoff", backoff.String())
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
	}
}

func (c *GeyserClient) runStream(ctx context.Context, filter Filter, out chan<- *models.Transaction) error {
	streamCtx := ctx
	if c.token != "" {
		streamCtx = metadata.AppendToOutgoingContext(ctx, "x-token", c.token)
	}
	stream, err := c.conn.NewStream(streamCtx, subscribeDesc, "/geyser.Geyser/Subscribe")
	if err != nil {
		return fmt.Errorf("new stream: %w", err)
	}

	req := &pb.SubscribeRequest{
		Transactions: map[string]*pb.SubscribeRequestFilterTransactions{
			"main": {
				AccountInclude: filter.AccountIncludes,
				AccountExclude: filter.AccountExcludes,
			},
		},
		Commitment: filter.Commitment,
	}
	if err := stream.SendMsg(req); err != nil {
		return fmt.Errorf("send req: %w", err)
	}

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	go func() {
		for {
			select {
			case <-streamCtx.Done():
				return
			case <-heartbeat.C:
				_ = stream.SendMsg(&pb.SubscribeRequest{})
			}
		}
	}()

	for {
		upd := &pb.SubscribeUpdate{}
		if err := stream.RecvMsg(upd); err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		if upd.Ping != nil {
			continue
		}
		if upd.Transaction == nil {
			continue
		}
		c.metrics.Received.Add(1)
		tx, err := transactionFromUpdate(upd.Transaction)
		if err != nil {
			c.metrics.Errors.Add(1)
			slog.Debug("convert update", "err", err)
			continue
		}
		select {
		case out <- tx:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func transactionFromUpdate(upd *pb.SubscribeUpdateTransaction) (*models.Transaction, error) {
	if upd.Transaction == nil || upd.Transaction.Transaction == nil || upd.Transaction.Transaction.Message == nil {
		return nil, errors.New("update has no transaction message")
	}
	info := upd.Transaction
	msg := info.Transaction.Message

	keys := make([]string, len(msg.AccountKeys))
	for i, k := range msg.AccountKeys {
		keys[i] = Base58Encode(k)
	}

	instrs := make([]models.Instruction, 0, len(msg.Instructions))
	for _, ix := range msg.Instructions {
		if int(ix.ProgramIDIndex) >= len(keys) {
			return nil, fmt.Errorf("program index %d out of range", ix.ProgramIDIndex)
		}
		accounts := make([]string, len(ix.Accounts))
		for i, idx := range ix.Accounts {
			if int(idx) >= len(keys) {
				return nil, fmt.Errorf("account index %d out of range", idx)
			}
			accounts[i] = keys[idx]
		}
		progID := keys[ix.ProgramIDIndex]
		instrs = append(instrs, models.Instruction{
			ProgramID: progID,
			Accounts:  accounts,
			Data:      base64.StdEncoding.EncodeToString(ix.Data),
			Kind:      string(ClassifyInstruction(progID, ix.Data)),
		})
	}

	tx := &models.Transaction{
		Signature:    Base58Encode(info.Signature),
		Slot:         upd.Slot,
		BlockTime:    time.Now().UTC(),
		Accounts:     keys,
		Instructions: instrs,
		Success:      info.Meta == nil || len(info.Meta.Err) == 0,
	}
	if info.Meta != nil {
		tx.Fee = info.Meta.Fee
	}
	fillSummary(tx)
	DecodeAllSwaps(tx)
	return tx, nil
}

func isPlaintext(endpoint string) bool {
	return strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "plaintext://")
}

func hostPort(endpoint string) string {
	for _, p := range []string{"https://", "http://", "grpc://", "plaintext://"} {
		if strings.HasPrefix(endpoint, p) {
			return endpoint[len(p):]
		}
	}
	return endpoint
}
