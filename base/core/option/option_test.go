// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package option_test

import (
	"encoding/json"
	"testing"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	. "code.kibou.tools/base/core/option"
)

func TestOption(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("Some", func(h check.Harness) {
		h.Parallel()

		opt := Some(42)
		h.Assertf(opt.IsSome(), "IsSome after Some(..)")
		h.Assertf(!opt.IsNone(), "Some => !None")
		v, ok := opt.Get()
		h.Assertf(ok && v == 42, "Get() = (%d, %v), want (42, true)", v, ok)
	})

	h.Run("None", func(h check.Harness) {
		h.Parallel()

		opt := None[int]()
		h.Assertf(opt.IsNone(), "None() => IsNone")
		h.Assertf(!opt.IsSome(), "None => !IsSome")
		_, ok := opt.Get()
		h.Assertf(!ok, "Get() on None should return false")
	})

	h.Run("Unwrap", func(h check.Harness) {
		h.Parallel()

		h.Assertf(Some(42).Unwrap() == 42, "Some(42).Unwrap() = %d, want 42", Some(42).Unwrap())
		want := assert.AssertionError{Fmt: "precondition violation: called Unwrap on None", Args: nil}
		h.AssertPanicsWith(want, func() {
			_ = None[int]().Unwrap()
		})
	})

	h.Run("Expect", func(h check.Harness) {
		h.Parallel()
		h.Assertf(Some(42).Expect("expected value") == 42, "Some(42).Expect(...) = %d, want 42", Some(42).Expect("expected value"))
		want := assert.AssertionError{Fmt: "invariant violation: %s", Args: []any{"expected value"}}
		h.AssertPanicsWith(want, func() {
			_ = None[int]().Expect("expected value")
		})
	})

	h.Run("ValueOr", func(h check.Harness) {
		h.Parallel()

		some := Some(10)
		h.Assertf(some.ValueOr(99) == 10, "Some(10).ValueOr(99) = %d, want 10", some.ValueOr(99))
		none := None[int]()
		h.Assertf(none.ValueOr(99) == 99, "None().ValueOr(99) = %d, want 99", none.ValueOr(99))
	})

	h.Run("Compare", func(h check.Harness) {
		h.Parallel()

		// Both Some: delegates to cmp.Compare on inner values.
		h.Assertf(Compare(Some(1), Some(2)) < 0, "Some(1) < Some(2)")
		h.Assertf(Compare(Some(2), Some(2)) == 0, "Some(2) == Some(2)")
		h.Assertf(Compare(Some(3), Some(2)) > 0, "Some(3) > Some(2)")

		// Both None: equal.
		h.Assertf(Compare(None[int](), None[int]()) == 0, "None == None")

		// None < Some (absent values sort before present).
		h.Assertf(Compare(None[int](), Some(0)) < 0, "None < Some(0)")
		h.Assertf(Compare(Some(0), None[int]()) > 0, "Some(0) > None")
	})

	h.Run("NewOption", func(h check.Harness) {
		h.Parallel()

		some := NewOption("hello", true)
		h.Assertf(some.IsSome(), "NewOption with ok=true should be Some")
		none := NewOption("hello", false)
		h.Assertf(none.IsNone(), "NewOption with ok=false should be None")
	})

	h.Run("JSON", func(h check.Harness) {
		h.Parallel()

		// Each case round-trips: Some encodes its inner value, None encodes
		// as null, and decoding inverts both.
		testCases := []struct {
			name string
			opt  Option[int]
			json string
		}{
			{name: "some", opt: Some(42), json: "42"},
			{name: "some zero", opt: Some(0), json: "0"},
			{name: "none", opt: None[int](), json: "null"},
		}
		for _, tc := range testCases {
			h.Run(tc.name, func(h check.Harness) {
				h.Parallel()

				got, err := json.Marshal(tc.opt)
				h.NoErrorf(err, "marshal %v", tc.opt)
				h.Assertf(string(got) == tc.json, "marshal %v = %s, want %s", tc.opt, got, tc.json)

				var decoded Option[int]
				h.NoErrorf(json.Unmarshal([]byte(tc.json), &decoded), "unmarshal %s", tc.json)
				h.Assertf(Compare(decoded, tc.opt) == 0, "unmarshal %s = %v, want %v", tc.json, decoded, tc.opt)
			})
		}

		// An absent field decodes to None, because encoding/json does not call
		// UnmarshalJSON for missing keys.
		h.Run("absent field", func(h check.Harness) {
			h.Parallel()

			var s struct {
				A Option[int] `json:"a"`
				B Option[int] `json:"b"`
			}
			h.NoErrorf(json.Unmarshal([]byte(`{"a":7}`), &s), "unmarshal struct")
			h.Assertf(Compare(s.A, Some(7)) == 0, "present field a = %v, want Some(7)", s.A)
			h.Assertf(s.B.IsNone(), "absent field b => None")
		})
	})
}
