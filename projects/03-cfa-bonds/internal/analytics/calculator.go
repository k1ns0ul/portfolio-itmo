package analytics

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

func CurrentYield(nominal, couponRate, price decimal.Decimal) (decimal.Decimal, error) {
	if price.IsZero() {
		return decimal.Zero, fmt.Errorf("current yield undefined at zero price")
	}
	annualCoupon := couponRate.Mul(nominal)
	return annualCoupon.Div(price).Round(6), nil
}

func CalculateYTM(nominal, price, couponRate decimal.Decimal, frequency, remainingPeriods int) (decimal.Decimal, error) {
	if frequency <= 0 {
		return decimal.Zero, fmt.Errorf("invalid coupon frequency %d", frequency)
	}
	if remainingPeriods <= 0 {
		return decimal.Zero, fmt.Errorf("no remaining periods for YTM")
	}
	if price.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("price must be positive for YTM")
	}

	face, _ := nominal.Float64()
	p, _ := price.Float64()
	rate, _ := couponRate.Float64()
	coupon := face * rate / float64(frequency)
	n := remainingPeriods

	y := coupon / p
	if y <= 0 {
		y = 0.05 / float64(frequency)
	}

	for iter := 0; iter < 60; iter++ {
		value := -p
		deriv := 0.0
		for t := 1; t <= n; t++ {
			cf := coupon
			if t == n {
				cf += face
			}
			disc := math.Pow(1+y, float64(t))
			value += cf / disc
			deriv += -float64(t) * cf / (disc * (1 + y))
		}
		if math.Abs(deriv) < 1e-12 {
			break
		}
		step := value / deriv
		y -= step
		if y <= -0.9999 {
			y = -0.9999
		}
		if math.Abs(step) < 1e-10 {
			break
		}
	}

	annual := y * float64(frequency)
	if math.IsNaN(annual) || math.IsInf(annual, 0) {
		return decimal.Zero, fmt.Errorf("YTM did not converge")
	}
	return decimal.NewFromFloat(annual).Round(6), nil
}
