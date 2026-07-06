// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wtf8_test

import (
	"fmt"
	"slices"
	"testing"
	"unicode/utf16"
	stdutf8 "unicode/utf8"

	"pgregory.net/rapid"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	baseutf8 "code.kibou.tools/base/utf8"
	"code.kibou.tools/base/wtf8"
)

func TestFromUTF8(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	input := baseutf8.MustParseText("aé😀")
	got := wtf8.FromUTF8(input)
	check.AssertSame(h, input.String(), got.String(), "converted text")
}

func TestParseText(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("unit", testParseTextUnit)
	h.Run("property", func(h check.Harness) {
		h.Run("valid input", testParseTextValidProperty)
		h.Run("invalid input", testParseTextInvalidProperty)
	})
}

func testParseTextUnit(h check.Harness) {
	h.Parallel()

	validCases := [][]byte{
		{},
		[]byte("aé😀"),
		{0xc2, 0x80},
		{0xe0, 0xa0, 0x80},
		{0xe1, 0x80, 0x80},
		{0xed, 0xa0, 0x80}, // lone high surrogate
		{0xed, 0xb0, 0x80}, // lone low surrogate
		{0xed, 0xa0, 0x80, 'x', 0xed, 0xb0, 0x80},
		{0xf0, 0x90, 0x80, 0x80},
		{0xf1, 0x80, 0x80, 0x80},
		{0xf4, 0x80, 0x80, 0x80},
	}
	for _, bytes := range validCases {
		got, err := wtf8.ParseText(string(bytes))
		h.Assertf(err == nil, "ParseText(% x) returned error: %v", bytes, err)
		check.AssertSame(h, string(bytes), got.String(), "parsed text")
	}

	invalidCases := []struct {
		bytes   []byte
		start   int
		end     int
		message string
	}{
		{
			bytes:   []byte{0xff},
			start:   0,
			end:     1,
			message: "invalid WTF-8 byte 0xFF at byte offset 0",
		},
		{
			bytes:   []byte{0xc2, 'A'},
			start:   0,
			end:     1,
			message: "invalid WTF-8 byte 0xC2 at byte offset 0",
		},
		{
			bytes:   []byte{0xe2, 0x82},
			start:   0,
			end:     2,
			message: "invalid WTF-8 bytes 0xE282 at byte span [0, 2)",
		},
		{
			// A surrogate pair must use the ordinary four-byte UTF-8 encoding
			// rather than two three-byte surrogate encodings.
			bytes:   []byte{0xed, 0xa0, 0x80, 0xed, 0xb0, 0x80},
			start:   3,
			end:     6,
			message: "invalid WTF-8 bytes 0xEDB080 at byte span [3, 6)",
		},
	}
	for _, tc := range invalidCases {
		_, err := wtf8.ParseText(string(tc.bytes))
		h.Assertf(err != nil, "ParseText(% x) succeeded unexpectedly", tc.bytes)
		span := err.FirstInvalidSpan()
		check.AssertSame(h, tc.start, span.Start(), "invalid span start")
		check.AssertSame(h, tc.end, span.End(), "invalid span end")
		check.AssertSame(h, tc.message, err.Error(), "error message")
	}
}

func testParseTextValidProperty(h check.Harness) {
	h.Parallel()

	gen := validWTF8BytesGen()
	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		bytes := gen.Draw(t, "bytes")

		got, err := wtf8.ParseText(string(bytes))
		h.Assertf(err == nil, "ParseText(% x) returned error: %v", bytes, err)
		check.AssertSame(h, string(bytes), got.String(), "parsed text")
	})
}

func testParseTextInvalidProperty(h check.Harness) {
	h.Parallel()

	validGen := validWTF8BytesGen()
	invalidGen := invalidWTF8FragmentGen()
	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		prefix := validGen.Draw(t, "validPrefix")
		invalid := invalidGen.Draw(t, "invalidFragment")
		input := append(append([]byte(nil), prefix...), invalid...)

		_, err := wtf8.ParseText(string(input))
		h.Assertf(err != nil, "ParseText(% x) succeeded unexpectedly", input)
		check.AssertSame(h, len(prefix), err.FirstInvalidSpan().Start(), "invalid span start")
	})
}

