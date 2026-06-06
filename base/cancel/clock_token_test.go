// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cancel_test

import (
	"slices"
	"testing"
	stdlib_time "time" //nolint:depguard // tests need deterministic timex.Instant values

	"pgregory.net/rapid"

	"code.kibou.tools/base/cancel"
	"code.kibou.tools/base/cancel/deadline"
	"code.kibou.tools/base/check"
	"code.kibou.tools/base/check/pbt/flat"
	"code.kibou.tools/base/core/option"
	"code.kibou.tools/base/errorx"
	"code.kibou.tools/base/timex"
)

func TestClockToken(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("Unit", testClockTokenUnit)
	h.Run("Properties", testClockTokenProperties)
}

func testClockTokenUnit(h check.Harness) {
	h.Parallel()

	h.Run("timeout", func(h check.Harness) {
		h.Parallel()

		scheduler := newManualScheduler()
		start := scheduler.GetInstant()
		timeout := 5 * timex.Second
		tok := cancel.NewClockToken(cancel.Never(), scheduler, cancel.OnTimeout(timeout))

		d, ok := tok.Deadline().Get()
		h.Assertf(ok, "token should have a deadline")
		h.Assertf(d.Instant().Equals(start.Add(timeout)), "deadline instant = %v, want start+5s", d.Instant())
		h.Assertf(d.Source() == deadline.NewSource(deadline.Source_InitializedWithTimeout, 0), "deadline source = %v", d.Source())
		h.Assertf(scheduler.PendingCount() == 1, "pending timers = %d, want 1", scheduler.PendingCount())
		h.Assertf(tok.KeepGoing() == nil, "token should initially be live")

		scheduler.Advance(timeout - timex.Nanosecond)
		h.Assertf(tok.KeepGoing() == nil, "token should be live before deadline")
		scheduler.Advance(timex.Nanosecond)
		h.Assertf(tok.KeepGoing() != nil, "token should be expired after deadline")

		gotErr := errorx.GetRootCauseAs[deadline.ExceededError](tok.KeepGoing()).Unwrap()
		h.Assertf(gotErr.Deadline().Instant().Compare(d.Instant()) == 0, "exceeded deadline instant = %v, want %v", gotErr.Deadline().Instant(), d.Instant())
		h.Assertf(scheduler.PendingCount() == 0, "pending timers = %d, want 0", scheduler.PendingCount())
	})

	h.Run("synchronous cancellation", func(h check.Harness) {
		h.Parallel()

		scheduler := newManualScheduler()
		base := scheduler.GetInstant()
		tok := cancel.NewClockToken(cancel.Never(), scheduler, cancel.OnDeadline(base))

		gotErr := errorx.GetRootCauseAs[deadline.ExceededError](tok.KeepGoing()).Unwrap()
		h.Assertf(gotErr.Deadline().Instant().Compare(base) == 0, "exceeded deadline instant = %v, want base", gotErr.Deadline().Instant())
		h.Assertf(scheduler.PendingCount() == 0, "pending timers = %d, want 0", scheduler.PendingCount())
	})

	h.Run("cancellation cause is sticky", func(h check.Harness) {
		h.Parallel()

		scheduler := newManualScheduler()
		tok := cancel.NewClockToken(cancel.Never(), scheduler, cancel.OnTimeout(5*timex.Second))
		errBoom := errorx.New("nostack", "boom")

		h.Assertf(tok.Cancel(errBoom) == cancel.Result_CanceledByUs, "first cancel should win")
		h.Assertf(tok.KeepGoing() == errBoom, "KeepGoing() = %v, want errBoom", tok.KeepGoing())
		h.Assertf(scheduler.PendingCount() == 0, "pending timers = %d, want 0", scheduler.PendingCount())

		scheduler.Advance(5 * timex.Second)
		h.Assertf(tok.KeepGoing() == errBoom, "deadline expiry should not replace explicit cancellation cause")
	})

	h.Run("parent deadline propagates", func(h check.Harness) {
		h.Parallel()

		parentTimeout := 5 * timex.Second
		childTimeout := 10 * timex.Second

		scheduler := newManualScheduler()
		parent := cancel.NewClockToken(cancel.Never(), scheduler, cancel.OnTimeout(parentTimeout))
		child := parent.NewClockChild(cancel.OnTimeout(childTimeout))

		parentDeadline := parent.Deadline().Unwrap()
		childDeadline := child.Deadline().Unwrap()
		h.Assertf(parentDeadline.Instant() == childDeadline.Instant(),
			"child deadline instant = %v, want %v", childDeadline.Instant(), parentDeadline.Instant())
	})
}

