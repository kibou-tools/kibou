// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package posix_tz

import (
	"strings"
	"testing"
	"unsafe"

	"pgregory.net/rapid"

	"code.kibou.tools/common/check"
	"code.kibou.tools/common/errorx"
	parseerr "code.kibou.tools/common/parse"
	. "code.kibou.tools/common/zero"
)

func TestParse(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("Unit", func(h check.Harness) {
		h.Parallel()
		h.Run("Valid", testParseValid)
		h.Run("Invalid", testParseInvalid)
	})

	h.Run("Property", func(h check.Harness) {
		h.Parallel()
		h.Run("NoPanic", testParseNoPanic)
		h.Run("ErrorIsParseError", testParseErrorShape)
		h.Run("FormatRoundTrip", testParseFormatRoundTrip)
		h.Run("ParseRoundTrip", testParseParseRoundTrip)
	})
}

func testParseValid(h check.Harness) {
	h.Parallel()

	h.Run("StandardOnly", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "UTC0")
		assertPOSIX(h, got, wantPOSIX{
			StdName:   "UTC",
			StdOffset: 0,
			DstName:   "",
			DstOffset: 0,
			HasDST:    false,
			Rule:      Zero[Rule](),
		})
	})

	h.Run("DSTRules", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "EST5EDT,M3.2.0,M11.1.0")
		assertPOSIX(h, got, wantPOSIX{
			StdName:   "EST",
			StdOffset: -5 * secondsPerHour,
			DstName:   "EDT",
			DstOffset: -4 * secondsPerHour,
			HasDST:    true,
			Rule:      Rule{Start: wantMonthWeekDayRuleDateTime(3, 2, 0, 2*secondsPerHour), End: wantMonthWeekDayRuleDateTime(11, 1, 0, 2*secondsPerHour)},
		})
	})

	h.Run("QuotedNamesAndExplicitOffsets", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "<-03>3<+05>-5,J60/1:02:03,365/-1")
		assertPOSIX(h, got, wantPOSIX{
			StdName:   "-03",
			StdOffset: -3 * secondsPerHour,
			DstName:   "+05",
			DstOffset: 5 * secondsPerHour,
			HasDST:    true,
			Rule:      Rule{Start: wantJulianRuleDateTime(60, secondsPerHour+2*secondsPerMinute+3), End: wantDayOfYearRuleDateTime(365, -secondsPerHour)},
		})
	})

	h.Run("ExplicitDSTOffset", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "AEST-10AEDT-11,M10.1.0,M4.1.0/3")
		check.AssertSame(h, true, got.HasDST, "has DST")
		check.AssertSame(h, "AEDT", got.DstName(), "dst name")
		check.AssertSame(h, int32(11*secondsPerHour), got.DstOffset(), "dst offset")
		assertRuleDateTime(h, "Start", wantMonthWeekDayRuleDateTime(10, 1, 0, 2*secondsPerHour), got.Rule().Start)
		assertRuleDateTime(h, "End", wantMonthWeekDayRuleDateTime(4, 1, 0, 3*secondsPerHour), got.Rule().End)
	})

	h.Run("GoIssueRegressions", testParseGoIssueRegressions)
}

