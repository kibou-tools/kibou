// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package core

import "code.kibou.tools/base/core/pair"

// Pair holds two values of (potentially different) types.
type Pair[A, B any] = pair.Pair[A, B]

// NewPair constructs a Pair from its two components.
func NewPair[A, B any](first A, second B) Pair[A, B] {
	return pair.NewPair(first, second)
}