// A tree of ChildClockToken values.
//
// For each token, there is one nominal deadline source:
// - No deadline
// - Timeout based deadline (from the token's creation instant)
// - Absolute deadline
//
// Creation times are weakly monotonic along any path down the tree, but sibling
// subtrees may be created in either order. We first generate that model tree,
// then drive one scheduler monotonically through all creation/deadline events.
// As nominal deadlines pass, those tokens should cancel their whole subtrees.
func testClockTokenProperties(h check.Harness) {
	h.Parallel()

	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		scheduler := newManualScheduler()
		nodeBudget := rapid.IntRange(1, 10).Draw(t, "node_budget")
		nextIndex := 1

		base := scheduler.GetInstant()
		rootCreationTime := drawClockTokenCreationTime(t, base)
		rootOpts, rootOwnDeadline := drawClockTokenDeadline(t, rootCreationTime)

		tree := flat.UnfoldTree(
			clockTokenTreeNode{
				creationTime: rootCreationTime,
				opts:         rootOpts,
				ownDeadline:  rootOwnDeadline,
			},
			func(node clockTokenTreeNode, yieldChild func(clockTokenTreeNode)) clockTokenTreeNode {
				remaining := nodeBudget - nextIndex
				if remaining > 0 {
					childCount := rapid.IntRange(0, min(remaining, 4)).Draw(t, "child_count")
					for range childCount {
						nextIndex++
						childCreationTime := drawClockTokenCreationTime(t, node.creationTime)
						childOpts, childOwnDeadline := drawClockTokenDeadline(t, childCreationTime)
						yieldChild(clockTokenTreeNode{
							creationTime: childCreationTime,
							opts:         childOpts,
							ownDeadline:  childOwnDeadline,
						})
					}
				}
				return node
			},
		)

		type clockTokenEventKind uint8
		const (
			clockTokenEvent_Create clockTokenEventKind = iota + 1
			clockTokenEvent_Deadline
		)
		type clockTokenEvent struct {
			at   timex.Instant
			kind clockTokenEventKind
			id   flat.TreeID
		}
		var events []clockTokenEvent
		for i := range tree.NodeCount() {
			id := tree.ID(i)
			node := tree.Value(id)
			events = append(events, clockTokenEvent{at: node.creationTime, kind: clockTokenEvent_Create, id: id})
			if d, ok := node.ownDeadline.Get(); ok {
				events = append(events, clockTokenEvent{at: d, kind: clockTokenEvent_Deadline, id: id})
			}
		}
		slices.SortFunc(events, func(lhs, rhs clockTokenEvent) int {
			if cmp := lhs.at.Compare(rhs.at); cmp != 0 {
				return cmp
			}
			if lhs.kind != rhs.kind {
				return int(lhs.kind) - int(rhs.kind)
			}
			return lhs.id.Compare(rhs.id)
		})

		created := make([]bool, tree.NodeCount())
		tokens := make([]cancel.ChildClockToken, tree.NodeCount())
		assertCanceled := func(expectedAll []bool) {
			expected := make([]bool, tree.NodeCount())
			got := make([]bool, tree.NodeCount())
			for j := range got {
				if !created[j] {
					continue
				}
				node := tree.Value(tree.ID(j))
				tok := tokens[j]
				expected[j] = expectedAll[j]
				got[j] = tok.KeepGoing() != nil
				h.Assertf(tok.GetInstant().Compare(scheduler.GetInstant()) == 0, "node %d clock should be scheduler clock", j)

				gotDeadline, gotDeadlineOK := tok.Deadline().Get()
				gotDeadlineAgain, gotDeadlineAgainOK := tok.Deadline().Get()
				h.Assertf(gotDeadlineOK == gotDeadlineAgainOK, "node %d Deadline presence changed between calls", j)
				if gotDeadlineOK {
					h.Assertf(gotDeadline == gotDeadlineAgain, "node %d Deadline value changed between calls", j)
				}
				h.Assertf(!node.creationTime.IsAfter(scheduler.GetInstant()), "node %d should not be created in the future", j)
			}
			check.AssertSame(h, expected, got, "canceled tokens")
		}

		createToken := func(id flat.TreeID) {
			node := tree.Value(id)
			if id.Index() == 0 {
				tokens[id.Index()] = cancel.NewClockToken(cancel.Never(), scheduler, node.opts...)
			} else {
				parentID := tree.ParentID(id).Unwrap()
				h.Assertf(created[parentID.Index()], "parent %d should be created before child %d", parentID.Index(), id.Index())
				tokens[id.Index()] = tokens[parentID.Index()].NewClockChild(node.opts...)
			}
			created[id.Index()] = true
		}

		var expiredIDs []flat.TreeID
		now := base
		for eventIndex := 0; eventIndex < len(events); {
			eventInstant := events[eventIndex].at
			scheduler.Advance(eventInstant.Sub(now))
			now = eventInstant

			for eventIndex < len(events) && events[eventIndex].at.Compare(eventInstant) == 0 {
				event := events[eventIndex]
				switch event.kind {
				case clockTokenEvent_Create:
					createToken(event.id)
				case clockTokenEvent_Deadline:
					expiredIDs = append(expiredIDs, event.id)
				}
				eventIndex++
			}

			sortedExpiredIDs := slices.Clone(expiredIDs)
			slices.SortFunc(sortedExpiredIDs, flat.TreeID.Compare)
			assertCanceled(tree.MarkSubtrees(sortedExpiredIDs))
		}
	})
}

