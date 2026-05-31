package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/andrey/mpc-cluster/internal/grpc"
	"github.com/andrey/mpc-cluster/internal/k8s"
	"github.com/andrey/mpc-cluster/internal/protocol"
	"github.com/andrey/mpc-cluster/internal/redis"
)

const defaultBitLen = 64

type CreateRequest struct {
	ID         string
	Type       OpType
	PartyCount int
	Threshold  int
}

type ManagerConfig struct {
	NodeImage       string
	Namespace       string
	CoordinatorGRPC string
	K8sEnabled      bool
	ResultTimeout   time.Duration
}

type resultCollector struct {
	expected int
	mu       sync.Mutex
	shares   map[int][]byte
	failed   string
	done     chan struct{}
	closed   bool
}

func newCollector(n int) *resultCollector {
	return &resultCollector{expected: n, shares: make(map[int][]byte), done: make(chan struct{})}
}

func (c *resultCollector) add(nodeID int, value []byte, errStr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	if errStr != "" {
		c.failed = errStr
		c.closed = true
		close(c.done)
		return
	}
	c.shares[nodeID] = value
	if len(c.shares) >= c.expected {
		c.closed = true
		close(c.done)
	}
}

type SessionManager struct {
	store *Store
	redis *redis.Client
	k8s   *k8s.Client
	field *protocol.Field
	cfg   ManagerConfig
	log   *slog.Logger

	mu        sync.Mutex
	tasks     map[string]map[int]*grpc.Task
	peers     map[string]map[int]string
	collector map[string]*resultCollector
}

func NewManager(store *Store, rc *redis.Client, kc *k8s.Client, cfg ManagerConfig, log *slog.Logger) *SessionManager {
	if cfg.ResultTimeout == 0 {
		cfg.ResultTimeout = 2 * time.Minute
	}
	return &SessionManager{
		store:     store,
		redis:     rc,
		k8s:       kc,
		field:     protocol.NewField(),
		cfg:       cfg,
		log:       log,
		tasks:     make(map[string]map[int]*grpc.Task),
		peers:     make(map[string]map[int]string),
		collector: make(map[string]*resultCollector),
	}
}

func (m *SessionManager) Create(ctx context.Context, req CreateRequest) (*Session, error) {
	if !req.Type.Valid() {
		return nil, fmt.Errorf("unsupported operation %q", req.Type)
	}
	if req.PartyCount < 2 {
		return nil, fmt.Errorf("party count must be >= 2, got %d", req.PartyCount)
	}
	if req.Type == OpComparison && req.PartyCount != 2 {
		return nil, fmt.Errorf("comparison requires exactly 2 parties")
	}
	threshold := req.Threshold
	if threshold == 0 {
		threshold = req.PartyCount
	}

	id := req.ID
	if id == "" {
		generated, err := newID()
		if err != nil {
			return nil, err
		}
		id = generated
	}
	sess := &Session{
		ID:         id,
		Type:       req.Type,
		Status:     StatusCreated,
		PartyCount: req.PartyCount,
		Threshold:  threshold,
	}
	if err := m.store.Create(ctx, sess); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.peers[id] = make(map[int]string)
	m.mu.Unlock()

	if m.cfg.K8sEnabled && m.k8s != nil {
		for nodeID := 0; nodeID < req.PartyCount; nodeID++ {
			spec := k8s.NodeJobSpec{
				SessionID:       id,
				NodeID:          nodeID,
				Image:           m.cfg.NodeImage,
				Namespace:       m.cfg.Namespace,
				CoordinatorAddr: m.cfg.CoordinatorGRPC,
			}
			if err := m.k8s.CreateNodeJob(ctx, spec); err != nil {
				m.store.Fail(ctx, id, fmt.Sprintf("spawn node %d: %v", nodeID, err))
				return nil, fmt.Errorf("spawn node %d: %w", nodeID, err)
			}
		}
		m.log.Info("spawned node jobs", "session", id, "count", req.PartyCount)
	}

	return sess, nil
}