func TestMustParseText(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	got := wtf8.MustParseText("aé😀")
	check.AssertSame(h, "aé😀", got.String(), "parsed text")

	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		_ = wtf8.MustParseText(string([]byte{0xff}))
	}()
	h.Assertf(panicked, "MustParseText should panic for invalid WTF-8")
}

func TestText_Len(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	text := wtf8.MustParseText("aé😀")
	check.AssertSame(h, len("aé😀"), text.Len(), "byte length")
}

func TestText_GetByte(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	input := "aé😀"
	text := wtf8.MustParseText(input)
	for i := range len(input) {
		check.AssertSame(h, input[i], text.GetByte(i), fmt.Sprintf("byte %d", i))
	}
}

func TestText_Slice(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	text := wtf8.MustParseText("aé😀")
	cases := []struct {
		start int
		end   int
		want  string
	}{
		{start: 0, end: 0, want: ""},
		{start: 0, end: 1, want: "a"},
		{start: 1, end: 3, want: "é"},
		{start: 3, end: 7, want: "😀"},
		{start: 7, end: 7, want: ""},
	}
	for _, tc := range cases {
		got := text.Slice(tc.start, tc.end)
		check.AssertSame(h, tc.want, got.String(), "slice")
	}

	invalidCases := [][2]int{{-1, 0}, {1, 0}, {2, 3}, {1, 2}, {0, 8}}
	for _, bounds := range invalidCases {
		start, end := bounds[0], bounds[1]
		h.AssertPanicsWith(
			assert.AssertionError{
				Fmt:  "precondition violation: invalid WTF-8 slice [%d, %d) for text of length %d",
				Args: []any{start, end, 7},
			},
			func() { _ = text.Slice(start, end) },
		)
	}
}

func TestText_String(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	cases := []struct {
		name        string
		text        string
		wantDefault string
		wantQuoted  string
	}{
		{name: "ordinary", text: "aé", wantDefault: "aé", wantQuoted: `"aé"`},
		{name: "high surrogate", text: "\xed\xa0\x80", wantDefault: `\ud800`, wantQuoted: `"\xed\xa0\x80"`},
		{name: "low surrogate", text: "\xed\xb0\x80", wantDefault: `\udc00`, wantQuoted: `"\xed\xb0\x80"`},
		{name: "surrogate with ASCII", text: "a\xed\xa0\x80b", wantDefault: `a\ud800b`, wantQuoted: `"a\xed\xa0\x80b"`},
		{name: "separated surrogates", text: "\xed\xa0\x80x\xed\xb0\x80", wantDefault: `\ud800x\udc00`, wantQuoted: `"\xed\xa0\x80x\xed\xb0\x80"`},
	}
	for _, tc := range cases {
		text := wtf8.MustParseText(tc.text)
		check.AssertSame(h, tc.wantDefault, fmt.Sprintf("%v", text), tc.name+" %v")
		check.AssertSame(h, tc.wantQuoted, fmt.Sprintf("%q", text), tc.name+" %q")
		check.AssertSame(h, tc.wantQuoted, fmt.Sprintf("%#v", text), tc.name+" %#v")
	}
}

func TestTextBuilder_WriteText(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("unit", testTextBuilder_WriteTextUnit)
	h.Run("property", testTextBuilder_WriteTextProperty)
}

