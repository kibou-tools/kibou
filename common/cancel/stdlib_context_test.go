// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cancel_test

import (
	"context"
	"testing"

	"code.kibou.tools/common/cancel"
	"code.kibou.tools/common/check"
	"code.kibou.tools/common/errorx"
	"code.kibou.tools/common/timex"
)

func TestAsStdlibContext(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("never token", func(h check.Harness) {
		h.Parallel()

		tok := cancel.Never()
		ctx := tok.AsStdlibContext()

		_, ok := ctx.Deadline()
		h.Assertf(!ok, "Never().AsStdlibContext().Deadline() should report no deadline")
		h.Assertf(ctx.Done() == nil, "Never().AsStdlibContext().Done() = %v, want nil", ctx.Done())
		h.Assertf(ctx.Err() == nil, "Never().AsStdlibContext().Err() = %v, want nil", ctx.Err())
		h.Assertf(ctx.Value("key") == nil, "Never().AsStdlibContext().Value() should always be nil")
	})

	h.Run("child token", func(h check.Harness) {
		h.Parallel()

		errBoom := errorx.New("nostack", "boom")
		tok := cancel.Never().NewChild()
		ctx := tok.AsStdlibContext()

		_, ok := ctx.Deadline()
		h.Assertf(!ok, "child context should report no deadline")
		h.Assertf(ctx.Done() == tok.Done(), "context Done channel should be token Done channel")
		h.Assertf(ctx.Err() == nil, "live child context Err() = %v, want nil", ctx.Err())
		h.Assertf(ctx.Value("key") == nil, "child context Value() should always be nil")

		select {
		case <-ctx.Done():
			h.Assertf(false, "context Done channel closed before token cancellation")
		default:
		}

		tok.Cancel(errBoom)
		select {
		case <-ctx.Done():
		default:
			h.Assertf(false, "context Done channel did not close after token cancellation")
		}
		h.Assertf(errorx.GetRootCauseAsValue(ctx.Err(), context.Canceled), "canceled child context Err() = %v, want %v", ctx.Err(), context.Canceled)
	})

	h.Run("clock token without deadline", func(h check.Harness) {
		h.Parallel()

		scheduler := newManualScheduler()
		tok := cancel.NewClockToken(cancel.Never(), scheduler)
		ctx := tok.AsStdlibContext()

		_, ok := ctx.Deadline()
		h.Assertf(!ok, "clock context without cancel options should report no deadline")
		h.Assertf(ctx.Done() == tok.Done(), "clock context Done channel should be token Done channel")
		h.Assertf(ctx.Err() == nil, "live clock context Err() = %v, want nil", ctx.Err())
		h.Assertf(ctx.Value("key") == nil, "clock context Value() should always be nil")
	})

	h.Run("clock token with deadline", func(h check.Harness) {
		h.Parallel()

		scheduler := newManualScheduler()
		start := scheduler.GetInstant()
		timeout := 5 * timex.Second
		tok := cancel.NewClockToken(cancel.Never(), scheduler, cancel.OnTimeout(timeout))
		ctx := tok.AsStdlibContext()

		gotDeadline, ok := ctx.Deadline()
		h.Assertf(ok, "clock context should report configured deadline")
		wantDeadline := start.Add(timeout).AsStdlibTime()
		h.Assertf(gotDeadline.Equal(wantDeadline), "clock context deadline = %v, want %v", gotDeadline, wantDeadline)
		h.Assertf(ctx.Done() == tok.Done(), "clock context Done channel should be token Done channel")
		h.Assertf(ctx.Err() == nil, "live clock context Err() = %v, want nil", ctx.Err())
		h.Assertf(ctx.Value("key") == nil, "clock context Value() should always be nil")

		scheduler.Advance(timeout)
		select {
		case <-ctx.Done():
		default:
			h.Assertf(false, "clock context Done channel did not close after deadline")
		}
		h.Assertf(errorx.GetRootCauseAsValue(ctx.Err(), context.DeadlineExceeded), "expired clock context Err() = %v, want %v", ctx.Err(), context.DeadlineExceeded)
	})
}
