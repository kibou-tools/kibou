// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package errorx

import (
	"strings"

	"code.kibou.tools/common/assert"
)

// Phrase is a short human-readable string used as a fragment of a
// single-line error message.
//
// A valid Phrase must be non-empty and not contain newlines,
// so that it can be substituted into single-line messages
// without disturbing the surrounding layout.
//
// Usage guidelines:
//   - As a parameter type: If a Phrase is passed as a parameter, it
//     is expected to be non-empty. For passing optional values,
//     parameter types should use Option[Phrase].
//   - As a field type: A struct may store a zero-value Phrase to
//     indicate the absence of one (to save on space). Such a field
//     should not be exported. Exported fields of type Phrase can be
//     assumed to be non-empty.
type Phrase struct {
	s string
}

// NewPhrase constructs a new Phrase from s.
//
// Pre-condition: s is non-empty and contains no newlines.
func NewPhrase(s string) Phrase {
	assert.Precondition(s != "", "empty Phrase")
	if strings.IndexByte(s, '\n') >= 0 {
		assert.Precondition(false, "Phrase contains newline")
	}
	return Phrase{s: s}
}

// Get returns the underlying text and whether the Phrase is non-empty.
func (p Phrase) Get() (text string, present bool) {
	return p.s, p.s != ""
}

// IsEmpty reports whether p is the zero (absent) Phrase.
func (p Phrase) IsEmpty() bool {
	return p.s == ""
}
