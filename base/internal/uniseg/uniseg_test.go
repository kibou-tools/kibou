// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package uniseg_test

import (
	"math"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/check"
	pbt_text "code.kibou.tools/base/check/pbt/text"
	"code.kibou.tools/base/internal/uniseg"
	"code.kibou.tools/base/ranges"
	"code.kibou.tools/base/utf8"
)

func TestContainingGraphemeCluster_CanExceedMaxInt16(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	text := utf8.MustParseText("a" + strings.Repeat("\u0301", 1<<14))
	segmented := uniseg.NewSegmentedText(text)

	err := segmented.CheckSpan(ranges.NewSpan(1, 1))
	h.Assertf(err != nil, "span inside grapheme cluster should fail boundary check")
	check.AssertSame(h, uniseg.SpanBoundaryErrorKind_NotGraphemeBoundary, err.Kind(), "error kind")

	window, ok := segmented.FindGraphemeClusterContaining(err.ByteOffset(), math.MaxInt16).Get()
	h.Assertf(ok, "expected containing grapheme cluster")
	check.AssertSame(h, true, window.IsFullCluster(), "full cluster")
	length := window.Span().Length().Unwrap()
	h.Assertf(length > math.MaxInt16, "containing grapheme cluster length %d <= MaxInt16", length)
}

func TestFindGraphemeClusterContaining(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	clusterCountGen := rapid.IntRange(1, 20)
	maxCodePointsGen := rapid.IntRange(1, 32)
	rapid.Check(h.T(), func(t *rapid.T) {
		h := check.NewBasic(t)
		clusterCount := clusterCountGen.Draw(t, "clusterCount")                 // make N grapheme clusters
		targetIndex := rapid.IntRange(0, clusterCount-1).Draw(t, "targetIndex") // pick one of them
		clusters := make([]string, 0, clusterCount)
		targetStart := 0
		var target string
		for i := 0; i < clusterCount; i++ {
			cluster := pbt_text.GraphemeCluster().Draw(t, "cluster"+itoa(i))
			if i == targetIndex {
				// FindGraphemeClusterContaining is meant to test the situation where you're
				// pointing to a code point in the middle of a multi-codepoint grapheme cluster,
				// so force the use of a multi-codepoint generator here.
				cluster = pbt_text.MultiCodePointGraphemeCluster().Draw(t, "target")
				target = cluster
			}
			if i < targetIndex {
				targetStart += len(cluster)
			}
			clusters = append(clusters, cluster)
		}
		maxCodePoints := maxCodePointsGen.Draw(t, "maxCodePoints")
		targetCodePointStarts := codePointStartOffsets(target)
		targetCodePointCount := len(targetCodePointStarts)
		assert.Invariantf(targetCodePointCount >= 2,
			"used pbt_text.MultiCodePointGraphemeCluster earlier, but got grapheme cluster %q with %d codepoints",
			target, targetCodePointCount)

		offsetCodePointIndex := rapid.IntRange(1, targetCodePointCount-1).Draw(t, "offsetCodePointIndex")

		text := utf8.MustParseText(strings.Join(clusters, ""))
		segmented := uniseg.NewSegmentedText(text)
		targetSpan := ranges.NewSpan(targetStart, targetStart+len(target))
		offset := targetStart + targetCodePointStarts[offsetCodePointIndex]

		window, ok := segmented.FindGraphemeClusterContaining(offset, maxCodePoints).Get()
		h.Assertf(ok, "expected grapheme cluster containing offset %d", offset)
		span := window.Span()
		h.Assertf(targetSpan.Start() <= span.Start(), "window start %d before cluster start %d", span.Start(), targetSpan.Start())
		h.Assertf(span.Start() <= offset, "window start %d after offset %d", span.Start(), offset)
		h.Assertf(offset < span.End(), "window end %d not after offset %d", span.End(), offset)
		h.Assertf(span.End() <= targetSpan.End(), "window end %d after cluster end %d", span.End(), targetSpan.End())
		assertCodePointBoundary(h, text, span.Start(), "window start")
		assertCodePointBoundary(h, text, span.End(), "window end")

		windowCodePoints := len(codePointStartOffsets(text.String()[span.Start():span.End()]))
		h.Assertf(windowCodePoints <= maxCodePoints, "window has %d codepoints, max %d", windowCodePoints, maxCodePoints)

		if targetCodePointCount <= maxCodePoints {
			check.AssertSame(h, true, window.IsFullCluster(), "full cluster")
			check.AssertSame(h, 0, span.CompareStrict(targetSpan), "full cluster span")
		} else {
			check.AssertSame(h, false, window.IsFullCluster(), "truncated cluster")
		}
	})
}

func codePointStartOffsets(text string) []int {
	starts := make([]int, 0, len(text))
	// This may be a bit confusing, but it actually does return byte offsets
	// for codepoints, not a uniformly increasing sequence [0, 1, 2, ...].
	for i := range text {
		starts = append(starts, i)
	}
	return starts
}

func assertCodePointBoundary(h check.BasicHarness, text utf8.Text, offset int, name string) {
	h.Assertf(offset == text.Len() || utf8.IsPotentialStartOfRune(text.GetByte(offset)),
		"%s is not a codepoint boundary: %d", name, offset)
}

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
			text := utf8.MustParseText(strings.Repeat(tc.unit, count))
			b.Run(tc.name+"/count="+itoa(count), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(text.Len()))
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
