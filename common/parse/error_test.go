// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package parse

import (
	"testing"

	"code.kibou.tools/common/check"
)

func TestError(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("ExpectedWithContextAndEOF", func(h check.Harness) {
		h.Parallel()
		err := NewError("POSIX TZ", "EST5:3", ErrorOptions{
			Parsed:   "EST5:3",
			Context:  "standard time offset",
			Expected: "two minute digits ([0-9]{2})",
			Reason:   "",
		})
		want := `failed to parse POSIX TZ "EST5:3": hit error after parsing "EST5:3": context - trying to parse standard time offset, expected - two minute digits ([0-9]{2}), next char - EOF`
		check.AssertSame(h, want, err.Error(), "Error()")
	})

	h.Run("ExpectedWithNextChar", func(h check.Harness) {
		h.Parallel()
		err := NewError("POSIX TZ", "EST5,J", ErrorOptions{
			Parsed:   "EST5",
			Context:  "",
			Expected: "end of input",
			Reason:   "",
		})
		want := `failed to parse POSIX TZ "EST5,J": hit error after parsing "EST5", expected - end of input, next char - ','`
		check.AssertSame(h, want, err.Error(), "Error()")
	})

	h.Run("Reason", func(h check.Harness) {
		h.Parallel()
		err := NewError("POSIX TZ", string([]byte{0xff}), ErrorOptions{
			Parsed:   "",
			Context:  "",
			Expected: "",
			Reason:   "invalid UTF-8",
		})
		want := "failed to parse POSIX TZ \"\\xff\": invalid UTF-8"
		check.AssertSame(h, want, err.Error(), "Error()")
	})
}