type clockTokenTreeNode struct {
	creationTime timex.Instant
	opts         []cancel.Option
	// None iff this token has no deadline directly configured on itself.
	ownDeadline option.Option[timex.Instant]
}

func drawClockTokenCreationTime(t *rapid.T, earliest timex.Instant) timex.Instant {
	d := timex.Duration(rapid.IntRange(0, 10).Draw(t, "creation_delay_seconds")) * timex.Second
	return earliest.Add(d)
}

func drawClockTokenDeadline(t *rapid.T, creationTime timex.Instant) ([]cancel.Option, option.Option[timex.Instant]) {
	deadlineKind := rapid.IntRange(0, 2).Draw(t, "deadline_kind")
	if deadlineKind == 0 {
		return nil, option.None[timex.Instant]()
	}
	d := timex.Duration(rapid.IntRange(0, 10).Draw(t, "deadline_seconds")) * timex.Second
	at := creationTime.Add(d)
	if deadlineKind == 1 {
		return []cancel.Option{cancel.OnTimeout(d)}, option.Some(at)
	}
	return []cancel.Option{cancel.OnDeadline(at)}, option.Some(at)
}

type manualScheduler struct {
	now    timex.Instant
	nextID int
	// Nil iff no functions have been scheduled.
	timers []*manualScheduledFunc
}

var _ timex.Scheduler = (*manualScheduler)(nil)

func newManualScheduler() *manualScheduler {
	return &manualScheduler{
		now:    timex.NewInstant(stdlib_time.Unix(1_000, 0)),
		nextID: 0,
		timers: nil,
	}
}

func (s *manualScheduler) GetInstant() timex.Instant {
	return s.now
}

func (s *manualScheduler) RunAfter(d timex.Duration, f func()) timex.ScheduledFunc {
	if d < 0 {
		panic("negative duration")
	}
	if f == nil {
		panic("nil scheduled function")
	}
	s.nextID++
	timer := &manualScheduledFunc{
		id:      s.nextID,
		at:      s.now.Add(d),
		f:       f,
		started: false,
		stopped: false,
	}
	s.timers = append(s.timers, timer)
	return timer
}

func (s *manualScheduler) Advance(d timex.Duration) {
	if d < 0 {
		panic("negative duration")
	}
	target := s.now.Add(d)
	for {
		timer := s.nextReadyTimer(target)
		if timer == nil {
			break
		}
		s.now = timer.at
		timer.started = true
		timer.f()
	}
	s.now = target
}

func (s *manualScheduler) PendingCount() int {
	count := 0
	for _, timer := range s.timers {
		if !timer.started && !timer.stopped {
			count++
		}
	}
	return count
}

func (s *manualScheduler) nextReadyTimer(target timex.Instant) *manualScheduledFunc {
	var best *manualScheduledFunc
	for _, timer := range s.timers {
		if timer.started || timer.stopped || timer.at.IsAfter(target) {
			continue
		}
		if best == nil || timer.at.IsBefore(best.at) || (timer.at.Compare(best.at) == 0 && timer.id < best.id) {
			best = timer
		}
	}
	return best
}

type manualScheduledFunc struct {
	id int
	at timex.Instant
	// Always non-nil.
	f       func()
	started bool
	stopped bool
}

var _ timex.ScheduledFunc = (*manualScheduledFunc)(nil)

func (f *manualScheduledFunc) Stop() timex.StopResult {
	if f.started || f.stopped {
		return timex.StopResult_TooLate
	}
	f.stopped = true
	return timex.StopResult_Stopped
}
