// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package benchmark

import "runtime"

// BlackHole acts as an optimization barrier for benchmark results.
//
//go:noinline
func BlackHole[T any](v T) {
	runtime.KeepAlive(v)
}
