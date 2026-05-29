// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package posix_tz

import (
	"code.kibou.tools/common/assert"
)

// The quoted abbreviation form and extended transition-time ranges are also
// documented by the IANA time-zone database theory file:
// https://data.iana.org/time-zones/tzdb/theory.html
const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour
)

// TimeZone is the parsed representation of a POSIX TZ
// string, as specified by POSIX.1-2024, Base Definitions, Ch. 8
// (Environment Variables), in the TZ entry:
// https://pubs.opengroup.org/onlinepubs/9799919799/basedefs/V1_chap08.html
type TimeZone struct {
	StdName string
	HasDST  bool

	// Seconds east of UTC.
	StdOffset int32

	rule      Rule
	dstName   string
	dstOffset int32 // Seconds east of UTC.
}

// DstName returns the DST abbreviation.
//
// Pre-condition: tz.HasDST must be true.
func (tz TimeZone) DstName() string {
	if !tz.HasDST {
		assert.Precondition(false, "DST name is only available when HasDST is true")
	}
	return tz.dstName
}

// DstOffset returns the DST offset in seconds east of UTC.
//
// Pre-condition: tz.HasDST must be true.
func (tz TimeZone) DstOffset() int32 {
	if !tz.HasDST {
		assert.Precondition(false, "DST offset is only available when HasDST is true")
	}
	return tz.dstOffset
}

// Rule returns the DST transition rule.
//
// Pre-condition: tz.HasDST must be true.
func (tz TimeZone) Rule() Rule {
	if !tz.HasDST {
		assert.Precondition(false, "DST rule is only available when HasDST is true")
	}
	return tz.rule
}

// Rule is a POSIX DST transition rule of the form:
//
// date[/time],date[/time]
type Rule struct {
	Start RuleDateTime
	End   RuleDateTime
}

// RuleDateTimeKind describes the format of the date retrieved
// from a rule.
//
// See code://docs/external/posix_tz.md#rule-date-format
type RuleDateTimeKind uint8

const (
	// RuleDateTimeKind_Julian indicates the date is of the form
	// 1 <= n <= 365, with leap days being disallowed.
	RuleDateTimeKind_Julian RuleDateTimeKind = iota + 1
	RuleDateTimeKind_DayOfYear
	RuleDateTimeKind_MonthWeekDay
)

// RuleDateTime is one date[/time] component in a POSIX DST transition rule.
type RuleDateTime struct {
	// second is in the range [-604799,604799].
	second    int32
	day       uint16 // Julian day, day-of-year or weekday, depending on kind.
	kind      RuleDateTimeKind
	weekMonth WeekMonth
}

// Second returns the rule part's transition time as a signed second offset from midnight.
func (dt RuleDateTime) Second() int32 {
	return dt.second
}

// Kind returns the rule part variant.
func (dt RuleDateTime) Kind() RuleDateTimeKind {
	return dt.kind
}

// JulianDay returns the one-based Julian day for a Julian rule part.
//
// Pre-condition: dt.Kind() must be RuleDateTimeKind_Julian.
func (dt RuleDateTime) JulianDay() JulianDay {
	if dt.kind != RuleDateTimeKind_Julian {
		assert.Precondition(false, "Julian day is only available for Julian rule parts")
	}
	return MustJulianDay(int16(dt.day))
}

// DayOfYear returns the zero-based day-of-year for a day-of-year rule part.
//
// Pre-condition: dt.Kind() must be RuleDateTimeKind_DayOfYear.
func (dt RuleDateTime) DayOfYear() DayOfYear {
	if dt.kind != RuleDateTimeKind_DayOfYear {
		assert.Precondition(false, "day-of-year is only available for day-of-year rule parts")
	}
	return MustDayOfYear(int16(dt.day))
}

// Weekday returns the weekday for a month-week-day rule part.
//
// Pre-condition: dt.Kind() must be RuleDateTimeKind_MonthWeekDay.
func (dt RuleDateTime) Weekday() Weekday {
	if dt.kind != RuleDateTimeKind_MonthWeekDay {
		assert.Precondition(false, "weekday is only available for month-week-day rule parts")
	}
	return MustWeekday(uint8(dt.day))
}

// Week returns the week number for a month-week-day rule part.
//
// Pre-condition: dt.Kind() must be RuleDateTimeKind_MonthWeekDay.
func (dt RuleDateTime) Week() Week {
	if dt.kind != RuleDateTimeKind_MonthWeekDay {
		assert.Precondition(false, "week is only available for month-week-day rule parts")
	}
	return dt.weekMonth.Week()
}

// Month returns the month for a month-week-day rule part.
//
// Pre-condition: dt.Kind() must be RuleDateTimeKind_MonthWeekDay.
func (dt RuleDateTime) Month() Month {
	if dt.kind != RuleDateTimeKind_MonthWeekDay {
		assert.Precondition(false, "month is only available for month-week-day rule parts")
	}
	return dt.weekMonth.Month()
}

// SecondInYear returns the moment this rule fires, expressed as a signed
// offset in seconds from midnight at the start of Jan 1 of `year` in
// local wall time (no UTC offset applied).
//
// The result can be negative or exceed the year length because POSIX
// permits transition times outside [0, 24h) — e.g., "/-1" means 23:00 the
// previous day, "/26" means 02:00 the next.
//
// TODO: I dislike the shape of this API, because it makes it difficult
// for the caller to understand what to do. Let's try to improve upon
// it in a later pass.
func (dt RuleDateTime) SecondInYear(year int) int {
	switch dt.Kind() {
	case RuleDateTimeKind_Julian:
		return dt.JulianDay().zeroBasedDayOfYear(year)*secondsPerDay + int(dt.Second())
	case RuleDateTimeKind_DayOfYear:
		return dt.DayOfYear().Int()*secondsPerDay + int(dt.Second())
	case RuleDateTimeKind_MonthWeekDay:
		month := dt.Month()
		day := nthWeekdayOfMonth(year, month, dt.Weekday(), dt.Week())
		return (dayOfYear(year, month.Int(), day)-1)*secondsPerDay + int(dt.Second())
	default:
		return assert.PanicUnknownCase[int](dt.Kind())
	}
}
