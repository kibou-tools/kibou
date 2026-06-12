// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package limited

import (
	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/op"
)

// Counter is a simple single-threaded counter value
// which starts returning op.NoGo after the [Counter.Limit]
// is hit.
type Counter[U uint8 | uint16 | uint32 | uint64] struct {
	limit U
	hits  U
}

// NewCounter creates a new counter with the given limit.
//
// The limit cannot be modified after counter creation.
func NewCounter[U uint8 | uint16 | uint32 | uint64](limit U) Counter[U] {
	return Counter[U]{limit, 0}
}

// Inc increments the counter and returns whether it's
// OK to do the operation guarded by this counter.
//
// Precondition: Inc must not be called more times
// than the maximum value of the type U. In other words,
// the counter panics on overflow.
func (c *Counter[U]) Inc() op.Next {
	c.hits++
	if c.hits == 0 {
		assert.Preconditionf(false, "counter of type %T incremented %d times", c.hits, c.hits-1)
	}
	if c.hits <= c.limit {
		return op.KeepGoing
	}
	return op.NoGo
}

// Dropped returns the number of operations that were skipped.
func (c *Counter[U]) Dropped() U {
	if c.hits <= c.limit {
		return 0
	}
	return c.hits - c.limit
}

// Hits returns the number of times the counter was incremented.
func (c *Counter[U]) Hits() U {
	return c.hits
}

// Limit returns the limit stored inside the counter.
func (c *Counter[U]) Limit() U {
	return c.limit
}
