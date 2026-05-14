// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package diag provides functionality for reporting rich diagnostics
// with customizable formatting.
//
// The primary intended user of this package is command-line tools.
// Applications like daemons or servers may also use this package for
// local development, and convert the data to structured logs when running
// standalone.
package diag

import (
	"iter"

	"code.kibou.tools/common/assert"
	. "code.kibou.tools/common/core/option"
)

// Diagnostic is a structured, renderer-agnostic description of an error.
type Diagnostic[C Code] interface {
	Severity() Severity
	// Code dictates the optional error code for a particular diagnostic.
	//
	// Small libraries will generally not expose a dedicated Code type,
	// in which case this method can return None.
	Code() Option[C]
	// Message indicates the key message of this error.
	//
	// Requirement: The returned string must be non-empty.
	Message() string
	// Snippets returns any source snippets associated with the diagnostic.
	//
	// Repeated calls to this method must return an identical sequence.
	Snippets() iter.Seq[Snippet]
	// Hints returns any associated hints related to the diagnostic.
	//
	// Repeated calls to this method must return an identical sequence.
	Hints() iter.Seq[Hint]
}

// Code is an identifier for a diagnostic meant to be stable
// across SemVer-compatible versions.
type Code interface {
	// Requirement: The returned string must be non-empty.
	String() string
	// SeeAlso returns a diagnostic-code-specific pointer to more information
	// about the error.
	//
	// For example:
	// - In a server, this may be the URL to the docs which describe the error
	//   code.
	// - In a command-line tool, this may be a command invocation like
	//   "mytool explain BUILD001".
	//
	// Renderers own the surrounding prose. For example, [RenderPretty] prepends
	// [RenderPrettyOptions.SeeAlsoPrefix] to this value.
	//
	// Requirement: If the returned value is Some, it must be non-empty.
	//
	SeeAlso() Option[string]
}

// Severity indicates whether a particular diagnostic
// may or may not lead to unsuccessful execution of the program.
type Severity uint8

const (
	// Severity_Error applies to a diagnostic D if the program
	// will not complete executing successfully upon encountering
	// it, regardless of any other diagnostics.
	Severity_Error Severity = iota + 1
	// Severity_Warning applies to diagnostic D if the program's
	// successful execution does not depend on it being issued,
	// but the diagnostic is meant to indicate something anomalous.
	//
	// Examples of reasons to issue warnings:
	//
	// - Very expensive operations (in time or other resource consumption)
	// - Potential bugs in user input
	Severity_Warning
	// Severity_InternalWarning indicates a bug in the program constructing or
	// reporting diagnostics, rather than a problem in user input.
	Severity_InternalWarning
)

// Text returns an English description of the Severity
// value suitable for rendering.
//
// The first letter of the string will be uppercase.
func (s Severity) Text() string {
	switch s {
	case Severity_Error:
		return "Error"
	case Severity_Warning:
		return "Warning"
	case Severity_InternalWarning:
		return "Internal warning"
	default:
		return assert.PanicUnknownCase[string](s)
	}
}

// HintKind tags a [Hint] with the role it plays in a diagnostic.
type HintKind uint8

const (
	// HintKind_Suggestion proposes a fix or alternative ("hint:").
	HintKind_Suggestion HintKind = iota + 1
	// HintKind_Context describes the surrounding situation ("context:").
	HintKind_Context
)

// Text returns an English description of the HintKind
// value, suitable for rendering.
//
// The first letter of the string will be lowercase.
func (k HintKind) Text() string {
	switch k {
	case HintKind_Suggestion:
		return "hint"
	case HintKind_Context:
		return "context"
	default:
		return assert.PanicUnknownCase[string](k)
	}
}

// Hint is a single annotation attached to a diagnostic. It carries a kind
// (context vs suggestion) and a non-empty message.
type Hint struct {
	kind HintKind
	msg  string
}

// NewHint constructs a Hint.
//
// Pre-conditions:
//
// 1. kind is one of the defined [HintKind] constants
// 2. msg must be non-empty.
func NewHint(kind HintKind, msg string) Hint {
	switch kind {
	case HintKind_Suggestion, HintKind_Context:
	default:
		assert.PanicUnknownCase[any](kind)
	}
	assert.Precondition(msg != "", "hint message should be non-empty")
	return Hint{kind: kind, msg: msg}
}

// Kind returns the [HintKind].
func (h Hint) Kind() HintKind { return h.kind }

// Msg returns the non-empty hint message.
func (h Hint) Msg() string { return h.msg }
