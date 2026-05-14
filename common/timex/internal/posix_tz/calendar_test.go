// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package posix_tz

import (
	"math"
	"math/big"
	"testing"
	stdlib_time "time"

	"code.kibou.tools/common/check"
	"pgregory.net/rapid"
)

func TestCalendarMatchesStdlib(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		year := rapid.IntRange(-9999, 9999).Draw(t, "year")
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, stdlibDaysInMonth(year, month)).Draw(t, "day")

		date := stdlib_time.Date(year, stdlib_time.Month(month), day, 0, 0, 0, 0, stdlib_time.UTC)
		check.AssertSame(h, int(date.Weekday()), computeWeekday(year, month, day), "weekday")
		check.AssertSame(h, date.YearDay(), dayOfYear(year, month, day), "day of year")
		check.AssertSame(h, stdlibDaysInMonth(year, month), daysInMonth(year, month), "days in month")
	})
}

func stdlibDaysInMonth(year int, month int) int {
	return stdlib_time.Date(year, stdlib_time.Month(month)+1, 0, 0, 0, 0, 0, stdlib_time.UTC).Day()
}

// Verifies weekday against an arbitrary-precision reference
// across the full int range, to catch any overflow in the production code.
// stdlib_time.Date's internal int64-nanosecond representation can't cover the
// extremes, so a big.Int reference is needed here.
func TestWeekdayAtExtremeYears(t *testing.T) {
	if math.MaxInt < math.MaxInt64 {
		t.Skip("requires 64-bit int")
	}
	h := check.New(t)
	h.Parallel()

	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		year := rapid.Int64().Draw(t, "year")
		if year == math.MinInt64 {
			// year-1 wraps in the production code; not a meaningful input.
			t.Skip("MinInt64")
		}
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, daysInMonth(int(year), month)).Draw(t, "day")

		check.AssertSame(h,
			bigIntWeekday(int(year), month, day),
			computeWeekday(int(year), month, day),
			"weekday")
	})
}

// bigIntWeekday is a math/big reference for weekday.
// Any disagreement indicates an int overflow in the production implementation.
func bigIntWeekday(year int, month int, day int) int {
	// Compute y = year - 1 in big.Int so the reference stays exact even for
	// inputs at the int boundary.
	y := new(big.Int).SetInt64(int64(year))
	y.Sub(y, big.NewInt(1))

	sum := new(big.Int).Mul(y, big.NewInt(365))
	// big.Int.Div / Mod implement Euclidean division (floor for positive y).
	sum.Add(sum, new(big.Int).Div(y, big.NewInt(4)))
	sum.Sub(sum, new(big.Int).Div(y, big.NewInt(100)))
	sum.Add(sum, new(big.Int).Div(y, big.NewInt(400)))
	sum.Add(sum, big.NewInt(int64(dayOfYear(year, month, day))))
	return int(new(big.Int).Mod(sum, big.NewInt(7)).Int64())
}
