// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package posix_tz

import (
	"strconv"
	"strings"

	"code.kibou.tools/common/assert"
)

// Format returns the canonical POSIX TZ string for tz.
//
// Parse(tz.Format()).Get() is guaranteed to return a TimeZone equal to tz.
// The canonical form:
//   - quotes a name only when it contains characters outside [A-Za-z];
//   - omits the explicit DST offset when it equals StdOffset + 1h, matching
//     the POSIX default;
//   - omits the "/time" suffix on a rule when it equals 02:00, the default.
func (tz TimeZone) Format() string {
	var b strings.Builder
	writeName(&b, tz.StdName)
	// Per code://docs/external/posix_tz.md#offset-format, the on-wire offset
	// is the value added to local time to arrive at UTC — the opposite of
	// "seconds east of UTC", hence the negation.
	writeOffset(&b, -tz.StdOffset)
	if !tz.HasDST {
		return b.String()
	}
	writeName(&b, tz.dstName)
	if tz.dstOffset != tz.StdOffset+secondsPerHour {
		writeOffset(&b, -tz.dstOffset)
	}
	b.WriteByte(',')
	writeRuleDateTime(&b, tz.rule.Start)
	b.WriteByte(',')
	writeRuleDateTime(&b, tz.rule.End)
	return b.String()
}

func writeName(b *strings.Builder, name string) {
	if nameNeedsQuoting(name) {
		b.WriteByte('<')
		b.WriteString(name)
		b.WriteByte('>')
		return
	}
	b.WriteString(name)
}

func nameNeedsQuoting(name string) bool {
	for i := 0; i < len(name); i++ {
		if !isUnquotedPOSIXAbbreviationByte(name[i]) {
			return true
		}
	}
	return false
}

// writeOffset writes a signed `[-+]?h[h[h]][:mm[:ss]]` value.
//
// The hour component is printed without leading zeros; minutes and seconds
// are zero-padded to two digits and elided when both are zero.
func writeOffset(b *strings.Builder, offset int32) {
	if offset < 0 {
		b.WriteByte('-')
		offset = -offset
	}
	h := offset / secondsPerHour
	rem := offset % secondsPerHour
	m := rem / secondsPerMinute
	s := rem % secondsPerMinute
	b.WriteString(strconv.FormatInt(int64(h), 10))
	if m == 0 && s == 0 {
		return
	}
	b.WriteByte(':')
	writeTwoDigit(b, m)
	if s == 0 {
		return
	}
	b.WriteByte(':')
	writeTwoDigit(b, s)
}

func writeTwoDigit(b *strings.Builder, n int32) {
	b.WriteByte(byte('0' + n/10))
	b.WriteByte(byte('0' + n%10))
}

func writeRuleDateTime(b *strings.Builder, dt RuleDateTime) {
	switch dt.Kind() {
	case RuleDateTimeKind_Julian:
		b.WriteByte('J')
		b.WriteString(strconv.FormatInt(int64(dt.JulianDay().Int()), 10))
	case RuleDateTimeKind_DayOfYear:
		b.WriteString(strconv.FormatInt(int64(dt.DayOfYear().Int()), 10))
	case RuleDateTimeKind_MonthWeekDay:
		b.WriteByte('M')
		b.WriteString(strconv.FormatInt(int64(dt.Month().Int()), 10))
		b.WriteByte('.')
		b.WriteString(strconv.FormatInt(int64(dt.Week().Int()), 10))
		b.WriteByte('.')
		b.WriteString(strconv.FormatInt(int64(dt.Weekday().Int()), 10))
	default:
		assert.PanicUnknownCase[any](dt.Kind())
	}
	if dt.Second() != 2*secondsPerHour {
		b.WriteByte('/')
		writeOffset(b, dt.Second())
	}
}
