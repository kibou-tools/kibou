// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package unicode_impl contains shared implementation details for Unicode
// encodings used by sibling packages.
package unicode_impl

import (
	"strconv"
	"strings"

	"code.kibou.tools/base/ranges"
)

// IsContinuationByte reports whether b is a UTF-8 continuation byte.
func IsContinuationByte(b byte) bool {
	return 0x80 <= b && b <= 0xbf
}

// InvalidBytesPrefix copies the bytes covered by span into a fixed-size value.
//
// Precondition: span is within s and span.Length() is at most four.
func InvalidBytesPrefix(s string, span ranges.Span[int]) [4]byte {
	var prefix [4]byte
	copy(prefix[:], s[span.Start():span.End()])
	return prefix
}

// FormatTextParseError formats the first invalid byte span for an encoding.
//
// Precondition: span.Length() is within [1, 4], and invalidBytes contains the
// bytes covered by span at its beginning.
func FormatTextParseError(encodingName string, span ranges.Span[int], invalidBytes [4]byte) string {
	length := span.Length().Unwrap()

	var b strings.Builder
	b.Grow(len("invalid ") + len(encodingName) + len(" bytes 0x") + 2*4 + len(" at byte span [") + 19 + len(", ") + 19 + len(")"))
	b.WriteString("invalid ")
	b.WriteString(encodingName)
	if length == 1 {
		b.WriteString(" byte 0x")
		writeInvalidBytesHex(&b, invalidBytes, 1)
		b.WriteString(" at byte offset ")
		writeInt(&b, span.Start())
		return b.String()
	}

	b.WriteString(" bytes 0x")
	writeInvalidBytesHex(&b, invalidBytes, length)
	b.WriteString(" at byte span [")
	writeInt(&b, span.Start())
	b.WriteString(", ")
	writeInt(&b, span.End())
	b.WriteByte(')')
	return b.String()
}

// Precondition: length is within [0, len(bytes)].
func writeInvalidBytesHex(b *strings.Builder, bytes [4]byte, length int) {
	const uppercaseHexDigits = "0123456789ABCDEF"
	for i := range length {
		value := bytes[i]
		b.WriteByte(uppercaseHexDigits[value>>4])
		b.WriteByte(uppercaseHexDigits[value&0x0f])
	}
}

func writeInt(b *strings.Builder, n int) {
	var buf [20]byte
	b.Write(strconv.AppendInt(buf[:0], int64(n), 10))
}
