// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cancel

import (
	"context"
	stdlib_time "time" //nolint:depguard // cancel is the designated timer wrapper for ClockToken

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/cancel/deadline"
	"code.kibou.tools/common/core/option"
	"code.kibou.tools/common/timex"
	. "code.kibou.tools/common/unit"
)

// --- Aliases for external use ---

type Option = CancelOption

// --- Package implementation ---

// ClockToken is a token which will be automatically canceled
// when the time crosses Deadline().
//
// You can think of a ClockToken as a triplet of:
//
//  1. A Token which can have children/propagates cancellation
//     unidirectionally to them.
//  2. A monotonic clock.
//  3. A Deadline value, fixed at the time of creation.
type ClockToken interface {
	Token
	timex.MonotonicClock

	// Deadline returns Some(deadline) iff this token has an associated deadline.
	//
	// See the methods on [cancel.Deadline] for more information.
	//
	// Requirement: Implementations must return equal values across
	// invocations.
	Deadline() option.Option[Deadline]

	// NewClockChild returns a ChildClockToken that is automatically
	// canceled if the receiver is canceled.
	// Cancellation flow is unidirectional: canceling the child does
	// not affect the parent.
	//
	// The cancellation for the ChildClockToken can be controlled by:
	//
	//   1. Calling its [ChildToken.Cancel] method.
	//   2. By customizing the options passed during construction.
	NewClockChild(options ...CancelOption) ChildClockToken
}

type ChildClockToken interface {
	ClockToken
	ChildToken
}

// CancelOption represents a way of cancelling a ClockToken.
//
// The zero value of this type is invalid.
// Use Timeout or Deadline for initialization.
type CancelOption struct {
	// Exactly one of timeout or deadline must be Some.
	timeout  option.Option[timex.Duration]
	deadline option.Option[timex.Instant]
}

// OnTimeout create a new CancelOption which indicates
// cancellation after duration d from the creation of the
// corresponding ClockToken.
//
// Pre-condition: d >= 0.
func OnTimeout(d timex.Duration) CancelOption {
	assert.Preconditionf(d >= 0, "duration must be >= 0, but got %v", d)
	return CancelOption{timeout: option.Some(d), deadline: option.None[timex.Instant]()}
}

// OnDeadline creates a new CancelOption which indicates expiry
// when a monotonic clock reaches the instant `i`.
//
// Pre-condition: !i.IsZero()
func OnDeadline(i timex.Instant) CancelOption {
	assert.Preconditionf(!i.IsZero(), "zero value for Instant indicates likely bug in caller")
	// TODO: What pre-condition should we set here...? Ideally, Deadline()
	// should be >= programStart, because it's almost certainly a bug if
	// you're setting a deadline for a time before the program started.
	// but that imposes a dependency on global state...
	return CancelOption{timeout: option.None[timex.Duration](), deadline: option.Some(i)}
}

// Pre-conditions:
//  1. parent must be non-nil.
//  2. scheduler must be non-nil.
func NewClockToken(parent Token, scheduler timex.Scheduler, opts ...CancelOption) ChildClockToken {
	assert.Precondition(parent != nil, "parent token must be non-nil")
	assert.Precondition(scheduler != nil, "scheduler must be non-nil")

	var tightest option.Option[Deadline]
	if parentClockToken, ok := parent.(ClockToken); ok {
		if pd, ok := parentClockToken.Deadline().Get(); ok {
			tightest = option.Some(pd.ReinterpretForChild())
		}
	}

	var lazyNow option.Option[timex.Instant]
	for _, opt := range opts {
		instant, hasDeadline := opt.deadline.Get()
		duration, hasTimeout := opt.timeout.Get()
		if !hasDeadline && !hasTimeout {
			assert.Precondition(false, "got zero CancelOption value as argument")
		}
		if hasDeadline && hasTimeout {
			assert.Invariantf(false, "provided CancelOption %v, with both deadline and timeout; this should be impossible through the public API", opt)
		}
		if hasDeadline {
			soonest(&tightest, deadline.New(instant, deadline.NewSource(deadline.Source_InitializedWithAbsoluteDeadline, 0)))
		} else {
			if lazyNow.IsNone() {
				lazyNow = option.Some(scheduler.GetInstant())
			}
			now := lazyNow.Unwrap()
			soonest(&tightest, deadline.New(now.Add(duration), deadline.NewSource(deadline.Source_InitializedWithTimeout, 0)))
		}
	}

	return newClockTokenImpl(parent, scheduler, tightest, lazyNow)
}

