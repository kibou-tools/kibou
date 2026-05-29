// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package posix_tz

import (
	"code.kibou.tools/common/assert"
	parseerr "code.kibou.tools/common/parse"
)

// ParseErrorKind identifies why POSIX TZ parsing failed.
type ParseErrorKind uint8

const (
	ParseErrorKind_Empty ParseErrorKind = iota + 1
	ParseErrorKind_InvalidUTF8
	ParseErrorKind_SpecViolation
	ParseErrorKind_TrailingData
)

type want uint8

const (
	want_None want = iota
	want_NameAtLeast3Bytes
	want_NameClosingQuote
	want_HourDigits
	want_HourInRange
	want_TwoMinuteDigits
	want_MinuteInRange
	want_TwoSecondDigits
	want_SecondInRange
	want_DSTRule
	want_CommaBeforeRule
	want_RuleSeparator
	want_JulianDay
	want_DayOfYear
	want_Month
	want_Week
	want_Weekday
)

type parseCtx uint8

const (
	parseCtx_StandardName parseCtx = iota + 1
	parseCtx_StandardOffset
	parseCtx_DSTName
	parseCtx_DSTOffset
	parseCtx_DSTRule
	parseCtx_RuleStartDateTime
	parseCtx_RuleEndDateTime
)

type specViolation struct {
	expected want
	context  parseCtx
}

// ParseError reports a failed POSIX TZ parse.
type ParseError struct {
	kind ParseErrorKind
	// Always non-nil.
	diagnostic *parseerr.Error
}

func newParseError(kind ParseErrorKind, source string, parsed string) *ParseError {
	options := parseerr.ErrorOptions{Parsed: parsed, Context: "", Expected: "", Reason: ""}
	switch kind {
	case ParseErrorKind_Empty:
		options.Expected = "non-empty input"
	case ParseErrorKind_InvalidUTF8:
		options.Reason = "invalid UTF-8"
	case ParseErrorKind_SpecViolation:
		assert.PanicUnknownCase[any](kind)
	case ParseErrorKind_TrailingData:
		options.Expected = "end of input"
	default:
		assert.PanicUnknownCase[any](kind)
	}
	return &ParseError{kind: kind, diagnostic: parseerr.NewError("POSIX TZ", source, options)}
}

func newSpecViolationParseError(violation specViolation, source string, parsed string) *ParseError {
	options := parseerr.ErrorOptions{
		Parsed:   parsed,
		Context:  violation.context.reason(),
		Expected: violation.expected.expected(violation.context),
		Reason:   "",
	}
	return &ParseError{kind: ParseErrorKind_SpecViolation, diagnostic: parseerr.NewError("POSIX TZ", source, options)}
}

func (e *ParseError) Kind() ParseErrorKind { return e.kind }
func (e *ParseError) Source() string       { return e.diagnostic.Source() }
func (e *ParseError) Parsed() string       { return e.diagnostic.Parsed() }

func (e *ParseError) Error() string { return e.diagnostic.Error() }

func (e *ParseError) Unwrap() error { return e.diagnostic }

func (e want) expected(context parseCtx) string {
	switch e {
	case want_None:
		return assert.PanicUnknownCase[string](e)
	case want_NameAtLeast3Bytes:
		return "time abbreviation with at least 3 bytes"
	case want_NameClosingQuote:
		return "closing '>' for quoted time abbreviation"
	case want_HourDigits:
		return "hour digits ([0-9]+)"
	case want_HourInRange:
		if context == parseCtx_StandardOffset || context == parseCtx_DSTOffset {
			return "hour in range [-24,24]"
		}
		return "hour in range [-167,167]"
	case want_TwoMinuteDigits:
		return "two minute digits ([0-9]{2})"
	case want_MinuteInRange:
		return "minute in range [0,59]"
	case want_TwoSecondDigits:
		return "two second digits ([0-9]{2})"
	case want_SecondInRange:
		return "second in range [0,59]"
	case want_DSTRule:
		return "daylight saving time transition rule"
	case want_CommaBeforeRule:
		return "comma before transition rule"
	case want_RuleSeparator:
		return "rule date/time separator"
	case want_JulianDay:
		return "Julian day in range [1,365]"
	case want_DayOfYear:
		return "day-of-year in range [0,365]"
	case want_Month:
		return "transition-rule month in range [1,12]"
	case want_Week:
		return "transition-rule week in range [1,5]"
	case want_Weekday:
		return "transition-rule weekday in range [0,6]"
	default:
		return assert.PanicUnknownCase[string](e)
	}
}

func (c parseCtx) reason() string {
	switch c {
	case parseCtx_StandardName:
		return "standard time abbreviation"
	case parseCtx_StandardOffset:
		return "standard time offset"
	case parseCtx_DSTName:
		return "daylight saving time abbreviation"
	case parseCtx_DSTOffset:
		return "daylight saving time offset"
	case parseCtx_DSTRule:
		return "daylight saving time rule"
	case parseCtx_RuleStartDateTime:
		return "daylight saving time rule start date/time"
	case parseCtx_RuleEndDateTime:
		return "daylight saving time rule end date/time"
	default:
		return assert.PanicUnknownCase[string](c)
	}
}
