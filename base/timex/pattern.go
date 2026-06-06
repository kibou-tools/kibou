// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package timex

import (
	"strconv"
	"strings"
)

// Pattern describes the textual representation of a time-like type.
//
// It can be used for formatting.
type Pattern struct {
	specs []spec
}

// NewPattern returns an empty formatting pattern.
func NewPattern() Pattern {
	return Pattern{specs: nil}
}

// Fixed appends literal text to p.
//
// The text is never interpreted as a stdlib time layout fragment; for example,
// Fixed("2006") formats as the literal string "2006".
func (p Pattern) Fixed(text string) Pattern {
	return p.append(spec{kind: patternSpecKind_Fixed, text: text})
}

// Year appends the astronomical year number with at least four digits.
//
// Year 0 is 1 BCE and formats as "0000"; the leading minus sign starts with
// year -1, which is 2 BCE.
func (p Pattern) Year() Pattern {
	return p.append(spec{kind: patternSpecKind_Year, text: ""})
}

// Month appends the month of the year as two digits, from "01" through "12".
func (p Pattern) Month() Pattern {
	return p.append(spec{kind: patternSpecKind_Month, text: ""})
}

// Day appends the day of the month as two digits, from "01" through "31".
func (p Pattern) Day() Pattern {
	return p.append(spec{kind: patternSpecKind_Day, text: ""})
}

type spec struct {
	kind patternSpecKind
	// text is set only when kind is patternSpecKind_Fixed.
	text string
}

type patternSpecKind uint8

const (
	patternSpecKind_Fixed patternSpecKind = iota + 1
	patternSpecKind_Year
	patternSpecKind_Month
	patternSpecKind_Day
)

func (p Pattern) append(spec spec) Pattern {
	p.specs = append(p.specs, spec)
	return p
}

func writePaddedInt(b *strings.Builder, value int, width int) {
	if value < 0 {
		b.WriteByte('-')
		value = -value
	}
	digits := strconv.Itoa(value)
	for i := len(digits); i < width; i++ {
		b.WriteByte('0')
	}
	b.WriteString(digits)
}
