package protocol

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

var DefaultPrime, _ = new(big.Int).SetString("170141183460469231731687303715884105727", 10)

type Field struct {
	P *big.Int
}

type FieldElement struct {
	v *big.Int
}

func NewField() *Field {
	return &Field{P: new(big.Int).Set(DefaultPrime)}
}

func NewFieldWithPrime(p *big.Int) *Field {
	return &Field{P: new(big.Int).Set(p)}
}

func (f *Field) reduce(x *big.Int) *big.Int {
	r := new(big.Int).Mod(x, f.P)
	if r.Sign() < 0 {
		r.Add(r, f.P)
	}
	return r
}

func (f *Field) Element(x *big.Int) FieldElement {
	return FieldElement{v: f.reduce(x)}
}

func (f *Field) FromInt(n int64) FieldElement {
	return FieldElement{v: f.reduce(big.NewInt(n))}
}

func (f *Field) FromString(s string) (FieldElement, error) {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return FieldElement{}, fmt.Errorf("parse field element %q", s)
	}
	return FieldElement{v: f.reduce(n)}, nil
}

func (f *Field) FromBytes(b []byte) FieldElement {
	return FieldElement{v: f.reduce(new(big.Int).SetBytes(b))}
}

func (f *Field) Zero() FieldElement {
	return FieldElement{v: big.NewInt(0)}
}

func (f *Field) One() FieldElement {
	return FieldElement{v: big.NewInt(1)}
}

func (f *Field) Add(a, b FieldElement) FieldElement {
	return FieldElement{v: f.reduce(new(big.Int).Add(a.v, b.v))}
}

func (f *Field) Sub(a, b FieldElement) FieldElement {
	return FieldElement{v: f.reduce(new(big.Int).Sub(a.v, b.v))}
}

func (f *Field) Mul(a, b FieldElement) FieldElement {
	return FieldElement{v: f.reduce(new(big.Int).Mul(a.v, b.v))}
}

func (f *Field) Neg(a FieldElement) FieldElement {
	return FieldElement{v: f.reduce(new(big.Int).Neg(a.v))}
}

func (f *Field) Inv(a FieldElement) (FieldElement, error) {
	if a.v.Sign() == 0 {
		return FieldElement{}, fmt.Errorf("inverse of zero in GF(p)")
	}
	inv := new(big.Int).ModInverse(a.v, f.P)
	if inv == nil {
		return FieldElement{}, fmt.Errorf("element %s has no inverse mod p", a.v.String())
	}
	return FieldElement{v: inv}, nil
}

func (f *Field) ScalarMul(a FieldElement, k *big.Int) FieldElement {
	return FieldElement{v: f.reduce(new(big.Int).Mul(a.v, k))}
}

func (f *Field) Rand() (FieldElement, error) {
	n, err := rand.Int(rand.Reader, f.P)
	if err != nil {
		return FieldElement{}, fmt.Errorf("sample field element: %w", err)
	}
	return FieldElement{v: n}, nil
}

func (e FieldElement) Big() *big.Int {
	return new(big.Int).Set(e.v)
}

func (e FieldElement) Bytes() []byte {
	if e.v == nil {
		return []byte{}
	}
	return e.v.Bytes()
}

func (e FieldElement) String() string {
	if e.v == nil {
		return "0"
	}
	return e.v.String()
}

func (e FieldElement) Equal(other FieldElement) bool {
	return e.v.Cmp(other.v) == 0
}

func (e FieldElement) IsZero() bool {
	return e.v == nil || e.v.Sign() == 0
}
