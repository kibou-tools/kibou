// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package diag

import "strings"

// Style styles a logical text fragment based on its [Role].
//
// Renderers call Wrap before writing each labeled fragment.
type Style interface {
	// Requirement: Wrap must not modify the visual width of the associated
	// text. For example, adding ANSI escape codes for foreground or
	// background color is fine.
	Wrap(r Role, s string) string
	// TODO: Later, we want to introduce some abstraction for styled text
	// so that we don't have the in-band signaling that this API shape has.
}

// Role labels a logical fragment of diagnostic output so that a [Style] can
// decide how to render it.
type Role uint8

const (
	// Role_SeverityError is the "error:" label.
	Role_SeverityError Role = iota + 1
	// Role_SeverityWarning is the "warning:" label.
	Role_SeverityWarning
	// Role_HintContext is the "context:" label.
	Role_HintContext
	// Role_HintSuggestion is the "hint:" label.
	Role_HintSuggestion
	// Role_Caret is the "^" pointer beneath a source line.
	Role_Caret
	// Role_Frame is a box-drawing character bracketing the diagnostic.
	Role_Frame
)

// PlainStyle applies no styling to every role. Suitable for
// usage in tests.
var PlainStyle Style = plainStyle{}

type plainStyle struct{}

func (plainStyle) Wrap(_ Role, s string) string { return s }

// ANSIStyle wraps text fragments with ANSI escape codes.
//
// The current choice of styles follows jj's default heading colors for the
// roles we support:
//
// - Bold red for errors
// - Bold yellow for warnings
// - Bold cyan for context and suggestions
//
// Fragments for other roles are left as-is.
var ANSIStyle Style = ansiStyle{}

type ansiStyle struct{}

// See list of colors here: https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
const (
	ansiReset      = "\x1b[0m"
	ansiBoldRed    = "\x1b[1;31m"
	ansiBoldYellow = "\x1b[1;33m"
	ansiBoldCyan   = "\x1b[1;36m"
)

func (ansiStyle) Wrap(r Role, s string) string {
	var b strings.Builder
	var style string
	switch r {
	case Role_SeverityError:
		style = ansiBoldRed
	case Role_SeverityWarning:
		style = ansiBoldYellow
	case Role_HintContext, Role_HintSuggestion:
		style = ansiBoldCyan
	case Role_Caret, Role_Frame:
		return s
	default:
		return s
	}
	b.Grow(len(style) + len(s) + len(ansiReset))
	b.WriteString(style)
	b.WriteString(s)
	b.WriteString(ansiReset)
	return b.String()
}
