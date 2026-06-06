// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package errorx

import (
	"math"
	"testing"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
)

func TestStableFormatter(t *testing.T) {
	h := check.New(t)

	h.Run("message with field", func(h check.Harness) {
		h.Parallel()

		f := NewStableFormatter()
		f.FormatConstMsg("not an absolute path")
		f.FormatDynamic(ValueKind_Path, "input", "foo\nbar")

		check.AssertSame(h, `not an absolute path (input="foo\nbar")`, f.Finish(), "stable formatter output")
	})

	h.Run("string fields", func(h check.Harness) {
		h.Parallel()

		f := NewStableFormatter()
		f.FormatConstString("message", "hello\tworld")
		f.FormatDynamic(ValueKind_Path, "custom", `custom "value"`)

		want := `(message="hello\tworld", custom="custom \"value\"")`
		check.AssertSame(h, want, f.Finish(), "stable formatter output")
	})

	h.Run("scalar fields", func(h check.Harness) {
		h.Parallel()

		f := NewStableFormatter()
		f.FormatBool("ok", true)
		f.FormatUint64("count", 42)
		f.FormatUintptr("addr", uintptr(4096))
		f.FormatInt64("offset", -7)
		f.FormatFloat64("ratio", 1.5)
		f.FormatFloat64("nan", math.NaN())

		want := "(ok=true, count=42, addr=4096, offset=-7, ratio=1.5, nan=NaN)"
		check.AssertSame(h, want, f.Finish(), "stable formatter output")
	})

	h.Run("groups", func(h check.Harness) {
		h.Parallel()

		f := NewStableFormatter()
		f.FormatGroup("outer", func(g Formatter) {
			g.FormatConstString("inner", "value")
			g.FormatGroup("empty", func(Formatter) {})
		})

		check.AssertSame(h, `(outer={(inner="value", empty={})})`, f.Finish(), "stable formatter output")
	})

	h.Run("empty keys", func(h check.Harness) {
		h.Parallel()

		f := NewStableFormatter()
		f.FormatDynamic(ValueKind_Path, "", "leading")
		f.FormatConstMsg("msg")
		f.FormatBool("", true)
		f.FormatConstString("", "const")
		f.FormatGroup("", func(g Formatter) {
			g.FormatConstMsg("inner")
			g.FormatUint64("", 3)
		})

		want := `("leading")msg (true, "const", {inner (3)})`
		check.AssertSame(h, want, f.Finish(), "stable formatter output")
	})

	h.Run("rejects unsupported value kind", func(h check.Harness) {
		h.Parallel()

		f := NewStableFormatter()
		want := assert.AssertionError{
			Fmt: "precondition violation: value kind %d is out of range [%d, %d]",
			Args: []any{
				ValueKind(-1),
				ValueKind_StdMin,
				ValueKind_StdMax,
			},
		}
		h.AssertPanicsWith(want, func() {
			f.FormatDynamic(ValueKind(-1), "custom", "custom value")
		})
	})
}
