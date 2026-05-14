// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package diag

import (
	"iter"

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/core/option"
	"code.kibou.tools/common/iterx"
)

// Report is an example struct which implements Diagnostic.
type Report[C Code] struct {
	severity Severity
	code     option.Option[C]
	// Always non-empty.
	message  string
	snippets []Snippet
	hints    []Hint
}

// NewReport creates a new Report with the given severity and message.
//
// Pre-condition: The message must be non-empty.
//
// Post-condition: The returned pointer is non-nil.
func NewReport[C Code](severity Severity, message string) *Report[C] {
	switch severity {
	case Severity_Error, Severity_Warning, Severity_InternalWarning:
	default:
		assert.PanicUnknownCase[any](severity)
	}
	assert.Precondition(message != "", "diagnostic message should be non-empty")
	return &Report[C]{
		severity: severity,
		code:     option.None[C](),
		message:  message,
		snippets: nil,
		hints:    nil,
	}
}

// Pre-condition: code must not already have been set earlier.
func (d *Report[C]) WithCode(code C) *Report[C] {
	assert.Precondition(d.code.IsNone(), "code should not already be set")
	d.code = option.Some(code)
	return d
}

func (d *Report[C]) WithSnippet(snippet Snippet) *Report[C] {
	d.snippets = append(d.snippets, snippet)
	return d
}

func (d *Report[C]) WithHint(hint Hint) *Report[C] {
	d.hints = append(d.hints, hint)
	return d
}

var _ Diagnostic[Code] = (*Report[Code])(nil)

func (d *Report[C]) Severity() Severity     { return d.severity }
func (d *Report[C]) Code() option.Option[C] { return d.code }
func (d *Report[C]) Message() string        { return d.message }
func (d *Report[C]) Snippets() iter.Seq[Snippet] {
	return iterx.FromSlice(d.snippets)
}
func (d *Report[C]) Hints() iter.Seq[Hint] { return iterx.FromSlice(d.hints) }
