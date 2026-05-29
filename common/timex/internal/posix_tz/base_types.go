// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package timex_base defines small bounded time/calendar value types.
package posix_tz

import (
	. "code.kibou.tools/common/zero"
	"code.kibou.tools/common/assert"
)

// DayOfYear is a zero-based day-of-year in the POSIX rule format,
// potentially allowing for leap years.
//
// The underlying value is in the range [0, 365], with Jan 1 ~ 0,
// and Dec 31 ~ 364 for non-leap years, and 365 for leap years.
type DayOfYear struct{ value int16 }

func NewDayOfYear(value int16) (DayOfYear, bool) {
	if value < 0 || value > 365 {
		return Zero[DayOfYear](), false
	}
	return DayOfYear{value: value}, true
}

func MustDayOfYear(value int16) DayOfYear {
	day, ok := NewDayOfYear(value)
	if !ok {
		assert.Preconditionf(false, "day-of-year %d out of range", value)
	}
	return day
}

func (day DayOfYear) Int16() int16 { return day.value }
func (day DayOfYear) Int() int     { return int(day.value) }

// JulianDay is a one-based POSIX Julian day that does not count leap days.
//
// See code://docs/external/posix_tz.md#rule-date-format
type JulianDay struct{ value int16 }

func NewJulianDay(value int16) (JulianDay, bool) {
	if value < 1 || value > 365 {
		return Zero[JulianDay](), false
	}
	return JulianDay{value: value}, true
}

func MustJulianDay(value int16) JulianDay {
	day, ok := NewJulianDay(value)
	if !ok {
		assert.Preconditionf(false, "Julian day %d out of range", value)
	}
	return day
}

func (day JulianDay) Int16() int16 { return day.value }
func (day JulianDay) Int() int     { return int(day.value) }

const firstJulianDayAfterFeb29 = 60

// zeroBasedDayOfYear returns a value in the range [0, 365].
func (day JulianDay) zeroBasedDayOfYear(year int) int {
	yearDay := day.Int() - 1
	if isLeapYear(year) && day.Int() >= firstJulianDayAfterFeb29 {
		yearDay++
	}
	return yearDay
}

// Zero-based weekday value, with 0 = Sunday.
type Weekday struct{ value uint8 }

func NewWeekday(value uint8) (Weekday, bool) {
	if value > 6 {
		return Zero[Weekday](), false
	}
	return Weekday{value: value}, true
}

func MustWeekday(value uint8) Weekday {
	weekday, ok := NewWeekday(value)
	if !ok {
		assert.Preconditionf(false, "weekday %d out of range", value)
	}
	return weekday
}

// Uint8 returns the weekday value in the range [0, 6].
func (weekday Weekday) Uint8() uint8 { return weekday.value }

// Int returns the weekday value in the range [0, 6].
func (weekday Weekday) Int() int { return int(weekday.value) }

type Week struct {
	// value ∈ [1, 5]
	value uint8
}

func NewWeek(value uint8) (Week, bool) {
	if value < 1 || value > 5 {
		return Zero[Week](), false
	}
	return Week{value: value}, true
}

func MustWeek(value uint8) Week {
	week, ok := NewWeek(value)
	if !ok {
		assert.Preconditionf(false, "week %d out of range", value)
	}
	return week
}

// Uint8 returns a value in the range [1, 5]
func (week Week) Uint8() uint8 { return week.value }

// Int returns a value in the range [1, 5]
func (week Week) Int() int { return int(week.value) }

type Month struct {
	// value ∈ [1, 12]
	value uint8
}

func NewMonth(value uint8) (Month, bool) {
	if value < 1 || value > 12 {
		return Zero[Month](), false
	}
	return Month{value: value}, true
}

func MustMonth(value uint8) Month {
	month, ok := NewMonth(value)
	if !ok {
		assert.Preconditionf(false, "month %d out of range", value)
	}
	return month
}

// Uint8 returns the month value in the range [1, 12].
func (month Month) Uint8() uint8 { return month.value }

// Int returns the month value in the range [1, 12].
func (month Month) Int() int { return int(month.value) }

type WeekMonth uint8

func NewWeekMonth(month Month, week Week) WeekMonth {
	// month.Uint8() ∈ [1, 12] and week.Uint8() ∈ [1, 5]
	// so this conversion is lossless.
	return WeekMonth(month.Uint8()<<4 | week.Uint8())
}

func (wm WeekMonth) Month() Month {
	month, ok := NewMonth(uint8(wm) >> 4)
	if !ok {
		assert.Invariantf(false, "WeekMonth has invalid month bits: %d", uint8(wm)>>4)
	}
	return month
}

func (wm WeekMonth) Week() Week {
	week, ok := NewWeek(uint8(wm) & 0x0f)
	if !ok {
		assert.Invariantf(false, "WeekMonth has invalid week bits: %d", uint8(wm)&0x0f)
	}
	return week
}
