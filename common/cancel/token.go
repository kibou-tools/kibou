// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package cancel provides cause-aware cancellation tokens.
package cancel

import (
	"context"
	"slices"
	"sync"
	stdlib_time "time" //nolint:depguard // cancel is the designated context wrapper

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/cancel/deadline"
	"code.kibou.tools/common/collections"
	"code.kibou.tools/common/errorx"
	. "code.kibou.tools/common/unit"
)

// --- Aliases for package-external use ---

type Result = CancelResult

// --- Package implementation ---

// Token is the base cancellation primitive.
//
// A token is a simple state machine:
//
//		 Alive ─► Canceled
//	           │      ▲
//	           └──────┘
//
// Once it has been canceled, it stays canceled.
type Token interface {
	// KeepGoing returns nil if the Token is still live, or a non-nil error
	// describing why cancellation occurred.
	//
	// Requirement: If the reason for cancellation is the crossing of a deadline,
	// then the error must be a deadline.ExceededError as its root cause.
	KeepGoing() error

	// Done returns a channel that is closed when the Token is canceled.
	//
	// This is meant to be used in select statements.
	Done() <-chan Unit

	// NewChild returns a ChildToken that is automatically canceled if the receiver
	// is canceled. Cancellation flow is unidirectional: canceling the child does
	// not affect the parent.
	//
	// In most cases, you don't really need to use this unless you're implementing
	// a data type related to unstructured concurrency yourself.
	//
	// If you want to cancel subtasks based on a timeout, take a look
	// at [ClockToken.NewClockChild] instead.
	NewChild() ChildToken

	// AsStdlibContext() reinterprets this cancellation token as a [context.Context]
	// value from the standard library.
	//
	// This is used for bridging into APIs which only work with [context.Context].
	// The [context.Context.Value] method will return nil for all keys.
	AsStdlibContext() context.Context
}

// CancelResult reports whether a cancellation attempt
// triggered cancellation, or if the token was already
// canceled earlier.
type CancelResult int

const (
	// Result_CanceledByUs means this Cancel call transitioned the token
	// from live to canceled and fixed the token's cancellation error.
	Result_CanceledByUs CancelResult = iota + 1

	// Result_AlreadyCanceled means the token was already canceled by someone else.
	Result_AlreadyCanceled
)

// ChildToken is both a cancellation signal and the authority to cancel that signal.
type ChildToken interface {
	Token

	// Cancel attempts to cancel the token with err.
	//
	// There are two possibilities:
	//
	//   1. The token was already canceled.
	//   2. The token was canceled by this particular Cancel invocation.
	//
	// The return value provides that information.
	//
	// Pre-condition: err != nil.
	//
	// Requirement: Cancel must run synchronously. In particular, the
	// following must be guaranteed:
	//
	//     tok.Cancel(someErr)
	//     gotErr := tok.KeepGoing()
	//     assert.Invariant(gotErr != nil, "Cancel is synchronous")
	Cancel(err error) CancelResult
}

type tokenContext struct {
	tok Token
}

func (c tokenContext) Deadline() (stdlib_time.Time, bool) {
	return stdlib_time.Time{}, false
}

func (c tokenContext) Done() <-chan struct{} {
	return c.tok.Done()
}

func (c tokenContext) Err() error {
	return stdlibContextErr(c.tok.KeepGoing())
}

func (c tokenContext) Value(_ any) any {
	return nil
}

func stdlibContextErr(err error) error {
	if err == nil {
		return nil
	}
	if errorx.GetRootCauseAs[deadline.ExceededError](err).IsSome() {
		return context.DeadlineExceeded
	}
	return context.Canceled
}

