// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package timex

type TimestampClock interface {
	GetTimestamp() Timestamp
}

type MonotonicClock interface {
	// GetInstant reads the value for 'now' from this monotonic
	// clock.
	//
	// Requirements: The value returned must be weakly monotonic
	// increasing. In other words, if a happens-before relationship
	// is established between two events A and B according to the
	// Go memory model, then the corresponding instants IA and IB
	// must satisfy IA <= IB.
	GetInstant() Instant
}