func testTextBuilder_WriteTextUnit(h check.Harness) {
	h.Parallel()

	high := string([]byte{0xed, 0xa0, 0x80})
	low := string([]byte{0xed, 0xb0, 0x80})
	nonHighED := string([]byte{0xed, 0x80, 0x80})
	afterLowRange := string([]byte{0xee, 0x80, 0x80})
	supplementary := string([]byte{0xf0, 0x90, 0x80, 0x80})
	cases := []struct {
		name  string
		left  string
		right string
		want  string
	}{
		{name: "ordinary", left: "foo", right: "bar", want: "foobar"},
		{name: "empty right", left: "foo", right: "", want: "foo"},
		{name: "high then low", left: high, right: low, want: supplementary},
		{name: "high then ASCII", left: high, right: "x", want: high + "x"},
		{name: "low then ASCII", left: low, right: "x", want: low + "x"},
		{name: "non-high ed sequence", left: nonHighED, right: "x", want: nonHighED + "x"},
		{name: "high then code point above low range", left: high, right: afterLowRange, want: high + afterLowRange},
	}
	for _, tc := range cases {
		builder := wtf8.NewTextBuilder(len(tc.left) + len(tc.right))
		builder.WriteText(mustParseBytes(h, []byte(tc.left)))
		builder.WriteText(mustParseBytes(h, []byte(tc.right)))
		check.AssertSame(h, tc.want, builder.Text().String(), tc.name)
	}
}

func testTextBuilder_WriteTextProperty(h check.Harness) {
	h.Parallel()

	gen := generalizedCodePointGen()
	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		leftCodePoint := gen.Draw(t, "left")
		rightCodePoint := gen.Draw(t, "right")
		leftBytes := appendGeneralizedUTF8(nil, leftCodePoint)
		rightBytes := appendGeneralizedUTF8(nil, rightCodePoint)
		left := mustParseBytes(h, leftBytes)
		right := mustParseBytes(h, rightBytes)

		var builder wtf8.TextBuilder
		builder.WriteText(left)
		builder.WriteText(right)
		got := builder.Text().String()
		_, gotErr := wtf8.ParseText(got)
		h.Assertf(gotErr == nil, "WriteText produced invalid WTF-8: % x: %v", []byte(got), gotErr)

		if supplementary := utf16.DecodeRune(leftCodePoint, rightCodePoint); supplementary != stdutf8.RuneError {
			want := string(stdutf8.AppendRune(nil, supplementary))
			check.AssertSame(h, want, got, "joined surrogate pair")
		} else {
			want := string(slices.Concat(leftBytes, rightBytes))
			check.AssertSame(h, want, got, "ordinary concatenation")
		}
	})
}

func TestTextBuilder_WriteASCIIByte(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	var builder wtf8.TextBuilder
	builder.WriteASCIIByte(0)
	builder.WriteASCIIByte(0x7f)
	check.AssertSame(h, string([]byte{0, 0x7f}), builder.Text().String(), "ASCII bytes")

	h.AssertPanicsWith(
		assert.AssertionError{
			Fmt:  "precondition violation: wtf8.TextBuilder.WriteASCIIByte: 0x%02x is not ASCII",
			Args: []any{byte(0x80)},
		},
		func() { builder.WriteASCIIByte(0x80) },
	)
}

// mustParseBytes parses raw WTF-8 bytes into a Text.
func mustParseBytes(h check.BasicHarness, bytes []byte) wtf8.Text {
	text, err := wtf8.ParseText(string(bytes))
	h.Assertf(err == nil, "ParseText(% x) returned error: %v", bytes, err)
	return text
}

type codePointRange struct {
	min int32
	max int32
}