type rawToken struct {
	// Cancelable parent.
	//
	// Nil when one of the following is true:
	//   - Created under Never().
	//   - Parent was already canceled during child construction.
	//
	// May remain non-nil after cancellation.
	// Immutable after construction.
	parent *rawToken

	// Index in parent.children.
	//
	// Valid only when:
	//   - parent != nil.
	//   - parent.children[parentIdx] == this token.
	//
	// Protected by parent.mu, not this token's mu.
	// (This implies that a single token's mu can protect
	// arbitrary many children's parentIdx.)
	parentIdx int

	// Protects:
	//   - err.
	//   - children.
	//   - parentIdx of each c in children (not our own)
	//
	// Does not protect:
	//   - parent: immutable after construction.
	//   - done: immutable after construction.
	mu sync.Mutex

	// Closed when canceled.
	//
	// Always non-nil.
	done chan Unit

	// Direct children registered for cancellation propagation.
	//
	// Nil when either:
	//   - No children are registered.
	//   - This token is canceled.
	children []*rawToken

	// Cancellation cause.
	//
	// Nil while alive. Non-nil after cancellation. Set once.
	err error
}

// newChildToken creates a new rawToken that has the
// `parent` argument as its parent.
//
// parent may be nil. If not, the parent's registerChild
// method will be called.
func newChildToken(parent *rawToken) *rawToken {
	t := &rawToken{
		parent:    parent,
		parentIdx: 0,
		mu:        sync.Mutex{},
		done:      make(chan Unit),
		children:  nil,
		err:       nil,
	}
	if parent == nil {
		return t
	}
	if err := parent.registerChild(t); err != nil {
		t.parent = nil
		t.Cancel(err)
	}
	return t
}

var _ Token = (*rawToken)(nil)

func (t *rawToken) KeepGoing() error {
	t.mu.Lock()
	err := t.err
	t.mu.Unlock()
	return err
}

func (t *rawToken) Done() <-chan Unit {
	return t.done
}

func (t *rawToken) AsStdlibContext() context.Context {
	return tokenContext{tok: t}
}

func (t *rawToken) NewChild() ChildToken {
	return newChildToken(t)
}

func (t *rawToken) Cancel(err error) CancelResult {
	assert.Precondition(err != nil, "Cancel called with nil error")

	// Use a queue to avoid recursion in cancellation.
	// Generally, a token tree shouldn't be deep, but avoiding
	// recursion eliminates the risk of stack overflow.
	queue := collections.NewDeque[*rawToken]()
	result := t.cancelSelfAndDetachChildren(err, &queue)
	switch result {
	case Result_CanceledByUs:
		if t.parent != nil {
			t.parent.unregisterChild(t)
		}
		// Use BFS instead of DFS for fairness/more intuitive behavior
		// for the common case where only t is canceled but none of
		// its descendants have been (directly) canceled.
		for {
			if descendant, ok := queue.TryPopFront().Get(); ok {
				descendant.cancelSelfAndDetachChildren(err, &queue)
				continue
			}
			break
		}
		return result
	case Result_AlreadyCanceled:
		// If the receiver was already canceled, then that cancellation logic
		// will handle cancellation of descendants too, so there's nothing
		// to do here. Don't double-cancel descendants.
		return Result_AlreadyCanceled
	default:
		return assert.PanicUnknownCase[CancelResult](result)
	}
}

// registerChild registers the child token as belonging to the receiver.
//
// Returns an error if the receiver has already been canceled.
//
// Pre-condition: The child pointer is owning/not accessed concurrently.
//
// Post-condition: If there's no error, child.parentIdx is set to the
// child's position in the receiver's [rawToken.children] slice.
func (t *rawToken) registerChild(child *rawToken) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return t.err
	}
	child.parentIdx = len(t.children)
	t.children = append(t.children, child)
	return nil
}