// testParseGoIssueRegressions exercises POSIX TZ strings drawn from real
// IANA tzdata footers and from inputs that have historically tripped up
// golang/go's time package. Each case is anchored to a closed (or
// closed-by-rejection) golang/go issue so that future parser changes can
// be audited against parity with the stdlib's stated behavior.
func testParseGoIssueRegressions(h check.Harness) {
	h.Parallel()

	// golang/go#5361: Atlantic/South_Georgia used a quoted negative
	// abbreviation with no DST. The historical IANA footer for that zone
	// is "<-02>2", which Go's tzset accepts but earlier versions
	// mishandled. The parser must produce a UTC-2 zone with no DST.
	h.Run("Issue5361_QuotedNegativeNoDST", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "<-02>2")
		assertPOSIX(h, got, wantPOSIX{
			StdName:   "-02",
			StdOffset: -2 * secondsPerHour,
			HasDST:    false,
			Rule:      Zero[Rule](),
		})
	})

	// golang/go#3604: Southern-hemisphere zones encode DST that starts in
	// the back half of the year and ends in the front half — the END rule
	// is earlier in the calendar than the START rule. Australia's footer
	// "AEST-10AEDT,M10.1.0,M4.1.0/3" exercises this; the parser must keep
	// the rule order exactly as given without "normalizing" it.
	h.Run("Issue3604_SouthernHemisphereRuleOrder", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "AEST-10AEDT,M10.1.0,M4.1.0/3")
		assertPOSIX(h, got, wantPOSIX{
			StdName:   "AEST",
			StdOffset: 10 * secondsPerHour,
			DstName:   "AEDT",
			DstOffset: 11 * secondsPerHour, // default DST offset = std + 1h
			HasDST:    true,
			Rule: Rule{
				Start: wantMonthWeekDayRuleDateTime(10, 1, 0, 2*secondsPerHour),
				End:   wantMonthWeekDayRuleDateTime(4, 1, 0, 3*secondsPerHour),
			},
		})
	})

	// golang/go#8134: TZif v3 extended the transition-time range from
	// [0, 24h) to [-167h, 167h] so that footers can encode transitions
	// that fall on the previous or next civil day. Real-world example:
	// Chile uses "<-04>4<-03>,M9.1.6/24,M4.1.6/22", where "/24" means
	// 24:00:00 (midnight at the end of the day). The parser must accept
	// the value verbatim and preserve the sign and magnitude.
	h.Run("Issue8134_TransitionTimeAtOrPast24h", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "<-04>4<-03>,M9.1.6/24,M4.1.6/22")
		assertPOSIX(h, got, wantPOSIX{
			StdName:   "-04",
			StdOffset: -4 * secondsPerHour,
			DstName:   "-03",
			DstOffset: -3 * secondsPerHour,
			HasDST:    true,
			Rule: Rule{
				Start: wantMonthWeekDayRuleDateTime(9, 1, 6, 24*secondsPerHour),
				End:   wantMonthWeekDayRuleDateTime(4, 1, 6, 22*secondsPerHour),
			},
		})
	})

	// Sibling of Issue8134: the negative end of the extended range.
	// IANA documents "/-1" (the previous day at 23:00) as legal; the
	// existing QuotedNamesAndExplicitOffsets case covers this for a
	// day-of-year rule, but we also need the Mm.w.d form because that's
	// what real-world footers use.
	h.Run("Issue8134_NegativeTransitionTime", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "STD5DST,M3.2.0/-1,M11.1.0/2")
		check.AssertSame(h, true, got.HasDST, "has DST")
		assertRuleDateTime(h, "Start", wantMonthWeekDayRuleDateTime(3, 2, 0, -secondsPerHour), got.Rule().Start)
	})

	// golang/go#3385: Pre-1970 DST history is encoded only in the static
	// transition table, never in the POSIX footer (the footer captures
	// the rule applicable *after* the last static transition). The
	// parser has no opinion on history; it just needs to round-trip the
	// modern Mm.w.d form without dropping fields. Sanity check using a
	// minimal Europe-style footer.
	h.Run("Issue3385_LastSundayRule", func(h check.Harness) {
		h.Parallel()
		got := mustParsePOSIXTimeZone(h, "CET-1CEST,M3.5.0,M10.5.0/3")
		assertPOSIX(h, got, wantPOSIX{
			StdName:   "CET",
			StdOffset: 1 * secondsPerHour,
			DstName:   "CEST",
			DstOffset: 2 * secondsPerHour,
			HasDST:    true,
			Rule: Rule{
				Start: wantMonthWeekDayRuleDateTime(3, 5, 0, 2*secondsPerHour),
				End:   wantMonthWeekDayRuleDateTime(10, 5, 0, 3*secondsPerHour),
			},
		})
	})
}

