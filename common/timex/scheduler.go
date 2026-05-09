// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package timex

// Scheduler represents a way of scheduling some work at some given
// point of time in the future.
//
// This interface does not make any guarantees of parallelism.
type Scheduler interface {
	MonotonicClock

	// RunAfter schedules f to run after d according to this
	// scheduler's clock.
	//
	// Production schedulers typically run f in its own goroutine.
	// Test schedulers may instead queue f for deterministic execution.
	//
	// If d is sufficiently small, the scheduler may perform the work
	// immediately, instead of queuing it for later.
	//
	// d == 0 is allowed as a hint to the scheduler to run f
	// immediately. The scheduler may choose to ignore this hint.
	//
	// Pre-conditions:
	// - d >= 0
	// - f is non-nil
	// - IMPORTANT: f must not panic. If it does, there are no guarantees
	//   about how the panic will be propagated.
	RunAfter(d Duration, f func()) ScheduledFunc
}

// ScheduledFunc is a handle for a function scheduled through [Scheduler].
type ScheduledFunc interface {
	// Stop prevents the scheduled function from running if it has not started.
	//
	// Stop does not wait for the function if it has already started.
	Stop() StopResult
}

type StopResult uint8

const (
	// StopResult_Stopped means Stop prevented the scheduled function from
	// starting. The function will not be called because of this schedule.
	StopResult_Stopped StopResult = iota + 1

	// StopResult_TooLate means Stop did not prevent the scheduled function from
	// starting. The function may be running, may have already completed, or the
	// schedule may have already been stopped by an earlier Stop call.
	StopResult_TooLate
)
