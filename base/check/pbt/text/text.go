// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package text provides property-test generators for interesting text.
package text

import (
	"strconv"
	"strings"

	"pgregory.net/rapid"

	"code.kibou.tools/base/assert"
)

// GraphemeCluster generates valid UTF-8 strings that are intended to form a
// single extended grapheme cluster.
func GraphemeCluster() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		kind := rapid.SampledFrom([]string{"single", "multi"}).Draw(t, "clusterKind")
		switch kind {
		case "single":
			return rapid.SampledFrom([]string{
				"\u0000",     // NUL
				"\u0061",     // a
				"\u00E9",     // é
				"\u754C",     // 界
				"\U0001F642", // 🙂
				"\U0001F684", // 🚄
			}).Draw(t, "normalCluster")
		case "multi":
			return MultiCodePointGraphemeCluster().Draw(t, "multiCluster")
		default:
			assert.Invariant(false, "unknown cluster kind")
			return ""
		}
	})
}

// MultiCodePointGraphemeCluster generates single grapheme clusters containing
// multiple code points, including long combining-mark and ZWJ cases.
func MultiCodePointGraphemeCluster() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		kind := rapid.SampledFrom([]string{"knownEmoji", "combiningMarks", "zwjEmoji", "zwjEmojiWithSkinTones"}).Draw(t, "clusterKind")
		switch kind {
		case "knownEmoji":
			return rapid.SampledFrom([]string{
				"\U0001F469\u200D\U0001F4BB",                                                       // 👩‍💻
				"\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466",                       // 👨‍👩‍👧‍👦
				"\U0001F469\U0001F3FD\u200D\U0001F4BB",                                             // 👩🏽‍💻
				"\U0001F3F3\uFE0F\u200D\U0001F308",                                                 // 🏳️‍🌈
				"\u2764\uFE0F\u200D\U0001F525",                                                     // ❤️‍🔥
				"\U0001F468\u200D\u2764\uFE0F\u200D\U0001F48B\u200D\U0001F468",                     // 👨‍❤️‍💋‍👨
				"\U0001F469\U0001F3FD\u200D\u2764\uFE0F\u200D\U0001F48B\u200D\U0001F468\U0001F3FF", // 👩🏽‍❤️‍💋‍👨🏿
			}).Draw(t, "knownEmojiCluster")
		case "combiningMarks":
			base := rapid.SampledFrom([]string{"a", "क", "م"}).Draw(t, "base")
			combiningMarkCount := rapid.IntRange(1, 128).Draw(t, "combiningMarkCount")
			marks := rapid.SliceOfN(rapid.SampledFrom([]string{"\u0301", "\u0308", "\u0327", "\u20dd"}), combiningMarkCount, combiningMarkCount).Draw(t, "combiningMarks")
			return base + strings.Join(marks, "")
		case "zwjEmoji":
			componentCount := rapid.IntRange(2, 64).Draw(t, "zwjComponentCount")
			components := rapid.SliceOfN(rapid.SampledFrom([]string{"👩", "👨", "👧", "👦", "💻", "🚄", "🔬"}), componentCount, componentCount).Draw(t, "zwjComponents")
			return strings.Join(components, "\u200d")
		case "zwjEmojiWithSkinTones":
			componentCount := rapid.IntRange(2, 32).Draw(t, "zwjSkinToneComponentCount")
			components := make([]string, 0, componentCount)
			for i := 0; i < componentCount; i++ {
				person := rapid.SampledFrom([]string{"👩", "👨", "🧑"}).Draw(t, "person"+strconv.Itoa(i))
				tone := rapid.SampledFrom([]string{"🏻", "🏼", "🏽", "🏾", "🏿"}).Draw(t, "skinTone"+strconv.Itoa(i))
				components = append(components, person+tone)
			}
			return strings.Join(components, "\u200d")
		default:
			assert.Invariant(false, "unknown cluster kind")
			return ""
		}
	})
}