func (m *SessionManager) RegisterNode(ctx context.Context, sessionID string, nodeID int, addr string) error {
	sess, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	count, err := m.store.RegisterNode(ctx, &NodeRecord{SessionID: sessionID, NodeID: nodeID, Addr: addr})
	if err != nil {
		return err
	}

	m.mu.Lock()
	if m.peers[sessionID] == nil {
		m.peers[sessionID] = make(map[int]string)
	}
	m.peers[sessionID][nodeID] = addr
	m.mu.Unlock()

	m.log.Info("node registered", "session", sessionID, "node", nodeID, "addr", addr, "have", count, "want", sess.PartyCount)

	if count == sess.PartyCount {
		if err := m.store.UpdateStatus(ctx, sessionID, StatusCreated, StatusNodesReady); err != nil {
			return err
		}
		if m.redis != nil {
			if err := m.redis.Publish(ctx, redis.ReadyChannel(sessionID), "ready"); err != nil {
				m.log.Warn("publish ready failed", "session", sessionID, "err", err)
			}
		}
	}
	return nil
}

func (m *SessionManager) Execute(ctx context.Context, sessionID string, inputs map[int]string) (string, error) {
	sess, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if sess.Status != StatusNodesReady {
		return "", fmt.Errorf("session %s in status %s, expected nodes_ready", sessionID, sess.Status)
	}

	slots, err := m.shareInputs(sess, inputs)
	if err != nil {
		return "", err
	}
	tripleShares, err := m.buildTriples(sess)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	peerMap := make(map[int]string, len(m.peers[sessionID]))
	for k, v := range m.peers[sessionID] {
		peerMap[k] = v
	}
	m.mu.Unlock()

	peerEntries := make([]*grpc.PeerEntry, 0, len(peerMap))
	for id, addr := range peerMap {
		peerEntries = append(peerEntries, &grpc.PeerEntry{NodeID: int32(id), Addr: addr})
	}

	tasks := make(map[int]*grpc.Task, sess.PartyCount)
	for nodeID := 0; nodeID < sess.PartyCount; nodeID++ {
		t := &grpc.Task{
			SessionID:  sessionID,
			NodeID:     int32(nodeID),
			Op:         string(sess.Type),
			PartyCount: int32(sess.PartyCount),
			BitLen:     defaultBitLen,
			Ready:      true,
			Peers:      peerEntries,
		}
		for slotIdx, shares := range slots {
			t.Shares = append(t.Shares, &grpc.ShareData{Slot: int32(slotIdx), Value: shares[nodeID].Bytes()})
		}
		for _, ts := range tripleShares {
			st := ts[nodeID]
			t.Triples = append(t.Triples, &grpc.TripleData{A: st.A.Bytes(), B: st.B.Bytes(), C: st.C.Bytes()})
		}
		tasks[nodeID] = t
	}

	coll := newCollector(sess.PartyCount)
	m.mu.Lock()
	m.tasks[sessionID] = tasks
	m.collector[sessionID] = coll
	m.mu.Unlock()

	if err := m.store.UpdateStatus(ctx, sessionID, StatusNodesReady, StatusComputing); err != nil {
		return "", err
	}
	m.store.CreateRound(ctx, &Round{SessionID: sessionID, Number: 1, Phase: "compute", Status: "running"})

	waitCtx, cancel := context.WithTimeout(ctx, m.cfg.ResultTimeout)
	defer cancel()

	select {
	case <-waitCtx.Done():
		m.store.Fail(ctx, sessionID, "timed out waiting for node results")
		return "", fmt.Errorf("session %s: %w", sessionID, waitCtx.Err())
	case <-coll.done:
	}

	if coll.failed != "" {
		m.store.Fail(ctx, sessionID, coll.failed)
		return "", fmt.Errorf("node reported failure: %s", coll.failed)
	}

	result, err := m.recombine(sess, coll)
	if err != nil {
		m.store.Fail(ctx, sessionID, err.Error())
		return "", err
	}

	m.store.CompleteRound(ctx, sessionID, 1)
	if err := m.store.SetResult(ctx, sessionID, result); err != nil {
		return "", err
	}
	m.cleanup(sessionID)
	m.log.Info("session completed", "session", sessionID, "result", result)
	return result, nil
}

