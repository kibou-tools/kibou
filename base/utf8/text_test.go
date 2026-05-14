// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package utf8_test

import (
	"strconv"
	"strings"
	"testing"
	stdutf8 "unicode/utf8"

	"pgregory.net/rapid"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	"code.kibou.tools/base/check/benchmark"
	"code.kibou.tools/base/ranges"
	. "code.kibou.tools/base/utf8"
)

func TestText(t *testing.T) {
	h := check.New(t)
	h.Parallel()

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
			name:    "unexpected continuation byte",
			text:    string([]byte{0x80, 'x'}),
			want:    ranges.NewSpan(0, 1),
			wantErr: "invalid UTF-8 byte 0x80 at byte offset 0",
		},
		{
			name:    "overlong lead byte",
			text:    string([]byte{0xc0, 0x80}),
			want:    ranges.NewSpan(0, 1),
			wantErr: "invalid UTF-8 byte 0xC0 at byte offset 0",
		},
		{
			name:    "bad second byte in two-byte sequence",
			text:    string([]byte{0xc2, 'A'}),
			want:    ranges.NewSpan(0, 2),
			wantErr: "invalid UTF-8 bytes 0xC241 at byte span [0, 2)",
		},
		{
			name:    "truncated two-byte sequence",
			text:    string([]byte{0xc2}),
			want:    ranges.NewSpan(0, 1),
			wantErr: "invalid UTF-8 byte 0xC2 at byte offset 0",
		},
		{
			name:    "truncated three-byte sequence after second byte",
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
		{
			name:    "surrogate three-byte sequence",
			text:    string([]byte{0xed, 0xa0, 0x80}),
			want:    ranges.NewSpan(0, 2),
			wantErr: "invalid UTF-8 bytes 0xEDA0 at byte span [0, 2)",
		},
		{
			name:    "bad third byte in three-byte sequence",
			text:    string([]byte{0xe2, 0x82, 'A'}),
			want:    ranges.NewSpan(0, 3),
			wantErr: "invalid UTF-8 bytes 0xE28241 at byte span [0, 3)",
		},
		{
			name:    "bad third byte in EE-EF sequence",
			text:    string([]byte{0xee, 0x80, 'A'}),
			want:    ranges.NewSpan(0, 3),
			wantErr: "invalid UTF-8 bytes 0xEE8041 at byte span [0, 3)",
		},
		{
			name:    "bad second byte in four-byte sequence",
			text:    string([]byte{0xf0, 0x80, 0x80, 0x80}),
			want:    ranges.NewSpan(0, 2),
			wantErr: "invalid UTF-8 bytes 0xF080 at byte span [0, 2)",
		},
		{
			name:    "bad second byte in F1-F3 sequence",
			text:    string([]byte{0xf1, 'A'}),
			want:    ranges.NewSpan(0, 2),
			wantErr: "invalid UTF-8 bytes 0xF141 at byte span [0, 2)",
		},
		{
			name:    "bad second byte in F4 sequence",
			text:    string([]byte{0xf4, 0x90, 0x80, 0x80}),
			want:    ranges.NewSpan(0, 2),
			wantErr: "invalid UTF-8 bytes 0xF490 at byte span [0, 2)",
		},
		{
			name:    "truncated four-byte sequence after third byte",
			text:    string([]byte{0xf0, 0x9f, 0x98}),
			want:    ranges.NewSpan(0, 3),
			wantErr: "invalid UTF-8 bytes 0xF09F98 at byte span [0, 3)",
		},
		{
			name:    "bad third byte in four-byte sequence",
			text:    string([]byte{0xf0, 0x9f, 'A'}),
			want:    ranges.NewSpan(0, 3),
			wantErr: "invalid UTF-8 bytes 0xF09F41 at byte span [0, 3)",
		},
		{
			name:    "bad fourth byte in four-byte sequence",
			text:    string([]byte{0xf0, 0x9f, 0x98, 'A'}),
			want:    ranges.NewSpan(0, 4),
			wantErr: "invalid UTF-8 bytes 0xF09F9841 at byte span [0, 4)",
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

	cases := []string{
		"",
		"ASCII",
		"aé😀",
		"\uFFFD",
		"e\u0301",
		"👩‍💻",
		"界",
		"नमस्ते",
		"مرحبا",
		"שלום",
		"🏳️‍🌈",
		"🇺🇳",
	}
	for _, tc := range cases {
		text := MustParseText(tc)
		check.AssertSame(h, tc, text.String(), "MustParseText String")
	}
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
			want := ranges.NewSpan(0, 0)
			wantOK := false
			for start, r := range text {
				end := start + stdutf8.RuneLen(r)
				if start < offset && offset < end {
					want = ranges.NewSpan(start, end)
					wantOK = true
					break
				}
			}

			got, gotOK := parsed.CodePointContaining(offset).Get()
			check.AssertSame(h, wantOK, gotOK, "presence")
			if gotOK {
				check.AssertSame(h, 0, got.CompareStrict(want), "span")
			}
		}
	})
}

func BenchmarkParseTextVsStdlibValidString(b *testing.B) {
	for _, charCount := range []int{10, 100, 1000, 10000} {
		input := validUTF8TextWithCharCount(charCount)
		name := "chars=" + strconv.Itoa(charCount)

		b.Run(name+"/ParseText", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				text, err := ParseText(input)
				if err != nil {
					b.Fatal(err)
				}
				benchmark.BlackHole(text)
			}
		})

		b.Run(name+"/stdlib.ValidString", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				valid := stdutf8.ValidString(input)
				benchmark.BlackHole(valid)
			}
		})
	}
}

func validUTF8TextWithCharCount(charCount int) string {
	pattern := []rune{'a', 'é', '😀', '界'}
	var b strings.Builder
	b.Grow(charCount * 4)
	for i := 0; i < charCount; i++ {
		b.WriteRune(pattern[i%len(pattern)])
	}
	return b.String()
}
