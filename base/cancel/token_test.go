// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cancel_test

import (
	"slices"
	"sync"
	"testing"

	"pgregory.net/rapid"

	"code.kibou.tools/base/cancel"
	"code.kibou.tools/base/check"
	"code.kibou.tools/base/check/pbt/flat"
	"code.kibou.tools/base/errorx"
)

func TestChildTokenCancel(t *testing.T) {
	h := check.New(t)

	errBoom := errorx.New("nostack", "boom")
	tok := cancel.Never().NewChild()
	h.Assertf(tok.KeepGoing() == nil, "new child should be live")
	h.Assertf(tok.Cancel(errBoom) == cancel.Result_CanceledByUs, "first cancel should win")
	h.Assertf(tok.KeepGoing() == errBoom, "KeepGoing() = %v, want errBoom", tok.KeepGoing())
	h.Assertf(tok.Cancel(errorx.New("nostack", "second boom")) == cancel.Result_AlreadyCanceled,
		"second cancel should lose")
	h.Assertf(tok.KeepGoing() == errBoom, "second cancel should not replace cause")
}

func TestTokenTreeCancellation(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	rapid.Check(h.T(), func(t *rapid.T) {
		basic := check.NewBasic(t)
		nodeBudget := rapid.IntRange(1, 10).Draw(t, "node_budget")
		nextIndex := 1

		type ChildToken = cancel.ChildToken

		tree := flat.UnfoldTree(cancel.Never().NewChild(), func(tok ChildToken, yieldChild func(ChildToken)) ChildToken {
			remaining := nodeBudget - nextIndex
			if remaining > 0 {
				childCount := rapid.IntRange(0, min(remaining, 4)).Draw(t, "child_count")
				for range childCount {
					nextIndex++
					yieldChild(tok.NewChild())
				}
			}
			return tok
		})

		allIDs := make([]flat.TreeID, tree.NodeCount())
		for i := range allIDs {
			allIDs[i] = tree.ID(i)
		}
		cancelIDs := rapid.SliceOfN(rapid.SampledFrom(allIDs), 0, tree.NodeCount()).Draw(t, "cancel_ids")

		var wg sync.WaitGroup
		wg.Add(1)
		var expected []bool
		go func() {
			defer wg.Done()
			sortedCancelIDs := slices.Clone(cancelIDs)
			slices.SortFunc(sortedCancelIDs, flat.TreeID.Compare)
			expected = tree.MarkSubtrees(sortedCancelIDs)
		}()

		for _, id := range cancelIDs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tree.Value(id).Cancel(errorx.Newf("nostack", "cancel token %d", id.Index()))
			}()
		}
		wg.Wait()

		got := make([]bool, tree.NodeCount())
		for i := range got {
			id := tree.ID(i)
			got[i] = tree.Value(id).KeepGoing() != nil
		}
		check.AssertSame(basic, expected, got, "canceled tokens")
	})
}

func TestCancel_NoStackOverflow(t *testing.T) {
	h := check.New(t)

	errBoom := errorx.New("nostack", "boom")
	root := cancel.Never().NewChild()
	var leaf cancel.Token = root
	for range 100_000 {
		leaf = leaf.NewChild()
	}
	root.Cancel(errBoom)
	<-leaf.Done()
	h.Assertf(leaf.KeepGoing() == errBoom, "leaf KeepGoing() = %v, want errBoom", leaf.KeepGoing())
}