func (m *SessionManager) FetchTask(sessionID string, nodeID int) *grpc.Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	byNode := m.tasks[sessionID]
	if byNode == nil {
		return &grpc.Task{SessionID: sessionID, NodeID: int32(nodeID), Ready: false}
	}
	t := byNode[nodeID]
	if t == nil {
		return &grpc.Task{SessionID: sessionID, NodeID: int32(nodeID), Ready: false}
	}
	return t
}

func (m *SessionManager) AcceptResult(sessionID string, nodeID int, value []byte, errStr string) error {
	m.mu.Lock()
	coll := m.collector[sessionID]
	m.mu.Unlock()
	if coll == nil {
		return fmt.Errorf("no active computation for session %s", sessionID)
	}
	coll.add(nodeID, value, errStr)
	return nil
}

func (m *SessionManager) Cancel(ctx context.Context, sessionID string) error {
	sess, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := m.store.Fail(ctx, sessionID, "cancelled by operator"); err != nil {
		return err
	}
	if m.cfg.K8sEnabled && m.k8s != nil {
		if err := m.k8s.DeleteSessionJobs(ctx, m.cfg.Namespace, sessionID); err != nil {
			m.log.Warn("delete jobs failed", "session", sessionID, "err", err)
		}
	}
	m.cleanup(sessionID)
	_ = sess
	return nil
}

func (m *SessionManager) shareInputs(sess *Session, inputs map[int]string) ([][]protocol.FieldElement, error) {
	count := len(inputs)
	if sess.Type == OpComparison && count != 2 {
		return nil, fmt.Errorf("comparison needs 2 inputs, got %d", count)
	}
	if count == 0 {
		return nil, fmt.Errorf("no inputs provided")
	}
	slots := make([][]protocol.FieldElement, count)
	for i := 0; i < count; i++ {
		raw, ok := inputs[i]
		if !ok {
			return nil, fmt.Errorf("missing input for slot %d", i)
		}
		secret, err := m.field.FromString(raw)
		if err != nil {
			return nil, fmt.Errorf("slot %d: %w", i, err)
		}
		shares, err := m.field.AdditiveShare(secret, sess.PartyCount)
		if err != nil {
			return nil, fmt.Errorf("share slot %d: %w", i, err)
		}
		slots[i] = shares
	}
	return slots, nil
}

func (m *SessionManager) buildTriples(sess *Session) ([][]protocol.SharedTriple, error) {
	need := tripleCount(sess.Type, sess.PartyCount)
	out := make([][]protocol.SharedTriple, need)
	for i := 0; i < need; i++ {
		t, err := protocol.GenerateTriple(m.field)
		if err != nil {
			return nil, fmt.Errorf("triple %d: %w", i, err)
		}
		shared, err := protocol.ShareTriple(m.field, t, sess.PartyCount)
		if err != nil {
			return nil, fmt.Errorf("share triple %d: %w", i, err)
		}
		out[i] = shared
	}
	return out, nil
}

func (m *SessionManager) recombine(sess *Session, coll *resultCollector) (string, error) {
	coll.mu.Lock()
	defer coll.mu.Unlock()
	if len(coll.shares) < sess.PartyCount {
		return "", fmt.Errorf("only %d of %d shares received", len(coll.shares), sess.PartyCount)
	}
	parts := make([]protocol.FieldElement, 0, sess.PartyCount)
	for i := 0; i < sess.PartyCount; i++ {
		b, ok := coll.shares[i]
		if !ok {
			return "", fmt.Errorf("missing result share from node %d", i)
		}
		parts = append(parts, m.field.FromBytes(b))
	}
	combined := m.field.AdditiveRecombine(parts)
	val := combined.Big()

	switch sess.Type {
	case OpAverage:
		q := new(big.Int).Div(val, big.NewInt(int64(sess.PartyCount)))
		return q.String(), nil
	default:
		return val.String(), nil
	}
}

func (m *SessionManager) cleanup(sessionID string) {
	m.mu.Lock()
	delete(m.tasks, sessionID)
	delete(m.collector, sessionID)
	m.mu.Unlock()
}

func tripleCount(op OpType, parties int) int {
	switch op {
	case OpSum, OpAverage:
		return 0
	case OpComparison:
		return 2
	case OpMax:
		if parties < 2 {
			return 1
		}
		return parties
	default:
		return 0
	}
}

func newID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return "sess_" + hex.EncodeToString(buf), nil
}
