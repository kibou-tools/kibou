// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package utf8 provides UTF-8 helpers used by Kibou code.
package utf8

import (
	stdutf8 "unicode/utf8"

	"code.kibou.tools/base/assert"
	. "code.kibou.tools/base/core/option"
	"code.kibou.tools/base/ranges"
)

// RuneError is the Unicode replacement character.
const RuneError = stdutf8.RuneError

// TryDecodeFirstRune tries to decode the first rune in s.
//
// The result kind is:
//   - RuneDecodingResultKind_Valid if s starts with a valid UTF-8 encoding.
//   - RuneDecodingResultKind_Invalid if s starts with invalid UTF-8.
//   - RuneDecodingResultKind_Empty if s is empty.
func TryDecodeFirstRune(s string) RuneDecodingResult {
	r, size := stdutf8.DecodeRuneInString(s)
	return RuneDecodingResult{r: r, byteLen: uint8(size)}
}

// RuneDecodingResult is the result of trying to decode a rune.
type RuneDecodingResult struct {
	r       rune
	byteLen uint8
}

type RuneDecodingResultKind uint8

const (
	RuneDecodingResultKind_Valid RuneDecodingResultKind = iota + 1
	RuneDecodingResultKind_Invalid
	RuneDecodingResultKind_Empty
)

// Rune returns the decoded rune.
//
// Precondition: Kind() == RuneDecodingResultKind_Valid.
func (r RuneDecodingResult) Rune() rune {
	if r.Kind() != RuneDecodingResultKind_Valid {
		assert.Precondition(false, "RuneDecodingResult.Rune requires a valid decoding result")
	}
	return r.r
}

// ByteLen returns the number of bytes read while decoding.
func (r RuneDecodingResult) ByteLen() int {
	return int(r.byteLen)
}

// Kind returns the decoding outcome.
func (r RuneDecodingResult) Kind() RuneDecodingResultKind {
	if r.byteLen == 0 {
		return RuneDecodingResultKind_Empty
	}
	if r.r == RuneError && r.byteLen == 1 {
		return RuneDecodingResultKind_Invalid
	}
	return RuneDecodingResultKind_Valid
}

// PrefixClassification describes how a byte sequence relates to UTF-8 encodings.
type PrefixClassification uint8

const (
	// PrefixClassification_Complete means the bytes are exactly one valid UTF-8
	// encoding.
	PrefixClassification_Complete PrefixClassification = iota + 1
	// PrefixClassification_Partial means that the bytes can form an incomplete
	// prefix of a valid UTF-8 encoded codepoint.
	PrefixClassification_Partial
	// PrefixClassification_Malformed means the bytes are neither a complete UTF-8
	// encoding nor a proper prefix of one.
	PrefixClassification_Malformed
)

// ClassifyPrefix classifies bytes as a complete UTF-8 encoding, a proper
// prefix of one, or malformed bytes.
//
// Precondition: len(bytes) ∈ [1, 4].
func ClassifyPrefix(bytes []byte) PrefixClassification {
	length := len(bytes)
	if length < 1 || 4 < length {
		assert.Preconditionf(false, "UTF-8 prefix length should be in [1, 4] but got %d", length)
	}

	if !stdutf8.FullRune(bytes) {
		return PrefixClassification_Partial
	}

	r, size := stdutf8.DecodeRune(bytes)
	if r == RuneError && size == 1 {
		return PrefixClassification_Malformed
	}
	if size == length {
		return PrefixClassification_Complete
	}
	return PrefixClassification_Malformed
}

// RuneLen returns the number of bytes required to encode r in UTF-8.
func RuneLen(r rune) int {
	return stdutf8.RuneLen(r)
}

// IsPotentialStartOfRune reports whether b could be the first byte of an encoded rune.
func IsPotentialStartOfRune(b byte) bool {
	return stdutf8.RuneStart(b)
}

func firstInvalidSpan(text string) Option[ranges.Span[int]] {
	for i := 0; i < len(text); {
		r, size := stdutf8.DecodeRuneInString(text[i:])
		if r != RuneError || size != 1 {
			i += size
			continue
		}

		end, kind := invalidPrefixSpanEnd(text, i)
		switch kind {
		case PrefixClassification_Partial, PrefixClassification_Malformed:
			return Some(ranges.NewSpan(i, end))
		case PrefixClassification_Complete:
			assert.Invariant(false, "DecodeRuneInString reported invalid UTF-8 for a complete prefix")
		default:
			assert.PanicUnknownCase[any](kind)
		}
	}
	return None[ranges.Span[int]]()
}

func invalidPrefixSpanEnd(text string, start int) (int, PrefixClassification) {
	limit := min(len(text), start+4)
	var prefix [4]byte
	for end := start + 1; end <= limit; end++ {
		length := end - start
		prefix[length-1] = text[end-1]
		kind := ClassifyPrefix(prefix[:length])
		if kind != PrefixClassification_Partial {
			return end, kind
		}
	}
	return len(text), PrefixClassification_Partial
}
