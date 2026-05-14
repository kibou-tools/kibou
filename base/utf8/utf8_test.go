// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package utf8_test

import (
	"strings"
	"testing"
	stdutf8 "unicode/utf8"

	"pgregory.net/rapid"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	"code.kibou.tools/base/ranges"
	. "code.kibou.tools/base/utf8"
)

func TestUTF8(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("Rune helpers", testRuneHelpers)
	h.Run("ParseText", func(h check.Harness) {
		h.Run("unit", testParseTextUnit)
		h.Run("property", testParseTextProperty)
	})
	h.Run("MustParseText", testMustParseText)
	h.Run("Text.CodePointContaining", func(h check.Harness) {
		h.Run("unit", testCodePointContainingUnit)
		h.Run("property", testCodePointContainingProperty)
	})
}

func testRuneHelpers(h check.Harness) {
	h.Parallel()

	check.AssertSame(h, '�', RuneError, "RuneError")

	check.AssertSame(h, 1, RuneLen('a'), "RuneLen ASCII")
	check.AssertSame(h, 2, RuneLen('é'), "RuneLen two-byte")
	check.AssertSame(h, 4, RuneLen('😀'), "RuneLen four-byte")

	decoded := TryDecodeFirstRune("éx")
	check.AssertSame(h, RuneDecodingResultKind_Valid, decoded.Kind(), "TryDecodeFirstRune kind")
	check.AssertSame(h, 'é', decoded.Rune(), "TryDecodeFirstRune rune")
	check.AssertSame(h, 2, decoded.ByteLen(), "TryDecodeFirstRune size")

	decoded = TryDecodeFirstRune(string([]byte{0xff}))
	check.AssertSame(h, RuneDecodingResultKind_Invalid, decoded.Kind(), "TryDecodeFirstRune invalid kind")
	check.AssertSame(h, 1, decoded.ByteLen(), "TryDecodeFirstRune invalid size")

	decoded = TryDecodeFirstRune("")
	check.AssertSame(h, RuneDecodingResultKind_Empty, decoded.Kind(), "TryDecodeFirstRune empty kind")
	check.AssertSame(h, 0, decoded.ByteLen(), "TryDecodeFirstRune empty size")

	decoded = TryDecodeFirstRune("\uFFFD")
	check.AssertSame(h, RuneDecodingResultKind_Valid, decoded.Kind(), "TryDecodeFirstRune replacement kind")
	check.AssertSame(h, RuneError, decoded.Rune(), "TryDecodeFirstRune replacement rune")
	check.AssertSame(h, 3, decoded.ByteLen(), "TryDecodeFirstRune replacement size")

	check.AssertSame(h, true, IsPotentialStartOfRune('a'), "IsPotentialStartOfRune ASCII")
	check.AssertSame(h, true, IsPotentialStartOfRune(0xc3), "IsPotentialStartOfRune leading byte")
	check.AssertSame(h, false, IsPotentialStartOfRune(0xa9), "IsPotentialStartOfRune continuation byte")
}

func testParseTextUnit(h check.Harness) {
	h.Parallel()

	valid, err := ParseText("aé😀")
	check.AssertSame(h, (*TextParseError)(nil), err, "valid text error")
	check.AssertSame(h, "aé😀", valid.String(), "valid text String")

	cases := []struct {
		name    string
		text    string
		want    ranges.Span[int]
		wantErr string
	}{
		{
			name:    "single invalid byte",
			text:    string([]byte{'a', 0xff, 'b'}),
			want:    ranges.NewSpan(1, 2),
			wantErr: "invalid UTF-8 byte 0xFF at byte offset 1",
		},
		{
			name:    "consecutive invalid bytes",
			text:    string([]byte{0xff, 0xfe, 'x'}),
			want:    ranges.NewSpan(0, 1),
			wantErr: "invalid UTF-8 byte 0xFF at byte offset 0",
		},
		{
			name:    "invalid after multibyte",
			text:    string([]byte{0xc3, 0xa9, 0xff, 0xfe}),
			want:    ranges.NewSpan(2, 3),
			wantErr: "invalid UTF-8 byte 0xFF at byte offset 2",
		},
		{
			name:    "long invalid byte run",
			text:    string([]byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb}),
			want:    ranges.NewSpan(0, 1),
			wantErr: "invalid UTF-8 byte 0xFF at byte offset 0",
		},
		{
			name:    "truncated three-byte sequence",
			text:    string([]byte{0xe2, 0x82}),
			want:    ranges.NewSpan(0, 2),
			wantErr: "invalid UTF-8 bytes 0xE282 at byte span [0, 2)",
		},
		{
			name:    "overlong three-byte sequence",
			text:    string([]byte{0xe0, 0x80, 0x80}),
			want:    ranges.NewSpan(0, 2),
			wantErr: "invalid UTF-8 bytes 0xE080 at byte span [0, 2)",
		},
	}

	for _, tc := range cases {
		_, err := ParseText(tc.text)
		h.Assertf(err != nil, "%s: expected parse error", tc.name)
		check.AssertSame(h, 0, err.FirstInvalidSpan().CompareStrict(tc.want), tc.name+" first invalid span")
		check.AssertSame(h, tc.wantErr, err.Error(), tc.name+" error")
	}
}