func testParseInvalid(h check.Harness) {
	h.Parallel()

	for _, tt := range []struct {
		input             string
		wantKind          ParseErrorKind
		wantParsed        string
		wantErrorContains string
	}{
		{input: "", wantKind: ParseErrorKind_Empty, wantParsed: "", wantErrorContains: "expected - non-empty input"},
		{input: "ES5", wantKind: ParseErrorKind_SpecViolation, wantParsed: "ES", wantErrorContains: "context - trying to parse standard time abbreviation, expected - time abbreviation with at least 3 bytes"},
		{input: "EST", wantKind: ParseErrorKind_SpecViolation, wantParsed: "EST", wantErrorContains: "context - trying to parse standard time offset, expected - hour digits ([0-9]+)"},
		{input: "EST5EDT", wantKind: ParseErrorKind_SpecViolation, wantParsed: "EST5EDT", wantErrorContains: "context - trying to parse daylight saving time rule, expected - daylight saving time transition rule"},
		{input: "EST5EDT,M13.1.0,M11.1.0", wantKind: ParseErrorKind_SpecViolation, wantParsed: "EST5EDT,M13", wantErrorContains: "context - trying to parse daylight saving time rule start date/time, expected - transition-rule month in range [1,12]"},
		{input: "EST5EDT,M3.2.7,M11.1.0", wantKind: ParseErrorKind_SpecViolation, wantParsed: "EST5EDT,M3.2.7", wantErrorContains: "context - trying to parse daylight saving time rule start date/time, expected - transition-rule weekday in range [0,6]"},
		{input: "EST5EDT,M3.2.0", wantKind: ParseErrorKind_SpecViolation, wantParsed: "EST5EDT,M3.2.0", wantErrorContains: "context - trying to parse daylight saving time rule end date/time, expected - rule date/time separator"},
		{input: "EST25", wantKind: ParseErrorKind_SpecViolation, wantParsed: "EST25", wantErrorContains: "context - trying to parse standard time offset, expected - hour in range [-24,24]"},
		{input: "EST5EDT,M3.2.0/168,M11.1.0", wantKind: ParseErrorKind_SpecViolation, wantParsed: "EST5EDT,M3.2.0/168", wantErrorContains: "context - trying to parse daylight saving time rule start date/time, expected - hour in range [-167,167]"},
		{input: "EST5EDT,M3.2.0/2:60,M11.1.0", wantKind: ParseErrorKind_SpecViolation, wantParsed: "EST5EDT,M3.2.0/2:60", wantErrorContains: "context - trying to parse daylight saving time rule start date/time, expected - minute in range [0,59]"},
		{input: string([]byte{'E', 'S', 'T', 0xff, '5'}), wantKind: ParseErrorKind_InvalidUTF8, wantParsed: "", wantErrorContains: "invalid UTF-8"},
	} {
		h.Run(tt.input, func(h check.Harness) {
			h.Parallel()
			_, err := Parse(tt.input).Get()
			parseErr, ok := err.(*ParseError)
			h.Assertf(ok, "error = %T %[1]v, want *ParseError", err)
			check.AssertSame(h, tt.wantKind, parseErr.Kind(), "Kind")
			check.AssertSame(h, tt.input, parseErr.Source(), "Source")
			check.AssertSame(h, tt.wantParsed, parseErr.Parsed(), "Parsed")
			h.Assertf(strings.Contains(parseErr.Error(), tt.wantErrorContains), "Error() = %q, want substring %q", parseErr.Error(), tt.wantErrorContains)
		})
	}
}

// Parse must never panic on arbitrary input. rapid treats panics as property
// failures, so this also covers calls that succeed and calls that error.
func testParseNoPanic(h check.Harness) {
	h.Parallel()
	rapid.Check(h.T(), func(t *rapid.T) {
		input := arbitraryParseInput().Draw(t, "input")
		Parse(input)
	})
}

// On failure, Parse must return a *ParseError with consistent invariants:
// Source equals the input, Parsed is a prefix of the input, and Error() is
// non-empty so that the error message can be surfaced to users.
func testParseErrorShape(h check.Harness) {
	h.Parallel()
	rapid.Check(h.T(), func(t *rapid.T) {
		hh := check.NewBasic(t)
		input := arbitraryParseInput().Draw(t, "input")
		_, err := Parse(input).Get()
		if err == nil {
			return
		}
		parseErr, ok := err.(*ParseError)
		hh.Assertf(ok, "error = %T %[1]v, want *ParseError", err)
		diagnostic := errorx.GetRootCauseAs[*parseerr.Error](err).Unwrap()
		check.AssertSame(hh, input, parseErr.Source(), "Source")
		check.AssertSame(hh, input, diagnostic.Source(), "diagnostic Source")
		hh.Assertf(strings.HasPrefix(input, parseErr.Parsed()), "Parsed %q is not a prefix of Source %q", parseErr.Parsed(), input)
		hh.Assertf(parseErr.Error() != "", "Error() returned empty string")
	})
}

// Format → Parse must round-trip: any TimeZone we can construct must format
// to a string that parses back to an equal TimeZone.
func testParseFormatRoundTrip(h check.Harness) {
	h.Parallel()
	rapid.Check(h.T(), func(t *rapid.T) {
		hh := check.NewBasic(t)
		tz := arbitraryTimeZone().Draw(t, "tz")
		formatted := tz.Format()
		got, err := Parse(formatted).Get()
		hh.Assertf(err == nil, "Parse(%q) failed: %v", formatted, err)
		assertTimeZoneEqual(hh, tz, got, formatted)
	})
}

