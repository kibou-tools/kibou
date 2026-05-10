// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cmpx_test

import (
	"testing"

	"code.kibou.tools/common/check"
	"code.kibou.tools/common/cmpx"
)

func TestCompareBool(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	type TestCase struct {
		LHS  bool
		RHS  bool
		Want int
	}

	testCases := []TestCase{
		{LHS: false, RHS: false, Want: 0},
		{LHS: false, RHS: true, Want: -1},
		{LHS: true, RHS: false, Want: 1},
		{LHS: true, RHS: true, Want: 0},
	}

	for _, tc := range testCases {
		check.AssertSame(h, tc.Want, cmpx.CompareBool(tc.LHS, tc.RHS), "comparison result")
	}
}
