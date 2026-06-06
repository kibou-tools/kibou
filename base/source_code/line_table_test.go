// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package source_code_test

import (
	"strings"
	"testing"

	"code.kibou.tools/base/check"
	"code.kibou.tools/base/ranges"
	"code.kibou.tools/base/source_code"

	"pgregory.net/rapid"
)

type lineTableModel struct {
	text  string
	lines []modelLine
}

type modelLine struct {
	Text       string
	StartByte  int
	EndByte    int
	LineNumber int
}

func TestLineTable(t *testing.T) {
	h := check.New(t)
	h.Run("unit", func(h check.Harness) {
		for _, tc := range []string{
			"",
			"abc",
			"abc\ndef",
			"abc\r\ndef",
			"abc\rdef",
			"abc\n",
			"abc\r\n",
		} {
			checkLineTable(h, tc, 1)
			checkLineTable(h, tc, 7)
		}
	})
	h.Run("property", func(h check.Harness) {
		rapid.Check(h.T(), func(t *rapid.T) {
			parts := rapid.SliceOfN(rapid.SampledFrom([]string{"", "a", "bc", "\n", "\r", "\r\n", "é", "👩‍💻"}), 0, 40).Draw(t, "parts")
			startLineNumber := rapid.IntRange(1, 100).Draw(t, "startLineNumber")
			checkLineTable(check.NewBasic(t), strings.Join(parts, ""), startLineNumber)
		})
	})
}

func checkLineTable(h check.BasicHarness, text string, startLineNumber int) {
	model := newLineTableModel(text, startLineNumber)
	table := source_code.NewLineTableAt(text, startLineNumber)
	check.AssertSame(h, len(text), table.TextLen(), "TextLen")
	check.AssertSame(h, len(model.lines), table.LineCount(), "LineCount")

	var gotLines []modelLine
	for line := range table.Lines() {
		gotLines = append(gotLines, modelLine{Text: line.Text(), StartByte: line.Span().Start(), EndByte: line.Span().End(), LineNumber: line.LineNumber()})
	}
	check.AssertSame(h, model.lines, gotLines, "Lines")

	for pos := 0; pos <= len(text); pos++ {
		got := table.LineAtByteOffset(pos)
		want := model.lineAtByteOffset(pos)
		check.AssertSame(h, want.Text, got.Text(), "LineAtByteOffset Text")
		wantSpan := ranges.NewSpan(want.StartByte, want.EndByte)
		check.AssertSame(h, 0, wantSpan.CompareStrict(got.Span()), "LineAtByteOffset Span")
		check.AssertSame(h, want.LineNumber, got.LineNumber(), "LineAtByteOffset LineNumber")
	}
}

func newLineTableModel(text string, startLineNumber int) lineTableModel {
	var lines []modelLine
	rest := text
	offset := 0
	for {
		sep := strings.IndexAny(rest, "\r\n")
		if sep < 0 {
			lines = append(lines, modelLine{Text: rest, StartByte: offset, EndByte: len(text), LineNumber: startLineNumber + len(lines)})
			return lineTableModel{text: text, lines: lines}
		}
		end := offset + sep
		lines = append(lines, modelLine{Text: rest[:sep], StartByte: offset, EndByte: end, LineNumber: startLineNumber + len(lines)})
		sepLen := 1
		if rest[sep] == '\r' && sep+1 < len(rest) && rest[sep+1] == '\n' {
			sepLen = 2
		}
		rest = rest[sep+sepLen:]
		offset = end + sepLen
	}
}

func (m lineTableModel) lineAtByteOffset(byteOffset int) modelLine {
	if byteOffset == len(m.text) {
		return m.lines[len(m.lines)-1]
	}
	for i, line := range m.lines {
		nextByte := len(m.text)
		if i+1 < len(m.lines) {
			nextByte = m.lines[i+1].StartByte
		}
		if byteOffset < nextByte {
			return line
		}
	}
	panic("unreachable")
}
