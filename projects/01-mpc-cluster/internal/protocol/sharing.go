package protocol

import (
	"fmt"

	"github.com/hashicorp/vault/shamir"
)

func (f *Field) AdditiveShare(secret FieldElement, n int) ([]FieldElement, error) {
	if n < 2 {
		return nil, fmt.Errorf("need at least 2 parties, got %d", n)
	}
	shares := make([]FieldElement, n)
	acc := f.Zero()
	for i := 0; i < n-1; i++ {
		r, err := f.Rand()
		if err != nil {
			return nil, fmt.Errorf("share %d/%d: %w", i, n, err)
		}
		shares[i] = r
		acc = f.Add(acc, r)
	}
	shares[n-1] = f.Sub(secret, acc)
	return shares, nil
}

func (f *Field) AdditiveRecombine(shares []FieldElement) FieldElement {
	sum := f.Zero()
	for _, s := range shares {
		sum = f.Add(sum, s)
	}
	return sum
}

func ShamirSplit(secret []byte, n, threshold int) ([][]byte, error) {
	if threshold < 2 || threshold > n {
		return nil, fmt.Errorf("invalid threshold %d for %d parts", threshold, n)
	}
	parts, err := shamir.Split(secret, n, threshold)
	if err != nil {
		return nil, fmt.Errorf("shamir split (n=%d t=%d): %w", n, threshold, err)
	}
	return parts, nil
}

func ShamirCombine(parts [][]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("no parts to combine")
	}
	secret, err := shamir.Combine(parts)
	if err != nil {
		return nil, fmt.Errorf("shamir combine %d parts: %w", len(parts), err)
	}
	return secret, nil
}
