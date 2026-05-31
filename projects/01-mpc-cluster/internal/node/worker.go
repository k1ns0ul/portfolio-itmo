package node

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/andrey/mpc-cluster/internal/config"
	mgrpc "github.com/andrey/mpc-cluster/internal/grpc"
	"github.com/andrey/mpc-cluster/internal/protocol"
)

type NodeWorker struct {
	cfg       *config.NodeConfig
	advertise string
	coord     *mgrpc.CoordinatorClient
	buf       *Buffer
	field     *protocol.Field
	log       *slog.Logger
}

func NewWorker(cfg *config.NodeConfig, advertise string, coord *mgrpc.CoordinatorClient, buf *Buffer, log *slog.Logger) *NodeWorker {
	return &NodeWorker{
		cfg:       cfg,
		advertise: advertise,
		coord:     coord,
		buf:       buf,
		field:     protocol.NewField(),
		log:       log,
	}
}

func (w *NodeWorker) Run(ctx context.Context) error {
	if err := w.register(ctx); err != nil {
		return err
	}
	task, err := w.awaitTask(ctx)
	if err != nil {
		return err
	}

	w.log.Info("task received", "op", task.Op, "parties", task.PartyCount, "shares", len(task.Shares), "triples", len(task.Triples))

	peers := NewPeerNetwork(w.cfg.NodeID, task.SessionID, w.buf, w.cfg.GatherTimeout)
	peerMap := map[int]string{}
	for _, pe := range task.Peers {
		peerMap[int(pe.NodeID)] = pe.Addr
	}
	if err := peers.Connect(peerMap); err != nil {
		w.report(ctx, task.SessionID, err)
		return err
	}
	defer peers.Close()

	result, err := w.compute(ctx, task, peers)
	if err != nil {
		w.report(ctx, task.SessionID, err)
		return err
	}

	_, err = w.coord.SubmitResult(ctx, &mgrpc.ResultMsg{
		SessionID: task.SessionID,
		NodeID:    int32(w.cfg.NodeID),
		Value:     result.Bytes(),
	})
	if err != nil {
		return fmt.Errorf("submit result: %w", err)
	}
	w.log.Info("result submitted", "session", task.SessionID, "node", w.cfg.NodeID)
	return nil
}

func (w *NodeWorker) register(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		ack, err := w.coord.RegisterNode(ctx, &mgrpc.NodeInfo{
			SessionID: w.cfg.SessionID,
			NodeID:    int32(w.cfg.NodeID),
			Addr:      w.advertise,
		})
		if err == nil && ack.Ok {
			w.log.Info("registered", "session", w.cfg.SessionID, "node", w.cfg.NodeID)
			return nil
		}
		if err != nil {
			w.log.Warn("register attempt failed", "err", err)
		} else {
			w.log.Warn("register rejected", "reason", ack.Message)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("registration aborted: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (w *NodeWorker) awaitTask(ctx context.Context) (*mgrpc.Task, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		task, err := w.coord.GetTask(ctx, &mgrpc.TaskRequest{
			SessionID: w.cfg.SessionID,
			NodeID:    int32(w.cfg.NodeID),
		})
		if err != nil {
			w.log.Warn("get task failed", "err", err)
		} else if task.Ready {
			return task, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for task: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (w *NodeWorker) compute(ctx context.Context, task *mgrpc.Task, peers protocol.PeerNetwork) (protocol.FieldElement, error) {
	slots := w.parseSlots(task)
	if len(slots) == 0 {
		return protocol.FieldElement{}, fmt.Errorf("task carries no input shares")
	}
	triples := w.parseTriples(task)
	bitLen := int(task.BitLen)
	if bitLen <= 0 {
		bitLen = 32
	}

	switch task.Op {
	case "sum", "average":
		acc := slots[0]
		for _, s := range slots[1:] {
			acc = w.field.Add(acc, s)
		}
		return acc, nil

	case "comparison":
		if len(slots) < 2 {
			return protocol.FieldElement{}, fmt.Errorf("comparison needs 2 shares")
		}
		return protocol.SecureCompare(ctx, w.field, slots[0], slots[1], w.cfg.NodeID, triples, peers, bitLen)

	case "max":
		running := slots[0]
		cursor := 0
		for i := 1; i < len(slots); i++ {
			cmp, err := protocol.SecureCompare(ctx, w.field, running, slots[i], w.cfg.NodeID, triples, peers, bitLen)
			if err != nil {
				return protocol.FieldElement{}, fmt.Errorf("compare step %d: %w", i, err)
			}
			if cursor >= len(triples) {
				return protocol.FieldElement{}, fmt.Errorf("out of beaver triples at step %d", i)
			}
			diff := w.field.Sub(running, slots[i])
			prod, err := protocol.SecureMul(ctx, w.field, cmp, diff, w.cfg.NodeID, triples[cursor], peers)
			if err != nil {
				return protocol.FieldElement{}, fmt.Errorf("select step %d: %w", i, err)
			}
			cursor++
			running = w.field.Add(slots[i], prod)
		}
		return running, nil

	default:
		return protocol.FieldElement{}, fmt.Errorf("unknown operation %q", task.Op)
	}
}

func (w *NodeWorker) parseSlots(task *mgrpc.Task) []protocol.FieldElement {
	ordered := append([]*mgrpc.ShareData(nil), task.Shares...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Slot < ordered[j].Slot })
	out := make([]protocol.FieldElement, 0, len(ordered))
	for _, s := range ordered {
		out = append(out, w.field.FromBytes(s.Value))
	}
	return out
}

func (w *NodeWorker) parseTriples(task *mgrpc.Task) []protocol.SharedTriple {
	out := make([]protocol.SharedTriple, 0, len(task.Triples))
	for _, t := range task.Triples {
		out = append(out, protocol.SharedTriple{
			A: w.field.FromBytes(t.A),
			B: w.field.FromBytes(t.B),
			C: w.field.FromBytes(t.C),
		})
	}
	return out
}

func (w *NodeWorker) report(ctx context.Context, sessionID string, cause error) {
	_, err := w.coord.SubmitResult(ctx, &mgrpc.ResultMsg{
		SessionID: sessionID,
		NodeID:    int32(w.cfg.NodeID),
		Err:       cause.Error(),
	})
	if err != nil {
		w.log.Error("failed to report error", "err", err)
	}
}
