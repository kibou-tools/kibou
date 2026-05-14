// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package utf8_test

import (
	"testing"

	"code.kibou.tools/base/check"
	. "code.kibou.tools/base/utf8"
)

func TestTryDecodeFirstRune(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	check.AssertSame(h, '�', ReplacementChar, "ReplacementChar")

	decoded := TryDecodeFirstRune("éx")
	check.AssertSame(h, RuneDecodingResultKind_Valid, decoded.Kind(), "valid kind")
	check.AssertSame(h, 'é', decoded.Rune(), "valid rune")
	check.AssertSame(h, 2, decoded.ByteLen(), "valid size")

	decoded = TryDecodeFirstRune(string([]byte{0xff}))
	check.AssertSame(h, RuneDecodingResultKind_Invalid, decoded.Kind(), "invalid kind")
	check.AssertSame(h, 1, decoded.ByteLen(), "invalid size")

	decoded = TryDecodeFirstRune("")
	check.AssertSame(h, RuneDecodingResultKind_Empty, decoded.Kind(), "empty kind")
	check.AssertSame(h, 0, decoded.ByteLen(), "empty size")

	decoded = TryDecodeFirstRune("\uFFFD")
	check.AssertSame(h, RuneDecodingResultKind_Valid, decoded.Kind(), "replacement kind")
	check.AssertSame(h, ReplacementChar, decoded.Rune(), "replacement rune")
	check.AssertSame(h, 3, decoded.ByteLen(), "replacement size")
}

func TestRuneLen(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	check.AssertSame(h, 1, RuneLen('a'), "ASCII")
	check.AssertSame(h, 2, RuneLen('é'), "two-byte")
	check.AssertSame(h, 4, RuneLen('😀'), "four-byte")
}

func TestIsPotentialStartOfRune(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	check.AssertSame(h, true, IsPotentialStartOfRune('a'), "ASCII")
	check.AssertSame(h, true, IsPotentialStartOfRune(0xc3), "leading byte")
	check.AssertSame(h, false, IsPotentialStartOfRune(0xa9), "continuation byte")
}
