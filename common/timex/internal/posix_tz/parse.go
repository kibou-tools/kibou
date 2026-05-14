// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package posix_tz

import (
	"unicode/utf8"

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/core/result"
	"code.kibou.tools/common/internal/ranges"
	. "code.kibou.tools/common/zero"
)

// Parse parses the input string based on the rules specified
// in POSIX.1-2024 Ch. 8.
//
// For docs, see code://docs/external/posix_tz.md
func Parse(source string) result.Result[TimeZone] {
	p := parser{s: source, i: 0}
	if p.done() {
		return p.err(ParseErrorKind_Empty)
	}
	if !utf8.ValidString(source) {
		return p.err(ParseErrorKind_InvalidUTF8)
	}

	stdName, err := p.name()
	if err != want_None {
		return p.specErr(err, parseCtx_StandardName)
	}
	stdRawOffset, err := p.utcOffset()
	if err != want_None {
		return p.specErr(err, parseCtx_StandardOffset)
	}
	// Per code://docs/external/posix_tz.md#offset-format, the offset
	// is "the value added to the local time to arrive at [UTC]",
	// which is the opposite of the usual convention, so we need
	// an extra - sign.
	stdOffset := -stdRawOffset

	var (
		hasDST    bool
		dstName   string
		dstOffset int32
		rule      Rule
	)
	if !p.done() {
		hasDST = true
		dstName, err = p.name()
		if err != want_None {
			return p.specErr(err, parseCtx_DSTName)
		}
		if p.done() || p.peek() == ',' {
			// Per code://docs/external/posix_tz.md#daylight-saving-time-default-offset
			// If the offset is not specified, it should default to +1 hour.
			dstOffset = stdOffset + int32(secondsPerHour)
		} else {
			dstRawOffset, err := p.utcOffset()
			if err != want_None {
				return p.specErr(err, parseCtx_DSTOffset)
			}
			// Extra - sign for same reason as stdRawOffset earlier.
			dstOffset = -dstRawOffset
		}
		if p.done() {
			return p.specErr(want_DSTRule, parseCtx_DSTRule)
		}
		if !p.tryConsume(',') {
			return p.specErr(want_CommaBeforeRule, parseCtx_DSTRule)
		}
		parsedRule, err, context := p.rule()
		if err != want_None {
			return p.specErr(err, context)
		}
		if !p.done() {
			return p.err(ParseErrorKind_TrailingData)
		}
		rule = parsedRule
	}

	return result.Success(TimeZone{
		StdName:   stdName,
		StdOffset: stdOffset,
		HasDST:    hasDST,
		dstName:   dstName,
		dstOffset: dstOffset,
		rule:      rule,
	})
}

type parser struct {
	s string
	i int
}

func (p *parser) done() bool { return p.i == len(p.s) }
func (p *parser) peek() byte { return p.s[p.i] }
func (p *parser) err(kind ParseErrorKind) result.Result[TimeZone] {
	return result.Failure[TimeZone](newParseError(kind, p.s, p.s[:p.i]))
}

func (p *parser) specErr(err want, context parseCtx) result.Result[TimeZone] {
	return result.Failure[TimeZone](newSpecViolationParseError(specViolation{err, context}, p.s, p.s[:p.i]))
}

// tryConsume returns true if the next byte matches b,
// after consuming it. If there is no next byte, or if it
// doesn't match b, it returns false without consuming anything.
func (p *parser) tryConsume(b byte) bool {
	if p.done() || p.peek() != b {
		return false
	}
	p.i++
	return true
}

func (p *parser) consumeWhile(subset charSubset) string {
	start := p.i
	switch subset {
	case charSubset_UnquotedAbbreviation:
		for !p.done() && isUnquotedPOSIXAbbreviationByte(p.peek()) {
			p.i++
		}
	case charSubset_QuotedAbbreviation:
		for !p.done() && isQuotedPOSIXAbbreviationByte(p.peek()) {
			p.i++
		}
	default:
		assert.PanicUnknownCase[any](subset)
	}
	return p.s[start:p.i]
}

// Parses the name based on code://docs/external/posix_tz.md#name-format
func (p *parser) name() (string, want) {
	if p.tryConsume('<') {
		name := p.consumeWhile(charSubset_QuotedAbbreviation)
		if len(name) < 3 {
			return "", want_NameAtLeast3Bytes
		}
		if !p.tryConsume('>') {
			return "", want_NameClosingQuote
		}
		return name, want_None
	}
	name := p.consumeWhile(charSubset_UnquotedAbbreviation)
	if len(name) < 3 {
		return "", want_NameAtLeast3Bytes
	}
	return name, want_None
}