// Parse → Format → Parse must be idempotent: for any input that parses, the
// canonical formatted form must re-parse to an equal TimeZone. This catches
// Format/Parse asymmetries on input shapes (leading zeros, explicit '+',
// negative sub-hour offsets) that Format never emits itself.
func testParseParseRoundTrip(h check.Harness) {
	h.Parallel()
	rapid.Check(h.T(), func(t *rapid.T) {
		hh := check.NewBasic(t)
		input := arbitraryParseInput().Draw(t, "input")
		tz1, err := Parse(input).Get()
		if err != nil {
			return
		}
		formatted := tz1.Format()
		tz2, err := Parse(formatted).Get()
		hh.Assertf(err == nil, "Parse(%q) failed after Format(%v): %v", formatted, tz1, err)
		assertTimeZoneEqual(hh, tz1, tz2, formatted)
	})
}

func TestRuleLayout(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	size := unsafe.Sizeof(Zero[RuleDateTime]())
	h.Assertf(size <= 8, "RuleDateTime size = %d bytes, want at most 8", size)
}

func TestSecondInYear(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("JulianSkipsLeapDay", func(h check.Harness) {
		h.Parallel()
		dt := wantJulianRuleDateTime(60, 0)
		check.AssertSame(h, 59*secondsPerDay, dt.SecondInYear(2023), "common year")
		check.AssertSame(h, 60*secondsPerDay, dt.SecondInYear(2024), "leap year")
	})

	h.Run("DayOfYearIncludesLeapDay", func(h check.Harness) {
		h.Parallel()
		dt := wantDayOfYearRuleDateTime(60, 0)
		check.AssertSame(h, 60*secondsPerDay, dt.SecondInYear(2023), "common year")
		check.AssertSame(h, 60*secondsPerDay, dt.SecondInYear(2024), "leap year")
	})

	h.Run("FifthWeekMeansLastWeekdayInMonth", func(h check.Harness) {
		h.Parallel()
		dt := wantMonthWeekDayRuleDateTime(2, 5, 0, 0)
		check.AssertSame(h, 55*secondsPerDay, dt.SecondInYear(2024), "last Sunday in February 2024")
	})
}

// arbitraryParseInput mixes structured "almost-valid" inputs with arbitrary
// byte strings, so the property tests cover both error paths near the grammar
// and pathological inputs.
func arbitraryParseInput() *rapid.Generator[string] {
	return rapid.OneOf(
		rapid.StringN(0, 32, -1),
		rapid.Custom(func(t *rapid.T) string {
			return arbitraryTimeZone().Draw(t, "tz").Format()
		}),
		rapid.Custom(func(t *rapid.T) string {
			// Mutate a valid string by truncating or appending garbage.
			s := arbitraryTimeZone().Draw(t, "tz").Format()
			cut := rapid.IntRange(0, len(s)).Draw(t, "cut")
			tail := rapid.StringN(0, 8, -1).Draw(t, "tail")
			return s[:cut] + tail
		}),
	)
}

func arbitraryTimeZone() *rapid.Generator[TimeZone] {
	return rapid.Custom(func(t *rapid.T) TimeZone {
		stdName := arbitraryName().Draw(t, "stdName")
		stdOffset := arbitraryOffset().Draw(t, "stdOffset")
		hasDST := rapid.Bool().Draw(t, "hasDST")
		var dstName string
		var dstOffset int32
		var rule Rule
		if hasDST {
			dstName = arbitraryName().Draw(t, "dstName")
			dstOffset = arbitraryOffset().Draw(t, "dstOffset")
			rule = Rule{
				Start: arbitraryRuleDateTime().Draw(t, "start"),
				End:   arbitraryRuleDateTime().Draw(t, "end"),
			}
		}
		return TimeZone{
			StdName:   stdName,
			StdOffset: stdOffset,
			HasDST:    hasDST,
			dstName:   dstName,
			dstOffset: dstOffset,
			rule:      rule,
		}
	})
}

func arbitraryName() *rapid.Generator[string] {
	alpha := rapid.StringMatching(`[A-Za-z]{3,6}`)
	alnum := rapid.StringMatching(`[A-Za-z0-9+\-]{3,6}`)
	return rapid.OneOf(alpha, alnum)
}

func arbitraryOffset() *rapid.Generator[int32] {
	return rapid.Int32Range(-24*secondsPerHour, 24*secondsPerHour)
}

