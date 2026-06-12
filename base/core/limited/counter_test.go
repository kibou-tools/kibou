// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package limited_test

import (
	"math"
	"testing"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	"code.kibou.tools/base/core/limited"
	"code.kibou.tools/base/core/op"
)

func TestCounter(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	counter := limited.NewCounter[uint8](2)
	h.Assertf(counter.Inc() == op.KeepGoing, "first hit should be allowed")
	h.Assertf(counter.Inc() == op.KeepGoing, "second hit should be allowed")
	h.Assertf(counter.Inc() == op.NoGo, "third hit should be dropped")
	h.Assertf(counter.Hits() == 3, "hits should be 3")
	h.Assertf(counter.Dropped() == 1, "dropped should be 1")

	for i := uint16(counter.Hits()); i < math.MaxUint8; i++ {
		counter.Inc()
	}
	h.AssertPanicsWith(assert.AssertionError{
		Fmt:  "precondition violation: counter of type %T incremented %d times",
		Args: []any{uint8(0), uint8(255)},
	}, func() {
		counter.Inc()
	})
}