// generalizedCodePointGen samples the code-point ranges corresponding to the
// byte-sequence classes in Table 3 of the generalized UTF-8 specification.
// Explicit ranges keep the smaller classes from being overwhelmed by the
// supplementary code-point range.
func generalizedCodePointGen() *rapid.Generator[rune] {
	rangeGen := rapid.SampledFrom([]codePointRange{
		// One-byte, two-byte, and e0 three-byte sequences.
		{min: 0, max: 0x7f},
		{min: 0x80, max: 0x7ff},
		{min: 0x800, max: 0xfff},

		// The e1–ef class.
		{min: 0x1000, max: 0xd7ff},
		{min: 0xd800, max: 0xdbff}, // high surrogates
		{min: 0xdc00, max: 0xdfff}, // low surrogates
		{min: 0xe000, max: 0xffff},

		// The three four-byte sequence classes.
		{min: 0x10000, max: 0x3ffff},
		{min: 0x40000, max: 0xfffff},
		{min: 0x100000, max: 0x10ffff},
	})
	return rapid.Custom(func(t *rapid.T) rune {
		bounds := rangeGen.Draw(t, "range")
		return rapid.Int32Range(bounds.min, bounds.max).Draw(t, "value")
	})
}

func validWTF8BytesGen() *rapid.Generator[[]byte] {
	codePointsGen := rapid.SliceOfN(generalizedCodePointGen(), 0, 64)
	return rapid.Custom(func(t *rapid.T) []byte {
		codePoints := codePointsGen.Draw(t, "codePoints")
		bytes := make([]byte, 0, 4*len(codePoints))
		for i, codePoint := range codePoints {
			bytes = appendGeneralizedUTF8(bytes, codePoint)
			if i+1 < len(codePoints) {
				// Separating generated code points prevents a high-low surrogate pair.
				bytes = append(bytes, 0)
			}
		}
		return bytes
	})
}

func invalidWTF8FragmentGen() *rapid.Generator[[]byte] {
	return rapid.SampledFrom([][]byte{
		// Invalid first bytes: a continuation byte, the highest overlong two-byte
		// prefix, and a prefix above the maximum valid code point. The trailing
		// ASCII byte in the last case exercises the four-byte invalid-span limit.
		{0x80},
		{0xc1},
		{0xf5, 0x80, 0x80, 0x80, 'x'},

		// Truncated two-byte sequence.
		{0xc2},

		// e0 sequence: truncated input, second byte below or above its restricted
		// range, and an invalid third byte.
		{0xe0},
		{0xe0, 0x9f, 0x80},
		{0xe0, 0xc0, 0x80},
		{0xe0, 0xa0, 0x7f},

		// e1–ef sequence: each invalid continuation position. Truncation is
		// already covered by the unit cases.
		{0xe1, 0x7f, 0x80},
		{0xe1, 0x80, 0x7f},

		// f0 sequence: truncated input, second byte below or above its restricted
		// range, and each remaining invalid continuation position.
		{0xf0},
		{0xf0, 0x8f, 0x80, 0x80},
		{0xf0, 0xc0, 0x80, 0x80},
		{0xf0, 0x90, 0x7f, 0x80},
		{0xf0, 0x90, 0x80, 0x7f},

		// f1–f3 sequence: truncated input and each invalid continuation position.
		{0xf1},
		{0xf1, 0x7f, 0x80, 0x80},
		{0xf1, 0x80, 0x7f, 0x80},
		{0xf1, 0x80, 0x80, 0x7f},

		// f4 sequence: truncated input, second byte below or above its restricted
		// range, and each remaining invalid continuation position.
		{0xf4},
		{0xf4, 0x7f, 0x80, 0x80},
		{0xf4, 0x90, 0x80, 0x80},
		{0xf4, 0x80, 0x7f, 0x80},
		{0xf4, 0x80, 0x80, 0x7f},
	})
}

// appendGeneralizedUTF8 appends the generalized UTF-8 encoding of codePoint.
func appendGeneralizedUTF8(bytes []byte, codePoint rune) []byte {
	// The standard library handles scalar values; only surrogates need encoding here.
	if !utf16.IsSurrogate(codePoint) {
		return stdutf8.AppendRune(bytes, codePoint)
	}
	return append(bytes,
		byte(0xe0|codePoint>>12),
		byte(0x80|(codePoint>>6)&0x3f),
		byte(0x80|codePoint&0x3f),
	)
}