func arbitraryRuleDateTime() *rapid.Generator[RuleDateTime] {
	transitionSec := rapid.Int32Range(-167*secondsPerHour-59*secondsPerMinute-59, 167*secondsPerHour+59*secondsPerMinute+59)
	return rapid.Custom(func(t *rapid.T) RuleDateTime {
		second := transitionSec.Draw(t, "second")
		switch rapid.IntRange(0, 2).Draw(t, "kind") {
		case 0:
			day := rapid.Int16Range(1, 365).Draw(t, "julianDay")
			return wantJulianRuleDateTime(day, second)
		case 1:
			day := rapid.Int16Range(0, 365).Draw(t, "dayOfYear")
			return wantDayOfYearRuleDateTime(day, second)
		default:
			month := rapid.Uint8Range(1, 12).Draw(t, "month")
			week := rapid.Uint8Range(1, 5).Draw(t, "week")
			weekday := rapid.Uint8Range(0, 6).Draw(t, "weekday")
			return wantMonthWeekDayRuleDateTime(month, week, weekday, second)
		}
	})
}

func assertTimeZoneEqual(h check.BasicHarness, want TimeZone, got TimeZone, label string) {
	check.AssertSame(h, want.StdName, got.StdName, label+" std name")
	check.AssertSame(h, want.StdOffset, got.StdOffset, label+" std offset")
	check.AssertSame(h, want.HasDST, got.HasDST, label+" has DST")
	if !want.HasDST {
		return
	}
	check.AssertSame(h, want.dstName, got.dstName, label+" dst name")
	check.AssertSame(h, want.dstOffset, got.dstOffset, label+" dst offset")
	assertRuleDateTime(h, label+" start", want.rule.Start, got.rule.Start)
	assertRuleDateTime(h, label+" end", want.rule.End, got.rule.End)
}

type wantPOSIX struct {
	StdName   string
	StdOffset int32
	DstName   string
	DstOffset int32
	HasDST    bool
	Rule      Rule
}

func mustParsePOSIXTimeZone(h check.Harness, input string) TimeZone {
	h.T().Helper()
	got, err := Parse(input).Get()
	h.Assertf(err == nil, "Parse(%q) failed: %v", input, err)
	return got
}

func assertPOSIX(h check.Harness, got TimeZone, want wantPOSIX) {
	h.T().Helper()
	check.AssertSame(h, want.StdName, got.StdName, "std name")
	check.AssertSame(h, want.StdOffset, got.StdOffset, "std offset")
	check.AssertSame(h, want.HasDST, got.HasDST, "has DST")
	if want.HasDST {
		check.AssertSame(h, want.DstName, got.DstName(), "dst name")
		check.AssertSame(h, want.DstOffset, got.DstOffset(), "dst offset")
		assertRuleDateTime(h, "Start", want.Rule.Start, got.Rule().Start)
		assertRuleDateTime(h, "End", want.Rule.End, got.Rule().End)
	}
}

func assertRuleDateTime(h check.BasicHarness, label string, want RuleDateTime, got RuleDateTime) {
	check.AssertSame(h, want.Kind(), got.Kind(), label+" kind")
	check.AssertSame(h, want.Second(), got.Second(), label+" second")
	switch want.Kind() {
	case RuleDateTimeKind_Julian:
		check.AssertSame(h, want.JulianDay().Int16(), got.JulianDay().Int16(), label+" Julian day")
	case RuleDateTimeKind_DayOfYear:
		check.AssertSame(h, want.DayOfYear().Int16(), got.DayOfYear().Int16(), label+" day-of-year")
	case RuleDateTimeKind_MonthWeekDay:
		check.AssertSame(h, want.Month().Uint8(), got.Month().Uint8(), label+" month")
		check.AssertSame(h, want.Week().Uint8(), got.Week().Uint8(), label+" week")
		check.AssertSame(h, want.Weekday().Uint8(), got.Weekday().Uint8(), label+" weekday")
	default:
		h.Assertf(false, "unknown rule date/time kind: %v", want.Kind())
	}
}

func wantJulianRuleDateTime(day int16, second int32) RuleDateTime {
	return RuleDateTime{
		kind:      RuleDateTimeKind_Julian,
		day:       uint16(MustJulianDay(day).Int16()),
		second:    second,
		weekMonth: 0,
	}
}

func wantDayOfYearRuleDateTime(day int16, second int32) RuleDateTime {
	return RuleDateTime{
		kind:      RuleDateTimeKind_DayOfYear,
		day:       uint16(MustDayOfYear(day).Int16()),
		second:    second,
		weekMonth: 0,
	}
}

func wantMonthWeekDayRuleDateTime(month uint8, week uint8, weekday uint8, second int32) RuleDateTime {
	return RuleDateTime{
		kind:      RuleDateTimeKind_MonthWeekDay,
		weekMonth: NewWeekMonth(MustMonth(month), MustWeek(week)),
		day:       uint16(MustWeekday(weekday).Uint8()),
		second:    second,
	}
}
