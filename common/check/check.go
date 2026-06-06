// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package check provides test assertion and snapshot helpers.
package check

import (
	"flag"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"code.kibou.tools/common/assert"
)

func init() {
	// For compatibility with autogold and other packages that also define
	// an -update parameter, only register the flag if it's not already defined.
	// See NOTE(id: update-flag) — autogold uses the same pattern.
	if flag.Lookup("update") == nil {
		flag.Bool("update", false, "update snapshot files")
	}
}

// IsUpdateFlagSet reports whether the -update flag is set.
func IsUpdateFlagSet() bool {
	f := flag.Lookup("update")
	assert.Invariant(f != nil, "update flag is not registered")
	getter, ok := f.Value.(flag.Getter)
	assert.Invariant(ok, "update flag does not implement flag.Getter")
	value, ok := getter.Get().(bool)
	assert.Invariantf(ok, "update flag has type %T, want bool", getter.Get())
	return value
}

// TB is the minimal interface for test assertion support.
// Both *testing.T and *rapid.T satisfy this.
// We cannot use testing.TB because it has a private method
// that prevents external types like *rapid.T from satisfying it.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
	Logf(format string, args ...any)
}

// BasicHarness provides test assertion helpers over a [TB].
type BasicHarness interface {
	Assertf(cond bool, msg string, args ...any)
	NoErrorf(err error, msg string, args ...any)
	AssertPanicsWith(want error, f func())
	Logf(msg string, args ...any)
}

type tbHarness struct {
	tb TB
}

// NewBasic returns a [BasicHarness] wrapping tb.
func NewBasic(tb TB) BasicHarness {
	return tbHarness{tb: tb}
}

func (h tbHarness) Assertf(cond bool, msg string, args ...any) {
	h.tb.Helper()
	if !cond {
		h.tb.Fatalf(msg, args...)
	}
}

func (h tbHarness) NoErrorf(err error, msg string, args ...any) {
	h.tb.Helper()
	if err != nil {
		h.tb.Fatalf("got error: %v\n"+msg, append([]any{err}, args...)...)
	}
}

func (h tbHarness) AssertPanicsWith(want error, f func()) {
	h.tb.Helper()
	var got any
	func() {
		defer func() {
			got = recover()
		}()
		f()
	}()
	h.Assertf(got != nil, "expected panic")
	gotErr, ok := got.(error)
	h.Assertf(ok, "panic value is %T, want error", got)
	AssertSame(h, want, gotErr, "panic value")
}

func (h tbHarness) Logf(msg string, args ...any) {
	h.tb.Helper()
	h.tb.Logf(msg, args...)
}

// Harness wraps testing.T with assertion, snapshot, and test management helpers.
type Harness struct {
	BasicHarness
	t *testing.T
}

// New returns a Harness. Value receiver per GO_STYLE_GUIDE.md.
func New(t *testing.T) Harness {
	t.Helper()
	return Harness{BasicHarness: NewBasic(t), t: t}
}

// Parallel marks the test as parallel (wraps testing.T.Parallel).
func (h Harness) Parallel() {
	h.t.Helper()
	h.t.Parallel() //nolint:forbidigo // this is the wrapper
}

// Run runs a subtest with a Harness (wraps testing.T.Run).
func (h Harness) Run(name string, f func(Harness)) {
	h.t.Helper()
	h.t.Run(name, func(t *testing.T) { //nolint:forbidigo // this is the wrapper
		t.Helper()
		f(New(t))
	})
}

// T returns the underlying *testing.T.
func (h Harness) T() *testing.T {
	h.t.Helper()
	return h.t
}

// Close closes c, failing the test if closing fails.
func (h Harness) Close(c io.Closer) {
	h.t.Helper()
	h.NoErrorf(c.Close(), "closing resource")
}

// AssertSame compares want and got using cmp.Diff and fails with a diff if they differ.
// Additional cmp options may be provided to customize comparison.
func AssertSame[T any](h BasicHarness, want, got T, what string, opts ...cmp.Option) {
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		h.Assertf(false, "%s mismatch (-want +got):\n%s", what, diff)
	}
}
