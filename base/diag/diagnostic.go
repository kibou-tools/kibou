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

	"code.kibou.tools/base/assert"
	. "code.kibou.tools/base/core/option"
)

// Diagnostic is a structured, renderer-agnostic description of an error.
type Diagnostic interface {
	// Severity describes whether an operation can/cannot be
	// successfully completed.
	Severity() Severity
	// Attribution indicates which error domain is responsible for having
	// caused this error.
	//
	// In the context of a generic library:
	//
	// - If the presence of this diagnostic indicates a bug or something
	//   suboptimal in the library, then the return value should be
	//   Attribution_Internal.
	// - If the presence of this diagnostic indicates something wrong or
	//   suboptimal with the caller, the environment (e.g. filesystem
	//   or network operations) or something else external to the library
	//   (such as some problem in a dependency), then the return value
	//   should be Attribution_External.
	//
	// In the context of an application:
	//
	// -
	Attribution() Attribution
	// Message indicates the key message of this error.
	//
	// Requirement: The returned string must be non-empty.
	Message() string
	// Snippets returns any source snippets associated with the diagnostic.
	//
	// Repeated calls to this method must return an identical sequence.
	Snippets() iter.Seq[Snippet]
	// Notes returns any associated notes related to the diagnostic.
	//
	// Repeated calls to this method must return an identical sequence.
	Notes() iter.Seq[Note]
}

type CodedDiagnostic[C Code] interface {
	Diagnostic
	Code() C
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
	default:
		return assert.PanicUnknownCase[string](s)
	}
}

// Attribution coarsely describes "who" is responsible for
// an error.
//
// For more details, see [Diagnostic.Attribution] and the
// design docs.
type Attribution uint8

const (
	// Attribution_Internal marks an error/warning as being caused by
	// the error domain it originated in.
	Attribution_Internal Attribution = iota + 1
	// Attribution_External marks a diagnostic as being caused by
	// something outside the error domain it originated in.
	Attribution_External
)

func (a Attribution) String() string {
	switch a {
	case Attribution_Internal:
		return "internal"
	case Attribution_External:
		return "external"
	default:
		return assert.PanicUnknownCase[string](a)
	}
}

func (a Attribution) Text() string {
	switch a {
	case Attribution_Internal:
		return "Internal"
	case Attribution_External:
		return ""
	default:
		return assert.PanicUnknownCase[string](a)
	}
}

// NoteKind tags a [Note] with the role it plays in a diagnostic.
type NoteKind uint8

const (
	// NoteKind_Suggestion proposes a fix or alternative ("hint:").
	NoteKind_Suggestion NoteKind = iota + 1
	// NoteKind_Context describes the surrounding situation ("note:").
	NoteKind_Context
)

// Text returns an English description of the NoteKind
// value, suitable for rendering.
//
// The first letter of the string will be lowercase.
func (k NoteKind) Text() string {
	switch k {
	case NoteKind_Suggestion:
		return "hint"
	case NoteKind_Context:
		return "note"
	default:
		return assert.PanicUnknownCase[string](k)
	}
}

// Note is a single annotation attached to a diagnostic. It carries a kind
// (context vs suggestion) and a non-empty message.
type Note struct {
	kind NoteKind
	msg  string
}

// NewNote constructs a Note.
//
// Pre-conditions:
//
// 1. kind is one of the defined [NoteKind] constants
// 2. msg must be non-empty.
func NewNote(kind NoteKind, msg string) Note {
	switch kind {
	case NoteKind_Suggestion, NoteKind_Context:
	default:
		assert.PanicUnknownCase[any](kind)
	}
	if msg == "" {
		assert.Precondition(false, "diag.NewNote: msg should be non-empty")
	}
	return Note{kind: kind, msg: msg}
}

// Kind returns the [NoteKind].
func (n Note) Kind() NoteKind { return n.kind }

// Msg returns the non-empty note message.
func (n Note) Msg() string { return n.msg }
