// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cancel

import (
	"testing"

	"code.kibou.tools/common/check"
	"code.kibou.tools/common/errorx"
)

func TestConcurrentChildAndParentCancelInterleaving(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("whitebox", func(h check.Harness) {
		h.Parallel()

		errBoom := errorx.New("nostack", "boom")
		parent := newChildToken(nil)
		child := parent.NewChild().(*rawToken)

		h.Assertf(parent.Cancel(errBoom) == Result_CanceledByUs, "parent cancellation should win")
		h.Assertf(parent.KeepGoing() == errBoom, "parent KeepGoing() = %v, want errBoom", parent.KeepGoing())
		h.Assertf(child.KeepGoing() == errBoom, "child should have been canceled by parent")
		h.Assertf(parent.children == nil, "canceled parent should have detached children")

		// This state can occur when a child cancellation reaches unregisterChild after
		// parent cancellation has already detached the child list. Call the helper
		// directly to cover that deterministic branch without relying on scheduler
		// races.
		parent.unregisterChild(child)
	})

	h.Run("blackbox", func(h check.Harness) {
		h.Parallel()

		for attempt := range 10 {
			parent := Never().NewChild()
			child := parent.NewChild()
			for range 1_000 {
				child.NewChild()
			}

			errChild := errorx.Newf("nostack", "child cancel attempt %d", attempt)
			errParent := errorx.Newf("nostack", "parent cancel attempt %d", attempt)
			childCancelReturned := make(chan struct{})
			go func() {
				defer close(childCancelReturned)
				child.Cancel(errChild)
			}()

			// child.Done is closed before child unregisters itself from parent.
			// The grandchildren above make it more likely that parent.Cancel runs
			// during that window and exercises unregisterChild's parent-already-canceled
			// path. The test's correctness does not rely on winning that race; coverage
			// can be used to check whether the interleaving was observed in a run.
			<-child.Done()
			parent.Cancel(errParent)
			<-childCancelReturned

			h.Assertf(parent.KeepGoing() != nil, "attempt %d: parent should be canceled", attempt)
			h.Assertf(child.KeepGoing() != nil, "attempt %d: child should be canceled", attempt)
		}
	})
}
