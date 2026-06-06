// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package timex

import (
	"strings"
	stdlib_time "time"

	"code.kibou.tools/base/assert"
)

type Timestamp struct {
	value stdlib_time.Time
}

func NewTimestamp(value stdlib_time.Time) Timestamp {
	assert.Preconditionf(value.Location() == stdlib_time.UTC,
		"Timestamp must be UTC; got location %q", value.Location().String())
	return Timestamp{value: value}
}

func (t Timestamp) Format(pat Pattern) string {
	var b strings.Builder
	for _, spec := range pat.specs {
		switch spec.kind {
		case patternSpecKind_Fixed:
			b.WriteString(spec.text)
		case patternSpecKind_Year:
			writePaddedInt(&b, t.value.Year(), 4)
		case patternSpecKind_Month:
			writePaddedInt(&b, int(t.value.Month()), 2)
		case patternSpecKind_Day:
			writePaddedInt(&b, t.value.Day(), 2)
		default:
			return assert.PanicUnknownCase[string](spec.kind)
		}
	}
	return b.String()
}