// Post-condition: if err == want_None, the returned value is in
// [-24 * 60 * 60, 24 * 60 * 60].
func (p *parser) utcOffset() (int32, want) {
	// POSIX std/dst offset fields accept an optional sign and hours
	// whose absolute value is in the range 0 through 24.
	return p.signedHMS(-24, 24, 2)
}

// Post-condition: if err == want_None, the returned value is in
// [- (7 * 24 * 60 * 60) + 1, (7 * 24 * 60 * 60) - 1], i.e.
// [-604799,604799]
func (p *parser) transitionTime() (int32, want) {
	// IANA TZif v3+ extends rule transition times to the range
	// -167:59:59 through 167:59:59.
	//
	// So the hour value itself is in [-167, +167].
	return p.signedHMS(-167, 167, 3)
}

// signedHMS parses a string of the form:
//
// [+|-]hhh[:mm[:ss]]
//
// where the :mm and :ss suffixes are optional.
//
// mm and ss values must both be exactly 2 digits
// in the range [00, 59].
func (p *parser) signedHMS(minHour int32, maxHour int32, maxHourDigits uint8) (int32, want) {
	// Tracking the sign separately from the hour magnitude preserves a
	// leading '-' even when all the digit fields are zero (e.g., "-0:00:01"
	// for −1 second).
	sign, hour, ok := p.parseInt32(int32Prefix_MayHaveSign, maxHourDigits)
	if !ok {
		return 0, want_HourDigits
	}
	if sign*hour < minHour || sign*hour > maxHour {
		return 0, want_HourInRange
	}
	offset := hour * secondsPerHour
	if p.tryConsume(':') {
		minuteStart := p.i
		_, minute, ok := p.parseInt32(int32Prefix_NoSign, 2)
		if !ok || p.i-minuteStart != 2 {
			return 0, want_TwoMinuteDigits
		}
		if minute > 59 {
			return 0, want_MinuteInRange
		}
		offset += minute * secondsPerMinute
		if p.tryConsume(':') {
			secondStart := p.i
			_, second, ok := p.parseInt32(int32Prefix_NoSign, 2)
			if !ok || p.i-secondStart != 2 {
				return 0, want_TwoSecondDigits
			}
			if second > 59 {
				return 0, want_SecondInRange
			}
			offset += second
		}
	}
	return sign * offset, want_None
}

func (p *parser) rule() (Rule, want, parseCtx) {
	start, err := p.ruleDateTime()
	if err != want_None {
		return Zero[Rule](), err, parseCtx_RuleStartDateTime
	}
	if !p.tryConsume(',') {
		return Zero[Rule](), want_RuleSeparator, parseCtx_RuleEndDateTime
	}
	end, err := p.ruleDateTime()
	if err != want_None {
		return Zero[Rule](), err, parseCtx_RuleEndDateTime
	}
	return Rule{Start: start, End: end}, want_None, 0
}

func (p *parser) ruleDateTime() (RuleDateTime, want) {
	var (
		kind      RuleDateTimeKind
		dayBits   uint16
		weekMonth WeekMonth
	)
	switch {
	case p.tryConsume('J'):
		_, day, ok := p.parseInt32(int32Prefix_NoSign, 3)
		if !ok {
			return Zero[RuleDateTime](), want_JulianDay
		}
		// maxDigits = 3 => day ∈ [0, 999] => int32->int16 narrowing is OK
		julianDay, ok := NewJulianDay(int16(day))
		if !ok {
			return Zero[RuleDateTime](), want_JulianDay
		}
		kind = RuleDateTimeKind_Julian
		dayBits = uint16(julianDay.Int16()) // OK as value is in [1, 365]
	case p.tryConsume('M'):
		month, ok := p.month()
		if !ok {
			return Zero[RuleDateTime](), want_Month
		}
		if !p.tryConsume('.') {
			return Zero[RuleDateTime](), want_RuleSeparator
		}
		week, ok := p.week()
		if !ok {
			return Zero[RuleDateTime](), want_Week
		}
		if !p.tryConsume('.') {
			return Zero[RuleDateTime](), want_RuleSeparator
		}
		weekday, ok := p.weekday()
		if !ok {
			return Zero[RuleDateTime](), want_Weekday
		}
		kind = RuleDateTimeKind_MonthWeekDay
		weekMonth = NewWeekMonth(month, week)
		dayBits = uint16(weekday.Uint8())
	default:
		_, day, ok := p.parseInt32(int32Prefix_NoSign, 3)
		if !ok {
			return Zero[RuleDateTime](), want_DayOfYear
		}
		// maxDigits = 3 => day ∈ [0, 999] => int32->int16 narrowing is OK
		dayOfYear, ok := NewDayOfYear(int16(day))
		if !ok {
			return Zero[RuleDateTime](), want_DayOfYear
		}
		kind = RuleDateTimeKind_DayOfYear
		dayBits = uint16(dayOfYear.Int16())
	}
	second := int32(2 * secondsPerHour)
	if p.tryConsume('/') {
		s, err := p.transitionTime()
		if err != want_None {
			return Zero[RuleDateTime](), err
		}
		// transitionTime ∈ [-604799,604799], so it can't fit into
		// 16 bits. So let's use 32 bits for simplicity.
		second = s
	}
	return RuleDateTime{
		kind:      kind,
		day:       dayBits,
		second:    second,
		weekMonth: weekMonth,
	}, want_None
}

