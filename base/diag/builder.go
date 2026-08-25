// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package diag

import (
	"iter"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/iterx"
)

// Report is a struct which implements the Diagnostic
// interface using pre-computed data.
type Report struct {
	severity    Severity
	attribution Attribution
	message     string
	snippets    []Snippet
	notes       []Note
}

// NewReport creates a new Report.
//
// Precondition: message must be non-empty.
//
// Postcondition: The returned pointer is non-nil.
func NewReport(severity Severity, attribution Attribution, message string) *Report {
	switch severity {
	case Severity_Error, Severity_Warning:
	default:
		assert.PanicUnknownCase[any](severity)
	}
	switch attribution {
	case Attribution_Internal, Attribution_External:
	default:
		assert.PanicUnknownCase[any](attribution)
	}
	assert.Precondition(message != "", "diagnostic message should be non-empty")
	return &Report{
		severity:    severity,
		attribution: attribution,
		message:     message,
		snippets:    nil,
		notes:       nil,
	}
}

// WithSnippet attaches a Snippet to the receiver.
//
// The receiver is returned for chaining.
func (d *Report) WithSnippet(snippet Snippet) *Report {
	d.snippets = append(d.snippets, snippet)
	return d
}

// WithNote attaches a Note to the receiver.
//
// The receiver is returned for chaining.
func (d *Report) WithNote(note Note) *Report {
	d.notes = append(d.notes, note)
	return d
}

var _ Diagnostic = (*Report)(nil)

func (d *Report) Severity() Severity       { return d.severity }
func (d *Report) Attribution() Attribution { return d.attribution }
func (d *Report) Message() string          { return d.message }
func (d *Report) Snippets() iter.Seq[Snippet] {
	return iterx.FromSlice(d.snippets)
}
func (d *Report) Notes() iter.Seq[Note] { return iterx.FromSlice(d.notes) }

// CodedReport is a Report paired with a Code.
type CodedReport[C Code] struct {
	report *Report
	code   C
}

// NewCodedReport constructs a new CodedReport.
//
// Precondition: message is non-empty.
//
// Postcondition: The returned pointer is non-nil.
func NewCodedReport[C Code](severity Severity, attribution Attribution, message string, code C) *CodedReport[C] {
	return &CodedReport[C]{report: NewReport(severity, attribution, message), code: code}
}

// WithSnippet attaches a Snippet to the receiver.
//
// The receiver is returned for chaining.
func (d *CodedReport[C]) WithSnippet(snippet Snippet) *CodedReport[C] {
	d.report.WithSnippet(snippet)
	return d
}

// WithNote attaches a Note to the receiver.
//
// The receiver is returned for chaining.
func (d *CodedReport[C]) WithNote(note Note) *CodedReport[C] {
	d.report.WithNote(note)
	return d
}

var _ CodedDiagnostic[Code] = (*CodedReport[Code])(nil)

func (d *CodedReport[C]) Severity() Severity       { return d.report.Severity() }
func (d *CodedReport[C]) Attribution() Attribution { return d.report.Attribution() }
func (d *CodedReport[C]) Code() C                  { return d.code }
func (d *CodedReport[C]) Message() string          { return d.report.Message() }
func (d *CodedReport[C]) Snippets() iter.Seq[Snippet] {
	return d.report.Snippets()
}
func (d *CodedReport[C]) Notes() iter.Seq[Note] { return d.report.Notes() }
