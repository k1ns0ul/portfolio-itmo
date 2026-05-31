package protocol

import "fmt"

type BeaverTriple struct {
	A FieldElement
	B FieldElement
	C FieldElement
}

type SharedTriple struct {
	A FieldElement
	B FieldElement
	C FieldElement
}

func GenerateTriple(f *Field) (BeaverTriple, error) {
	a, err := f.Rand()
	if err != nil {
		return BeaverTriple{}, fmt.Errorf("triple factor a: %w", err)
	}
	b, err := f.Rand()
	if err != nil {
		return BeaverTriple{}, fmt.Errorf("triple factor b: %w", err)
	}
	return BeaverTriple{A: a, B: b, C: f.Mul(a, b)}, nil
}

func ShareTriple(f *Field, t BeaverTriple, n int) ([]SharedTriple, error) {
	aShares, err := f.AdditiveShare(t.A, n)
	if err != nil {
		return nil, fmt.Errorf("share a: %w", err)
	}
	bShares, err := f.AdditiveShare(t.B, n)
	if err != nil {
		return nil, fmt.Errorf("share b: %w", err)
	}
	cShares, err := f.AdditiveShare(t.C, n)
	if err != nil {
		return nil, fmt.Errorf("share c: %w", err)
	}
	out := make([]SharedTriple, n)
	for i := 0; i < n; i++ {
		out[i] = SharedTriple{A: aShares[i], B: bShares[i], C: cShares[i]}
	}
	return out, nil
}

func MulOpen(f *Field, xShare, yShare FieldElement, t SharedTriple) (dShare, eShare FieldElement) {
	dShare = f.Sub(xShare, t.A)
	eShare = f.Sub(yShare, t.B)
	return dShare, eShare
}

func MulFinalize(f *Field, d, e FieldElement, t SharedTriple, myIndex int) FieldElement {
	res := t.C
	res = f.Add(res, f.Mul(d, t.B))
	res = f.Add(res, f.Mul(e, t.A))
	if myIndex == 0 {
		res = f.Add(res, f.Mul(d, e))
	}
	return res
}
