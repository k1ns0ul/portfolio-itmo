package db

import (
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

func NumericFromDecimal(d decimal.Decimal) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   d.Coefficient(),
		Exp:   d.Exponent(),
		Valid: true,
	}
}

func DecimalFromNumeric(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid {
		return decimal.Zero, nil
	}
	if n.NaN {
		return decimal.Zero, fmt.Errorf("numeric value is NaN")
	}
	if n.InfinityModifier != pgtype.Finite {
		return decimal.Zero, fmt.Errorf("numeric value is not finite")
	}
	coeff := n.Int
	if coeff == nil {
		coeff = big.NewInt(0)
	}
	return decimal.NewFromBigInt(coeff, n.Exp), nil
}

func MustDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(fmt.Sprintf("invalid decimal literal %q: %v", s, err))
	}
	return d
}