func testParseTextProperty(h check.Harness) {
	h.Parallel()

	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		bytes := rapid.SliceOfN(rapid.Uint8(), 0, 64).Draw(t, "bytes")
		text := string(bytes)

		parsed, err := ParseText(text)
		wantValid := stdutf8.ValidString(text)
		check.AssertSame(h, wantValid, err == nil, "ParseText validity")
		if wantValid {
			check.AssertSame(h, text, parsed.String(), "parsed text")
			return
		}

		h.Assertf(err != nil, "invalid UTF-8 should return a TextParseError")
		span := err.FirstInvalidSpan()
		length := span.Length().Unwrap()
		h.Assertf(0 <= span.Start(), "span start %d before 0", span.Start())
		h.Assertf(span.End() <= len(text), "span end %d after text length %d", span.End(), len(text))
		h.Assertf(1 <= length && length <= 4, "span length %d outside [1, 4]", length)
		check.AssertSame(h, true, stdutf8.ValidString(text[:span.Start()]), "prefix before invalid span is valid")
		check.AssertSame(h, false, stdutf8.ValidString(text[:span.End()]), "prefix through invalid span is invalid")
		h.Assertf(err.Error() != "", "error message should be non-empty")
	})
}

func testMustParseText(h check.Harness) {
	h.Parallel()

	text := MustParseText("aé😀")
	check.AssertSame(h, "aé😀", text.String(), "MustParseText String")
}

func testCodePointContainingUnit(h check.Harness) {
	h.Parallel()

	text := MustParseText("aé😀")
	cases := []struct {
		name string
		off  int
		want ranges.Span[int]
		ok   bool
	}{
		{name: "start", off: 0, want: ranges.NewSpan(0, 0), ok: false},
		{name: "between ASCII and two-byte", off: 1, want: ranges.NewSpan(0, 0), ok: false},
		{name: "inside two-byte", off: 2, want: ranges.NewSpan(1, 3), ok: true},
		{name: "between two-byte and four-byte", off: 3, want: ranges.NewSpan(0, 0), ok: false},
		{name: "inside four-byte first continuation", off: 4, want: ranges.NewSpan(3, 7), ok: true},
		{name: "inside four-byte second continuation", off: 5, want: ranges.NewSpan(3, 7), ok: true},
		{name: "inside four-byte third continuation", off: 6, want: ranges.NewSpan(3, 7), ok: true},
		{name: "end", off: 7, want: ranges.NewSpan(0, 0), ok: false},
	}

	for _, tc := range cases {
		got, ok := text.CodePointContaining(tc.off).Get()
		check.AssertSame(h, tc.ok, ok, tc.name+" present")
		if ok {
			check.AssertSame(h, 0, got.CompareStrict(tc.want), tc.name+" span")
		}
	}

	h.AssertPanicsWith(
		assert.AssertionError{Fmt: "precondition violation: offset %d outside text bounds [0, %d]", Args: []any{-1, 7}},
		func() { _ = text.CodePointContaining(-1) },
	)
	h.AssertPanicsWith(
		assert.AssertionError{Fmt: "precondition violation: offset %d outside text bounds [0, %d]", Args: []any{8, 7}},
		func() { _ = text.CodePointContaining(8) },
	)
}

func testCodePointContainingProperty(h check.Harness) {
	h.Parallel()

	partsGen := rapid.SliceOfN(rapid.SampledFrom([]string{"", "a", "é", "😀", "\u0301", "\uFFFD", "👩‍💻"}), 0, 40)
	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		parts := partsGen.Draw(t, "parts")
		text := strings.Join(parts, "")
		parsed := MustParseText(text)

		for offset := 0; offset <= len(text); offset++ {
			want, wantOK := codePointContainingReference(text, offset)
			got, gotOK := parsed.CodePointContaining(offset).Get()
			check.AssertSame(h, wantOK, gotOK, "presence")
			if gotOK {
				check.AssertSame(h, 0, got.CompareStrict(want), "span")
			}
		}
	})
}

func codePointContainingReference(text string, offset int) (ranges.Span[int], bool) {
	for start, r := range text {
		end := start + stdutf8.RuneLen(r)
		if start < offset && offset < end {
			return ranges.NewSpan(start, end), true
		}
	}
	return ranges.NewSpan(0, 0), false
}
