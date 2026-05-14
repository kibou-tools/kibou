// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package uniseg_test

import (
	"strings"
	"testing"

	"code.kibou.tools/common/internal/uniseg"
)

func BenchmarkNewSegmentedText(b *testing.B) {
	for _, tc := range []struct {
		name string
		unit string
	}{
		{name: "ASCII", unit: "a"},
		{name: "Accented", unit: "é"},
		{name: "SimpleEmoji", unit: "🙂"},
		{name: "ComplexEmoji", unit: "👩‍💻"},
	} {
		for _, count := range []int{10, 100, 1000} {
			text := strings.Repeat(tc.unit, count)
			b.Run(tc.name+"/count="+itoa(count), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(text)))
				for b.Loop() {
					_ = uniseg.NewSegmentedText(text)
				}
			})
		}
	}
}

func itoa(n int) string {
	switch n {
	case 10:
		return "10"
	case 100:
		return "100"
	case 1000:
		return "1000"
	default:
		return "unknown"
	}
}
