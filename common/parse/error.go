// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package parse provides common error types for parsers.
package parse

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Error reports a parse failure with source/progress information and optional
// context about what the parser expected at the failure point.
type Error struct {
	subject string
	source  string
	parsed  string

	context  string
	expected string
	reason   string
}

type ErrorOptions struct {
	// Parsed is the prefix of Source that was parsed before the error was hit.
	// It may be empty if the parser failed at the start of Source.
	Parsed string
	// Context describes what the parser was trying to parse when it failed. It
	// may be empty if there is no more specific context.
	Context string
	// Expected describes what the parser expected at the failure point. It may be
	// empty if Reason is used instead.
	Expected string
	// Reason describes the failure directly. It may be empty when Expected is set.
	Reason string
}

func NewError(subject string, source string, options ErrorOptions) *Error {
	return &Error{
		subject:  subject,
		source:   source,
		parsed:   options.Parsed,
		context:  options.Context,
		expected: options.Expected,
		reason:   options.Reason,
	}
}

func (e *Error) Subject() string {
	return e.subject
}

func (e *Error) Source() string {
	return e.source
}

func (e *Error) Parsed() string {
	return e.parsed
}

func (e *Error) Context() string {
	return e.context
}

func (e *Error) Expected() string {
	return e.expected
}

func (e *Error) Reason() string {
	return e.reason
}

func (e *Error) Error() string {
	// TODO: Migrate this logic to instead use a structured formatter
	// so that we get JSON + multiline formatting + coloring for "free".
	var b strings.Builder
	fmt.Fprintf(&b, "failed to parse %s %q", e.subject, e.source)
	if e.reason != "" {
		fmt.Fprintf(&b, ": %s", e.reason)
		return b.String()
	}
	if e.parsed != "" || e.context != "" || e.expected != "" {
		fmt.Fprintf(&b, ": hit error after parsing %q", e.parsed)
	}
	if e.context != "" {
		fmt.Fprintf(&b, ": context - trying to parse %s", e.context)
	}
	if e.expected != "" {
		fmt.Fprintf(&b, ", expected - %s, next char - %s", e.expected, e.nextChar())
	}
	return b.String()
}

func (e *Error) nextChar() string {
	if len(e.parsed) >= len(e.source) {
		return "EOF"
	}
	r, size := utf8.DecodeRuneInString(e.source[len(e.parsed):])
	if r == utf8.RuneError && size == 1 {
		return fmt.Sprintf("byte 0x%02x", e.source[len(e.parsed)])
	}
	return fmt.Sprintf("%q", r)
}