// unregisterChild unregisters the child token from the receiver's list.
//
// This method is meant to be called only during the cancellation of
// the `child` argument.
//
// Pre-condition: The receiver is the parent of the child argument.
func (t *rawToken) unregisterChild(child *rawToken) {
	parent := t
	parent.mu.Lock()
	defer parent.mu.Unlock()
	if parent.err != nil {
		// Our parent was canceled while `child` is being canceled.
		// Since we have the lock now, it means that cancelSelfAndDetachChildren
		// must've finished, so parent.children must be nil.
		// Assert that as a sanity check.
		assert.Invariant(parent.children == nil, "canceled token has children")
		// Since parent.children is already nil, the parent's cancellation
		// logic will handle cleanup of the old slice; there's nothing we
		// can do here.
		return
	}
	// parent.err == nil => parent is alive right now.
	//
	// child.parentIdx is protected by parent.mu (acquired above),
	// so this read won't race.
	idx := child.parentIdx

	n := len(parent.children)
	{
		// Perform some sanity checks that parentIdx is correct.
		if idx < 0 || n <= idx {
			assert.PanicInvariantViolation[any]("child.parentIdx (%d) not in [0, %d); please report this as a bug", idx, n)
			return
		}
		if c := parent.children[idx]; c != child {
			assert.PanicInvariantViolation[any]("child token (%p) at index differs from child being unregistered (%p); please report this as a bug", c, child)
			return
		}
	}
	// idx ∈ [0, n) => n >= 1
	last := n - 1 // n >= 1 => last ∈ [0, n)
	if idx != last {
		// Move last child earlier to be able to trim the slice's tail.
		moved := parent.children[last]
		parent.children[idx] = moved
		// parent is alive => ∀ c ∈ parent.children, c.parent == parent
		if moved.parent != parent {
			assert.PanicInvariantViolation[any]("child token (%p) has parent (%p), but it's registered as child of token %p",
				moved, moved.parent, parent)
			return
		}
		// Write to parentIdx is OK, because moved is also a child of parent,
		// so its parentIdx field is also protected by parent.mu
		moved.parentIdx = idx
	}
	parent.children[last] = nil // eagerly drop reference to child for GC
	parent.children = parent.children[:last]
	if last == 0 {
		// Restore field invariant that parent.children = nil iff there are no registered children
		parent.children = nil
		return
	}
	// n >= 2 because (last == 0 <=> n == 2) was already handled above.
	newLen := n - 1 // n >= 2 => newLen >= 1

	// Try to free up space if the slice is being underutilized.
	// The 25% utilization threshold is chosen arbitrarily.
	if 4*newLen <= cap(parent.children) {
		// TODO: We don't have strong guarantees about the capacity
		// of the buffer returned by slices.Clone. We need a guarantee
		// here that the utilization percentage will exceed 25%,
		// or perhaps a stronger guarantee that we won't have to Clone
		// the slice again on the next removal.
		parent.children = slices.Clone(parent.children)
	}
}

// cancelSelfAndDetachChildren cancels the receiver (if it hasn't
// already been canceled). If the receiver token has any children,
// they are pushed to the back of the queue parameter.
//
// The detailed cancellation sequence is:
//
//  1. Lock t.mu.
//  2. Set t.err.
//  3. Close t.done.
//  4. Detach t.children.
//  5. Unlock t.mu.
//  6. Enqueue detached children.
//
// Rationale:
//   - Set err before closing done so waiters can observe the cause.
//   - Detach children under mu so only one caller owns propagation.
//   - Enqueue after unlock to avoid growing queue while holding t.mu.
//
// Pre-condition: queue != nil.
func (t *rawToken) cancelSelfAndDetachChildren(err error, queue *collections.Deque[*rawToken]) CancelResult {
	assert.Precondition(queue != nil, "cancelSelfAndDetachChildren should be able to append to queue")
	t.mu.Lock()
	if t.err != nil {
		t.mu.Unlock()
		return Result_AlreadyCanceled
	}
	t.err = err
	close(t.done)
	children := t.children
	t.children = nil
	t.mu.Unlock()

	queue.ReserveMore(len(children))
	for _, child := range children {
		queue.PushBack(child)
	}
	return Result_CanceledByUs
}
