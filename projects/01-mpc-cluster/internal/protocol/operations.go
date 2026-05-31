package protocol

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
)

type PeerNetwork interface {
	NextRound() int
	Broadcast(ctx context.Context, roundNum int, data []byte) error
	Gather(ctx context.Context, roundNum int) (map[int][]byte, error)
}

func (f *Field) SecureAdd(aShares, bShares []FieldElement) ([]FieldElement, error) {
	if len(aShares) != len(bShares) {
		return nil, fmt.Errorf("share count mismatch: %d vs %d", len(aShares), len(bShares))
	}
	out := make([]FieldElement, len(aShares))
	for i := range aShares {
		out[i] = f.Add(aShares[i], bShares[i])
	}
	return out, nil
}

func ScalarMul(f *Field, share FieldElement, scalar *big.Int) FieldElement {
	return f.ScalarMul(share, scalar)
}

func SecureMul(ctx context.Context, f *Field, xShare, yShare FieldElement, myIndex int, triple SharedTriple, peers PeerNetwork) (FieldElement, error) {
	dShare, eShare := MulOpen(f, xShare, yShare, triple)

	round := peers.NextRound()
	if err := peers.Broadcast(ctx, round, encodePair(dShare.Bytes(), eShare.Bytes())); err != nil {
		return FieldElement{}, fmt.Errorf("broadcast d,e round %d: %w", round, err)
	}
	others, err := peers.Gather(ctx, round)
	if err != nil {
		return FieldElement{}, fmt.Errorf("gather d,e round %d: %w", round, err)
	}

	d := dShare
	e := eShare
	for sender, raw := range others {
		db, eb, derr := decodePair(raw)
		if derr != nil {
			return FieldElement{}, fmt.Errorf("decode share from node %d: %w", sender, derr)
		}
		d = f.Add(d, f.FromBytes(db))
		e = f.Add(e, f.FromBytes(eb))
	}

	return MulFinalize(f, d, e, triple, myIndex), nil
}

func SecureCompare(ctx context.Context, f *Field, xShare, yShare FieldElement, myIndex int, triples []SharedTriple, peers PeerNetwork, bitLen int) (FieldElement, error) {
	if bitLen <= 0 || bitLen > 126 {
		return FieldElement{}, fmt.Errorf("bit length %d out of range", bitLen)
	}
	diff := f.Sub(xShare, yShare)

	round := peers.NextRound()
	if err := peers.Broadcast(ctx, round, diff.Bytes()); err != nil {
		return FieldElement{}, fmt.Errorf("broadcast diff round %d: %w", round, err)
	}
	others, err := peers.Gather(ctx, round)
	if err != nil {
		return FieldElement{}, fmt.Errorf("gather diff round %d: %w", round, err)
	}

	opened := diff
	for _, raw := range others {
		opened = f.Add(opened, f.FromBytes(raw))
	}

	half := new(big.Int).Rsh(f.P, 1)
	gt := 0
	val := opened.Big()
	if val.Sign() != 0 && val.Cmp(half) < 0 {
		gt = 1
	}

	if myIndex == 0 {
		return f.FromInt(int64(gt)), nil
	}
	return f.Zero(), nil
}

func encodePair(a, b []byte) []byte {
	buf := make([]byte, 0, 8+len(a)+len(b))
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(a)))
	buf = append(buf, hdr[:]...)
	buf = append(buf, a...)
	binary.BigEndian.PutUint32(hdr[:], uint32(len(b)))
	buf = append(buf, hdr[:]...)
	buf = append(buf, b...)
	return buf
}

func decodePair(raw []byte) ([]byte, []byte, error) {
	if len(raw) < 4 {
		return nil, nil, fmt.Errorf("payload too short: %d bytes", len(raw))
	}
	la := binary.BigEndian.Uint32(raw[:4])
	off := 4 + int(la)
	if len(raw) < off+4 {
		return nil, nil, fmt.Errorf("truncated payload, want %d have %d", off+4, len(raw))
	}
	a := raw[4:off]
	lb := binary.BigEndian.Uint32(raw[off : off+4])
	off += 4
	if len(raw) < off+int(lb) {
		return nil, nil, fmt.Errorf("truncated second element")
	}
	b := raw[off : off+int(lb)]
	return a, b, nil
}
