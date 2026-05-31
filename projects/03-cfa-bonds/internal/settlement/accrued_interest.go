package settlement

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

const daysBasis = 365

func CalcAccruedInterest(nominal, couponRate decimal.Decimal, frequency int, lastCouponDate, settlementDate time.Time) (decimal.Decimal, error) {
	if frequency <= 0 {
		return decimal.Zero, fmt.Errorf("coupon frequency must be positive, got %d", frequency)
	}
	if settlementDate.Before(lastCouponDate) {
		return decimal.Zero, fmt.Errorf("settlement %s precedes last coupon %s",
			settlementDate.Format("2006-01-02"), lastCouponDate.Format("2006-01-02"))
	}

	daysElapsed := int(settlementDate.Sub(lastCouponDate).Hours() / 24)
	periodDays := daysBasis / frequency
	if periodDays <= 0 {
		return decimal.Zero, fmt.Errorf("degenerate coupon period for frequency %d", frequency)
	}
	if daysElapsed > periodDays {
		daysElapsed = periodDays
	}

	couponAmount := nominal.Mul(couponRate).Div(decimal.NewFromInt(int64(frequency)))
	fraction := decimal.NewFromInt(int64(daysElapsed)).Div(decimal.NewFromInt(int64(periodDays)))
	return couponAmount.Mul(fraction).Round(6), nil
}

func CouponAmountPerUnit(nominal, couponRate decimal.Decimal, frequency int) decimal.Decimal {
	return nominal.Mul(couponRate).Div(decimal.NewFromInt(int64(frequency))).Round(6)
}
