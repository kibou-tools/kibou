// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package codegen

import "github.com/typesanitizer/happygo/common/assert"

type enum int

const enumKnown enum = 1

//go:noinline
func UseAlways(b bool) {
	assert.Always(b, "always violation: %d", 1)
}

//go:noinline
func UsePrecondition(b bool) {
	assert.Precondition(b, "precondition violation: %d")
}

//go:noinline
func UsePreconditionf(b bool) {
	assert.Preconditionf(b, "precondition violation: %d", 1)
}

//go:noinline
func UseInvariant(b bool) {
	assert.Invariant(b, "invariant violation: %d")
}

//go:noinline
func UseInvariantf(b bool) {
	assert.Invariantf(b, "invariant violation: %d", 1)
}

//go:noinline
func UsePostcondition(b bool) {
	assert.Postcondition(b, "postcondition violation: %d")
}

//go:noinline
func UsePostconditionf(b bool) {
	assert.Postconditionf(b, "postcondition violation: %d", 1)
}

//go:noinline
func UsePanicInvariantViolation() int {
	return assert.PanicInvariantViolation[int]("invariant violation: %d", 1)
}

//go:noinline
func UsePanicUnknownCase() int {
	return assert.PanicUnknownCase[int](enumKnown)
}