type charSubset uint8

const (
	charSubset_UnquotedAbbreviation charSubset = iota + 1
	charSubset_QuotedAbbreviation
)

func isUnquotedPOSIXAbbreviationByte(b byte) bool {
	// Per code://docs/external/posix_tz.md#unquoted-form,
	// only alphabetic characters are allowed.
	return 'A' <= b && b <= 'Z' || 'a' <= b && b <= 'z'
}

func isQuotedPOSIXAbbreviationByte(b byte) bool {
	// Per code://docs/external/posix_tz.md#quoted-form,
	// alphanumeric characters and '+' and '-' are allowed.
	return 'A' <= b && b <= 'Z' || 'a' <= b && b <= 'z' || '0' <= b && b <= '9' || b == '+' || b == '-'
}

func (p *parser) month() (Month, bool) {
	_, month, ok := p.parseInt32(int32Prefix_NoSign, 2)
	if !ok {
		return Zero[Month](), false
	}
	// maxDigits = 2 => month ∈ [0, 99] => int32->uint8 narrowing is OK
	return NewMonth(uint8(month))
}

func (p *parser) week() (Week, bool) {
	_, week, ok := p.parseInt32(int32Prefix_NoSign, 1)
	if !ok {
		return Zero[Week](), false
	}
	// maxDigits = 1 => week ∈ [0, 9] => int32->uint8 narrowing is OK
	return NewWeek(uint8(week))
}

func (p *parser) weekday() (Weekday, bool) {
	_, weekday, ok := p.parseInt32(int32Prefix_NoSign, 1)
	if !ok {
		return Zero[Weekday](), false
	}
	// maxDigits = 1 => weekday ∈ [0, 9] => int32->uint8 narrowing is OK
	return NewWeekday(uint8(weekday))
}

type int32Prefix uint8

const (
	int32Prefix_MayHaveSign int32Prefix = iota + 1
	int32Prefix_NoSign
)

// maxSafeInt32Digits is one less than the number of decimal digits in MaxInt32.
// This guarantees n*10+digit cannot overflow int32 for any allowed input.
const maxSafeInt32Digits = 9

// parseInt32 parses a decimal int32 with at most maxDigits digits, returning
// the sign and the (non-negative) magnitude separately so callers can
// distinguish a parsed "-0" from "+0".
//
// When prefix is int32Prefix_NoSign, sign is always +1.
//
// Pre-condition: maxDigits ∈ (0, maxSafeInt32Digits].
func (p *parser) parseInt32(prefix int32Prefix, maxDigits uint8) (sign int32, value int32, ok bool) {
	if maxDigits == 0 {
		assert.Preconditionf(false, "maxDigits must be positive, got %d", maxDigits)
	}
	if maxDigits > maxSafeInt32Digits {
		assert.Preconditionf(false, "maxDigits %d may overflow int32", maxDigits)
	}
	asciiNum := ranges.ClosedRange[byte]{Lo: '0', Hi: '9'}

	sign = 1
	switch prefix {
	case int32Prefix_MayHaveSign:
		if p.tryConsume('-') {
			sign = -1
		} else {
			p.tryConsume('+')
		}
	case int32Prefix_NoSign:
		break
	default:
		return assert.PanicUnknownCase[int32](prefix), 0, false
	}
	if p.done() || !asciiNum.Contains(p.peek()) {
		return 0, 0, false
	}
	n := int32(0)
	digits := uint8(0)
	for !p.done() && digits < maxDigits && asciiNum.Contains(p.peek()) {
		n = n*10 + int32(p.peek()-'0')
		p.i++
		digits++
	}
	return sign, n, true
}
