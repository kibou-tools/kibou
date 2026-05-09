// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package deadline

import (
	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/timex"
)

// --- Aliases for exported use ---

type Source = DeadlineSource
type SourceKind = DeadlineSourceKind
type ExceededError = DeadlineExceededError

// --- Package implementation ---

type Deadline struct {
	instant timex.Instant
	source  DeadlineSource
}

func New(instant timex.Instant, source DeadlineSource) Deadline {
	return Deadline{instant, source}
}

func (d Deadline) Instant() timex.Instant {
	return d.instant
}

func (d Deadline) Source() DeadlineSource {
	return d.source
}

type DeadlineExceededError struct {
	deadline Deadline
}

func NewExceededError(deadline Deadline) DeadlineExceededError {
	return DeadlineExceededError{deadline}
}

func (e DeadlineExceededError) Error() string {
	return "cancel: deadline exceeded"
}

func (e DeadlineExceededError) Deadline() Deadline {
	return e.deadline
}

// ReinterpretForChild interprets the current Deadline as a potential
// Deadline for a child token.
func (d Deadline) ReinterpretForChild() Deadline {
	var newSource DeadlineSourceKind
	var newDepth int
	switch source := d.source; source.kind {
	case Source_InitializedWithTimeout:
		newSource = Source_AncestorTimeout
		newDepth = 1
	case Source_InitializedWithAbsoluteDeadline:
		newSource = Source_AncestorAbsoluteDeadline
		newDepth = 1
	case Source_AncestorTimeout, Source_AncestorAbsoluteDeadline:
		newSource = source.kind
		newDepth = source.ancestorLevel + 1
	default:
		assert.PanicUnknownCase[DeadlineSourceKind](source.kind)
	}
	return New(d.instant, NewSource(newSource, newDepth))
}

type DeadlineSourceKind uint8

const (
	// Source_AncestorTimeout indicates that a particular deadline
	Source_AncestorTimeout DeadlineSourceKind = iota + 1
	Source_AncestorAbsoluteDeadline
	Source_InitializedWithTimeout
	Source_InitializedWithAbsoluteDeadline
)

type DeadlineSource struct {
	kind DeadlineSourceKind
	// See NewDeadlineSource for invariants
	ancestorLevel int
}

// Pre-condition:
//   - If kind is Source_InitializedWithTimeout
//     or Source_InitializedWithAbsoluteDeadline,
//     then ancestorLevel == 0
//   - If kind is Source_AncestorTimeout
//     or Source_AncestorAbsoluteDeadline,
//     then ancestorLevel >= 1
func NewSource(kind DeadlineSourceKind, ancestorLevel int) DeadlineSource {
	switch kind {
	case Source_InitializedWithTimeout, Source_InitializedWithAbsoluteDeadline:
		assert.Preconditionf(ancestorLevel == 0, "got ancestorLevel=%d for DeadlineSourceKind %v", ancestorLevel, kind)
	case Source_AncestorTimeout, Source_AncestorAbsoluteDeadline:
		assert.Preconditionf(ancestorLevel >= 1, "got ancestorLevel=%d for DeadlineSourceKind %v", ancestorLevel, kind)
	}
	return DeadlineSource{kind, ancestorLevel}
}
