package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("session not found")
var ErrStatusConflict = errors.New("session status conflict")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Create(ctx context.Context, sess *Session) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, type, status, party_count, threshold)
		VALUES ($1, $2, $3, $4, $5)`,
		sess.ID, string(sess.Type), string(sess.Status), sess.PartyCount, sess.Threshold)
	if err != nil {
		return fmt.Errorf("insert session %s: %w", sess.ID, err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) (*Session, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, type, status, party_count, threshold, result, error_msg, created_at, updated_at
		FROM sessions WHERE id = $1`, id)
	sess, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load session %s: %w", id, err)
	}
	return sess, nil
}

func (s *Store) UpdateStatus(ctx context.Context, id string, expected, next Status) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sessions SET status = $1, updated_at = now()
		WHERE id = $2 AND status = $3`,
		string(next), id, string(expected))
	if err != nil {
		return fmt.Errorf("update status %s -> %s: %w", expected, next, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("session %s not in %s: %w", id, expected, ErrStatusConflict)
	}
	return nil
}

func (s *Store) SetResult(ctx context.Context, id, result string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sessions SET status = $1, result = $2, updated_at = now()
		WHERE id = $3`, string(StatusCompleted), result, id)
	if err != nil {
		return fmt.Errorf("store result for %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Fail(ctx context.Context, id, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions SET status = $1, error_msg = $2, updated_at = now()
		WHERE id = $3`, string(StatusFailed), reason, id)
	if err != nil {
		return fmt.Errorf("mark session %s failed: %w", id, err)
	}
	return nil
}

func (s *Store) ListActive(ctx context.Context) ([]*Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, status, party_count, threshold, result, error_msg, created_at, updated_at
		FROM sessions
		WHERE status NOT IN ($1, $2)
		ORDER BY created_at DESC`, string(StatusCompleted), string(StatusFailed))
	if err != nil {
		return nil, fmt.Errorf("query active sessions: %w", err)
	}
	defer rows.Close()
	return collectSessions(rows)
}

func (s *Store) ListAll(ctx context.Context, limit, offset int) ([]*Session, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM sessions").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, status, party_count, threshold, result, error_msg, created_at, updated_at
		FROM sessions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query sessions page: %w", err)
	}
	defer rows.Close()
	list, err := collectSessions(rows)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *Store) CreateRound(ctx context.Context, r *Round) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO rounds (session_id, number, phase, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (session_id, number) DO UPDATE SET phase = EXCLUDED.phase, status = EXCLUDED.status`,
		r.SessionID, r.Number, r.Phase, r.Status)
	if err != nil {
		return fmt.Errorf("create round %d for %s: %w", r.Number, r.SessionID, err)
	}
	return nil
}

func (s *Store) GetCurrentRound(ctx context.Context, sessionID string) (*Round, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT session_id, number, phase, status, created_at, completed_at
		FROM rounds WHERE session_id = $1
		ORDER BY number DESC LIMIT 1`, sessionID)
	var r Round
	err := row.Scan(&r.SessionID, &r.Number, &r.Phase, &r.Status, &r.CreatedAt, &r.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("current round for %s: %w", sessionID, err)
	}
	return &r, nil
}

func (s *Store) ListRounds(ctx context.Context, sessionID string) ([]*Round, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT session_id, number, phase, status, created_at, completed_at
		FROM rounds WHERE session_id = $1 ORDER BY number ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list rounds for %s: %w", sessionID, err)
	}
	defer rows.Close()
	var out []*Round
	for rows.Next() {
		var r Round
		if err := rows.Scan(&r.SessionID, &r.Number, &r.Phase, &r.Status, &r.CreatedAt, &r.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan round: %w", err)
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}

func (s *Store) CompleteRound(ctx context.Context, sessionID string, number int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE rounds SET status = 'completed', completed_at = now()
		WHERE session_id = $1 AND number = $2`, sessionID, number)
	if err != nil {
		return fmt.Errorf("complete round %d: %w", number, err)
	}
	return nil
}

func (s *Store) RegisterNode(ctx context.Context, n *NodeRecord) (int, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO session_nodes (session_id, node_id, addr, status)
		VALUES ($1, $2, $3, 'ready')
		ON CONFLICT (session_id, node_id) DO UPDATE SET addr = EXCLUDED.addr, status = 'ready'`,
		n.SessionID, n.NodeID, n.Addr)
	if err != nil {
		return 0, fmt.Errorf("register node %d: %w", n.NodeID, err)
	}
	var cnt int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM session_nodes WHERE session_id = $1", n.SessionID).Scan(&cnt); err != nil {
		return 0, fmt.Errorf("count nodes for %s: %w", n.SessionID, err)
	}
	return cnt, nil
}

func (s *Store) ListNodes(ctx context.Context, sessionID string) ([]*NodeRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT session_id, node_id, addr, status, registered_at
		FROM session_nodes WHERE session_id = $1 ORDER BY node_id ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list nodes for %s: %w", sessionID, err)
	}
	defer rows.Close()
	var out []*NodeRecord
	for rows.Next() {
		var n NodeRecord
		if err := rows.Scan(&n.SessionID, &n.NodeID, &n.Addr, &n.Status, &n.RegisteredAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		out = append(out, &n)
	}
	return out, rows.Err()
}

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	var typ, status string
	err := row.Scan(&s.ID, &typ, &status, &s.PartyCount, &s.Threshold, &s.Result, &s.ErrorMsg, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.Type = OpType(typ)
	s.Status = Status(status)
	return &s, nil
}

func collectSessions(rows pgx.Rows) ([]*Session, error) {
	var out []*Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
