// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package posix_tz

// Computes the 0-based weekday value (with 0 ~ Sunday),
// based on the Proleptic Gregorian calendar, with 0001-01-01
// being considered a Monday.
func computeWeekday(year int, month int, day int) int {
	// Days before Jan 1 of `year` is 365·y + ⌊y/4⌋ − ⌊y/100⌋ + ⌊y/400⌋
	// where y = year − 1.
	y := year - 1
	// Since 365 ≡ 1 (mod 7), 365·y ≡ y (mod 7), reduce the (y mod 7)
	// up front to avoid overflow.
	base := modFloor(y, 7)
	// The remaining leap-day terms sum to (1/4 − 1/100 + 1/400)·y ≈ 0.2425·y,
	// well within int range.
	leapYearAdjustment := divFloor(y, 4) - divFloor(y, 100) + divFloor(y, 400)
	currentYearAdjustment := dayOfYear(year, month, day)
	return modFloor(base+leapYearAdjustment+currentYearAdjustment, 7)
}

// dayOfYear returns the 1-based day in the year, with the value
// being in [1, 365] during non-leap years, and in [1, 366] in leap years.
func dayOfYear(year int, month int, day int) int {
	day += daysBeforeMonth(month)
	if month > 2 && isLeapYear(year) {
		day++
	}
	return day
}

// Pre-condition: month ∈ [1, 12]
func daysInMonth(year int, month int) int {
	switch month {
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// Pre-condition: month ∈ [1, 12]
func daysBeforeMonth(month int) int {
	return [12]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}[month-1]
}

// nthWeekdayOfMonth returns the day-of-month of the n-th `weekday` in
// (year, month), per POSIX Mm.w.d semantics: n=5 means "last occurrence",
// i.e., the 5th if it exists, otherwise the 4th.
//
// Per code://docs/external/posix_tz.md#rule-date-format, weekdays are
// computed based on the Proleptic Gregorian calendar.
func nthWeekdayOfMonth(year int, month Month, weekday Weekday, n Week) int {
	m := month.Int()
	firstOfMonthWeekday := computeWeekday(year, m, 1)
	// For 'weekday' (e.g. Friday), compute the first day in the month
	// for which the day is that weekday. => firstHit ∈ [1, 7]
	// E.g. if first day is Tuesday, then Fri - Tue = 3, so 4th of the
	// month will be the first Friday.
	firstHit := 1 + modFloor(weekday.Int()-firstOfMonthWeekday, 7)
	// n = 1 => firstHit is correct, if n >= 2, we have to advance by 7.
	day := firstHit + (n.Int()-1)*7
	if day > daysInMonth(year, m) {
		// n ∈ [1, 5], but it's possible for a month to not have (say)
		// a 5th Friday, so in that case, n = 5 means "last", so go back
		// by 1 week.
		day -= 7
	}
	return day
}

// divFloor returns floor(x/y) (i.e. rounding towards -∞),
// unlike Go's / operator, which rounds toward 0.
//
// Pre-condition: y > 0.
func divFloor(x int, y int) int {
	q := x / y
	// If x < 0 && x % y != 0, (e.g. x = -9, y = 4),
	// we need to subtract 1 (adjust -2 -> -3).
	if x%y < 0 {
		q--
	}
	return q
}

// modFloor returns x mod y in the range [0, y)
// (i.e. textbook Euclidean mod) unlike Go's % operator,\
// which keeps the sign of x.
//
// Pre-condition: y > 0.
func modFloor(x int, y int) int {
	r := x % y
	if r < 0 {
		r += y
	}
	return r
}