func soonest(lhs *option.Option[Deadline], rhs Deadline) {
	if lhsDeadline, ok := lhs.Get(); ok {
		if !rhs.Instant().IsBefore(lhsDeadline.Instant()) {
			return
		}
		// rhs < lhs
	}
	// rhs < lhs || lhs.IsNone()
	*lhs = option.Some(rhs)
}

type clockToken struct {
	// Always non-nil.
	child ChildToken
	// Always non-nil.
	scheduler timex.Scheduler
	deadline  option.Option[Deadline]
	// Nil iff no future deadline was scheduled.
	timer timex.ScheduledFunc
}

// Pre-conditions:
// 1. parent is non-nil.
// 2. scheduler is non-nil.
func newClockTokenImpl(parent Token, scheduler timex.Scheduler, optDeadline option.Option[Deadline], lazyNow option.Option[timex.Instant]) *clockToken {
	ct := &clockToken{
		child:     parent.NewChild(),
		scheduler: scheduler,
		deadline:  optDeadline,
		timer:     nil,
	}
	deadline_, ok := optDeadline.Get()
	if !ok {
		return ct
	}
	now, ok := lazyNow.Get()
	if !ok {
		now = scheduler.GetInstant()
	}
	delay := deadline_.Instant().Sub(now)
	if delay <= 0 {
		ct.child.Cancel(deadline.NewExceededError(deadline_))
		return ct
	}
	ct.timer = scheduler.RunAfter(delay, func() {
		ct.child.Cancel(deadline.NewExceededError(deadline_))
	})
	return ct
}

var _ Token = (*clockToken)(nil)

func (c *clockToken) KeepGoing() error {
	return c.child.KeepGoing()
}

func (c *clockToken) Done() <-chan Unit {
	return c.child.Done()
}

func (c *clockToken) NewChild() ChildToken {
	return c.child.NewChild()
}

func (c *clockToken) AsStdlibContext() context.Context {
	return clockTokenContext{c: c}
}

var _ timex.MonotonicClock = (*clockToken)(nil)

func (c *clockToken) GetInstant() timex.Instant {
	return c.scheduler.GetInstant()
}

var _ ClockToken = (*clockToken)(nil)

func (c *clockToken) Deadline() option.Option[Deadline] {
	return c.deadline
}

func (c *clockToken) NewClockChild(opts ...CancelOption) ChildClockToken {
	return NewClockToken(c, c.scheduler, opts...)
}

var _ ChildToken = (*clockToken)(nil)

func (c *clockToken) Cancel(err error) CancelResult {
	res := c.child.Cancel(err)
	if c.timer != nil {
		c.timer.Stop()
	}
	return res
}

type clockTokenContext struct {
	// Always non-nil.
	c *clockToken
}

var _ context.Context = (*clockTokenContext)(nil)

func (cc clockTokenContext) Deadline() (stdlib_time.Time, bool) {
	d, ok := cc.c.deadline.Get()
	if !ok {
		return stdlib_time.Time{}, false
	}
	return d.Instant().AsStdlibTime(), true
}

func (cc clockTokenContext) Done() <-chan struct{} {
	return cc.c.Done()
}

func (cc clockTokenContext) Err() error {
	return stdlibContextErr(cc.c.KeepGoing())
}

func (cc clockTokenContext) Value(_ any) any {
	return nil
}
