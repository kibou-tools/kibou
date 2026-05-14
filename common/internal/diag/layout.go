// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package diag

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/core/option"
	"code.kibou.tools/common/internal/uniseg"
	"code.kibou.tools/common/source_code"
)

type prettyDoc struct {
	lines []prettyLine
	// scratch is temporary storage for use within a single prettyDoc method.
	// Methods must call getScratch before use and stopUsingScratch when done.
	scratch      strings.Builder
	usingScratch bool
}

type prettyLine struct {
	// hasFrame indicates whether a line has frameText as a prefix or not.
	//
	// Any fragments will be rendered after the prefix.
	hasFrame  bool
	fragments []prettyFragment
}

type prettyFragment struct {
	role option.Option[Role]
	text string
}

type diagPieces struct {
	// Inner string is always non-empty.
	CodeString option.Option[string]
	// Inner string is always non-empty.
	CodeSeeAlso option.Option[string]
	// Always non-empty.
	Message string
}

func buildPrettyDoc[C Code](diag Diagnostic[C], opts RenderPrettyOptions) prettyDoc {
	var doc prettyDoc
	pieces := checkRequirements(diag)
	doc.layoutHeader(diag.Severity(), pieces, opts)

	for snippet := range diag.Snippets() {
		// Add "|\n" before each code snippet for readability.
		doc.addFrameLine()
		doc.layoutSnippet(&snippet, opts)
	}

	firstHint := true
	for h := range diag.Hints() {
		if firstHint {
			// Add "|\n" before the first hint/context line, for visual room.
			// This mimics the extra "| \n" before a source code snippet with the
			// gutter and line number, creating balance.
			doc.addFrameLine()
			firstHint = false
		}
		doc.layoutHintLine(h, opts)
	}

	doc.layoutDetailsURL(pieces.CodeSeeAlso, opts)
	return doc
}

func checkRequirements[C Code](diag Diagnostic[C]) diagPieces {
	optCode := diag.Code()
	codeString := option.None[string]()
	codeSeeAlso := option.None[string]()
	if code, ok := optCode.Get(); ok {
		text := code.String()
		assert.Requirement(text != "", "diag.Code.String", "result should be non-empty")
		codeString = option.Some(text)
		codeSeeAlso = code.SeeAlso()
		if seeAlso, ok := codeSeeAlso.Get(); ok {
			assert.Requirement(seeAlso != "", "diag.Code.SeeAlso", "result should be non-empty")
		}
	}
	msg := diag.Message()
	assert.Requirement(msg != "", "diag.Diagnostic.Message", "diagnostic message should be non-empty")
	return diagPieces{CodeString: codeString, CodeSeeAlso: codeSeeAlso, Message: msg}
}

func (d *prettyDoc) layoutHeader(severity Severity, pieces diagPieces, opts RenderPrettyOptions) {
	var role Role
	switch severity {
	case Severity_Error:
		role = Role_SeverityError
	case Severity_Warning, Severity_InternalWarning:
		role = Role_SeverityWarning
	default:
		assert.PanicUnknownCase[any](severity)
	}
	severityLabel := severity.Text() + ":"
	severityLabelWidth := displayWidth(severityLabel)
	msg := pieces.Message
	if code, ok := pieces.CodeString.Get(); ok {
		b := d.getScratch()
		b.Grow(len(msg) + len(code) + len(" ()"))
		b.WriteString(msg)
		b.WriteString(" (")
		b.WriteString(code)
		b.WriteString(")")
		msg = b.String()
		d.stopUsingScratch()
	}

	lines := wrapIndented(msg, opts.MaxWidth, severityLabelWidth+1, displayWidth(frameText)+severityLabelWidth)
	d.addLine(roleFragment(role, severityLabel), plainFragment(" "), plainFragment(lines[0]))
	indent := spaceRun(severityLabelWidth)
	for _, line := range lines[1:] {
		d.addFrameLine(plainFragment(indent), plainFragment(line))
	}
}

func (d *prettyDoc) layoutSnippet(snippet *Snippet, opts RenderPrettyOptions) {
	text := snippet.Text()     // always non-empty
	labels := snippet.Labels() // potentially empty
	lineTable := source_code.NewLineTable(text)
	multiLine := lineTable.LineCount() > 1
	tabWidth := int(opts.TabWidth)

	blocks := layoutSnippetBlocks(&lineTable, labels, tabWidth)
	d.layoutLocationHeader(snippet, blocks, multiLine)

	if len(blocks) == 0 {
		if !multiLine {
			d.addFrameLine(plainFragment("  "), plainFragment(expandTabs(text, tabWidth)))
			return
		}
		for line := range lineTable.Lines() {
			prefix := ""
			if lineNo, ok := snippetDisplayLineNo(snippet, line.LineNumber()).Get(); ok {
				prefix = sourceLinePrefix(lineNo)
			}
			d.addFrameLine(plainFragment("  "), plainFragment(prefix), plainFragment(expandTabs(line.Text(), tabWidth)))
		}
		return
	}

	for i, block := range blocks {
		if i > 0 {
			d.addFrameLine()
		}
		switch block.kind {
		case snippetBlockKind_SingleLine:
			d.layoutSingleLineBlock(block.singleLineLabels, multiLine, snippet)
		case snippetBlockKind_MultiLine:
			d.layoutMultiLineBlock(block.multiLineLabel, multiLine, snippet)
		default:
			assert.PanicUnknownCase[any](block.kind)
		}
	}
}

func (d *prettyDoc) layoutHintLine(h Hint, opts RenderPrettyOptions) {
	label := h.Kind().Text()
	msg := h.Msg()
	firstUsed := displayWidth(frameText) + /* " " */ 1 + displayWidth(label) + /* ": " */ 2
	restUsed := displayWidth(frameText) + /* " " */ 1 + /* "  " (indent) */ 2
	lines := wrapIndented(msg, opts.MaxWidth, firstUsed, restUsed)
	d.addFrameLine(plainFragment(" "), roleFragment(hintRole(h.Kind()), label), plainFragment(": "), plainFragment(lines[0]))
	for _, line := range lines[1:] {
		d.addFrameLine(plainFragment("   "), plainFragment(line))
	}
}

func (d *prettyDoc) layoutDetailsURL(seeAlsoOpt option.Option[string], opts RenderPrettyOptions) {
	seeAlso, ok := seeAlsoOpt.Get()
	if !ok {
		return
	}
	d.addFrameLine()
	b := d.getScratch()
	b.Grow(len(opts.SeeAlsoPrefix) + len(seeAlso))
	b.WriteString(opts.SeeAlsoPrefix)
	b.WriteString(seeAlso)
	msg := b.String()
	d.stopUsingScratch()
	lines := wrapIndented(msg, opts.MaxWidth, displayWidth(frameText)+1, displayWidth(frameText)+1)
	for _, line := range lines {
		d.addFrameLine(plainFragment(" "), plainFragment(line))
	}
}

func (d *prettyDoc) layoutLocationHeader(snippet *Snippet, blocks []snippetBlock, multiLine bool) {
	loc, ok := snippet.Location().Get()
	if !ok {
		return
	}
	locStr, _ := loc.Get()
	// Append :line:col only when there's exactly one label on a multi-line
	// source — the line:col would otherwise be ambiguous.
	if len(blocks) == 1 && blocks[0].labelCount() == 1 && multiLine {
		lineNo, col := blocks[0].startLineColumn()
		if lineNo, ok := snippetDisplayLineNo(snippet, lineNo).Get(); ok {
			locStr = fmt.Sprintf("%s:%d:%d", locStr, lineNo, col+1)
		}
	}
	d.addFrameLine(plainFragment("  In "), plainFragment(locStr))
	d.addFrameLine()
}

func (d *prettyDoc) layoutSingleLineBlock(laidOut []laidOutLabel, multiLine bool, snippet *Snippet) {
	// laidOut is already sorted by (lineNo, col). Walk in groups of same lineNo.
	i := 0
	for i < len(laidOut) {
		j := i + 1
		for j < len(laidOut) && laidOut[j].lineNo == laidOut[i].lineNo {
			j++
		}
		group := laidOut[i:j]
		i = j

		var srcPrefix, caretPrefix string
		if multiLine {
			if lineNo, ok := snippetDisplayLineNo(snippet, group[0].lineNo).Get(); ok {
				srcPrefix = sourceLinePrefix(lineNo)
				caretPrefix = sourceCaretPrefix(lineNo)
			}
		}
		d.addFrameLine(plainFragment("  "), plainFragment(srcPrefix), plainFragment(group[0].line))

		if len(group) == 1 {
			d.layoutInlineLabel(caretPrefix, group[0])
			continue
		}
		// Multi-label group. If any two labels overlap column-wise, fall back
		// to rendering each label on its own caret row inline. Otherwise use
		// the shared caret row + dangling layout.
		if anyOverlap(group) {
			for _, g := range overlapLabelOrder(group) {
				d.layoutInlineLabel(caretPrefix, g)
			}
			continue
		}
		d.layoutStackedLabels(caretPrefix, group)
	}
}

func (d *prettyDoc) layoutMultiLineBlock(label laidOutMultiLineLabel, multiLine bool, snippet *Snippet) {
	for i, segment := range label.segments {
		var srcPrefix, caretPrefix string
		if multiLine {
			if lineNo, ok := snippetDisplayLineNo(snippet, segment.lineNo).Get(); ok {
				srcPrefix = sourceLinePrefix(lineNo)
				caretPrefix = sourceCaretPrefix(lineNo)
			}
		}
		d.addFrameLine(plainFragment("  "), plainFragment(srcPrefix), plainFragment(segment.line))

		marker := markerRunFor(label.label, segment.width)
		if i == len(label.segments)-1 {
			if msg, ok := label.label.Msg().Get(); ok {
				b := d.getScratch()
				b.Grow(len(marker) + 1 + len(msg))
				b.WriteString(marker)
				b.WriteByte(' ')
				b.WriteString(msg)
				marker = b.String()
				d.stopUsingScratch()
			}
		}
		d.addFrameLine(plainFragment("  "), plainFragment(caretPrefix), plainFragment(spaceRun(segment.col)), roleFragment(Role_Caret, marker))
	}
}

func (d *prettyDoc) layoutInlineLabel(caretPrefix string, l laidOutLabel) {
	pad := spaceRun(l.col)
	marker := markerRun(l)
	if msg, ok := l.label.Msg().Get(); ok {
		b := d.getScratch()
		b.Grow(len(marker) + 1 + len(msg))
		b.WriteString(marker)
		b.WriteByte(' ')
		b.WriteString(msg)
		marker = b.String()
		d.stopUsingScratch()
	}
	d.addFrameLine(plainFragment("  "), plainFragment(caretPrefix), plainFragment(pad), roleFragment(Role_Caret, marker))
}

func (d *prettyDoc) layoutStackedLabels(caretPrefix string, group []laidOutLabel) {
	// Caret row.
	caretLine := d.getScratch()
	col := 0
	for _, g := range group {
		if g.col > col {
			writeSpaces(caretLine, g.col-col)
			col = g.col
		}
		caretLine.WriteString(markerRun(g))
		col += g.width
	}
	d.addFrameLine(plainFragment("  "), plainFragment(caretPrefix), roleFragment(Role_Caret, caretLine.String()))
	d.stopUsingScratch()

	// Dangling rows for labels with text, rightmost-first.
	withText := make([]laidOutLabel, 0, len(group))
	for _, g := range group {
		if !g.label.Msg().IsEmpty() {
			withText = append(withText, g)
		}
	}
	for i := len(withText) - 1; i >= 0; i-- {
		cur := withText[i]
		line := d.getScratch()
		c := 0
		for j := 0; j < i; j++ {
			target := withText[j].col
			writeSpaces(line, target-c)
			line.WriteString("|")
			c = target + 1
		}
		writeSpaces(line, cur.col-c)
		msg, _ := cur.label.Msg().Get()
		line.WriteString("└── ")
		line.WriteString(msg)
		d.addFrameLine(plainFragment("  "), plainFragment(caretPrefix), plainFragment(line.String()))
		d.stopUsingScratch()
	}
}

func (d *prettyDoc) addLine(fragments ...prettyFragment) {
	d.lines = append(d.lines, prettyLine{hasFrame: false, fragments: fragments})
}

// addFrameLine adds a line prefixed by the diagnostic frame marker. With no
// fragments, this creates visual space for readability.
func (d *prettyDoc) addFrameLine(fragments ...prettyFragment) {
	d.lines = append(d.lines, prettyLine{hasFrame: true, fragments: fragments})
}

func (d *prettyDoc) getScratch() *strings.Builder {
	assert.Precondition(!d.usingScratch, "prettyDoc scratch is already in use")
	d.scratch.Reset()
	d.usingScratch = true
	return &d.scratch
}

func (d *prettyDoc) stopUsingScratch() {
	assert.Precondition(d.usingScratch, "prettyDoc scratch is not in use")
	d.scratch.Reset()
	d.usingScratch = false
}

type snippetBlockKind uint8

const (
	snippetBlockKind_SingleLine snippetBlockKind = iota + 1
	snippetBlockKind_MultiLine
)

type snippetBlock struct {
	kind snippetBlockKind
	// startByte is used for stable ordering of blocks in source order.
	startByte int

	singleLineLabels []laidOutLabel
	multiLineLabel   laidOutMultiLineLabel
}

type laidOutLabel struct {
	label  LabeledSpan
	line   string // text of the source line containing the label
	lineNo int    // 1-based line number
	col    int    // display column (0-based) of the marker's start within line
	width  int    // display width of the marker (at least 1)
}

type laidOutMultiLineLabel struct {
	label    LabeledSpan
	segments []laidOutLineSegment
}

type laidOutLineSegment struct {
	line   string // text of the source line containing the segment
	lineNo int    // 1-based line number
	col    int    // display column (0-based) of the marker's start within line
	width  int    // display width of the marker (at least 1)
}

const frameText = "│"

func (b snippetBlock) labelCount() int {
	switch b.kind {
	case snippetBlockKind_SingleLine:
		return len(b.singleLineLabels)
	case snippetBlockKind_MultiLine:
		return 1
	default:
		return assert.PanicUnknownCase[int](b.kind)
	}
}

func (b snippetBlock) startLineColumn() (lineNo int, col int) {
	switch b.kind {
	case snippetBlockKind_SingleLine:
		first := b.singleLineLabels[0]
		return first.lineNo, first.col
	case snippetBlockKind_MultiLine:
		first := b.multiLineLabel.segments[0]
		return first.lineNo, first.col
	default:
		return assert.PanicUnknownCase[int](b.kind), 0
	}
}

type lineOffset struct {
	line source_code.Line
	off  int
}

func layoutSnippetBlocks(lineTable *source_code.LineTable, labels []LabeledSpan, tabWidth int) []snippetBlock {
	if len(labels) == 0 {
		return nil
	}

	labels = slices.Clone(labels)
	slices.SortStableFunc(labels, func(a, b LabeledSpan) int {
		return a.Span().CompareStrict(b.Span())
	})

	var blocks []snippetBlock
	var singleLineLabels []LabeledSpan
	flushSingleLineLabels := func() {
		if len(singleLineLabels) == 0 {
			return
		}
		var emptyMultiLineLabel laidOutMultiLineLabel
		blocks = append(blocks, snippetBlock{
			kind:             snippetBlockKind_SingleLine,
			startByte:        singleLineLabels[0].Span().StartByte(),
			singleLineLabels: layoutLabels(lineTable, singleLineLabels, tabWidth),
			multiLineLabel:   emptyMultiLineLabel,
		})
		singleLineLabels = nil
	}

	for _, label := range labels {
		if !spanIsMultiLine(lineTable, label.Span()) {
			singleLineLabels = append(singleLineLabels, label)
			continue
		}

		flushSingleLineLabels()
		blocks = append(blocks, snippetBlock{
			kind:             snippetBlockKind_MultiLine,
			startByte:        label.Span().StartByte(),
			singleLineLabels: nil,
			multiLineLabel:   layoutMultiLineLabel(lineTable, label, tabWidth),
		})
	}
	flushSingleLineLabels()
	return blocks
}

func layoutLabels(lineTable *source_code.LineTable, labels []LabeledSpan, tabWidth int) []laidOutLabel {
	if len(labels) == 0 {
		return nil
	}
	out := make([]laidOutLabel, 0, len(labels))
	for _, l := range labels {
		line, off := lineForSpan(lineTable, l.Span())
		out = append(out, laidOutLabel{
			label:  l,
			line:   expandTabs(line.Text(), tabWidth),
			lineNo: line.LineNumber(),
			col:    displayColumn(line.Text(), off, tabWidth),
			width:  spanDisplayWidth(line.Text(), off, l.Span().ByteLen(), tabWidth),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].lineNo != out[j].lineNo {
			return out[i].lineNo < out[j].lineNo
		}
		return out[i].col < out[j].col
	})
	return out
}

func layoutMultiLineLabel(lineTable *source_code.LineTable, label LabeledSpan, tabWidth int) laidOutMultiLineLabel {
	span := label.Span()
	start := lineOffsetForByte(lineTable, span.StartByte())
	end := lineOffsetForByte(lineTable, span.EndByte())
	assert.Invariant(start.line.LineNumber() != end.line.LineNumber(), "span should cross line boundary")

	segments := make([]laidOutLineSegment, 0, end.line.LineNumber()-start.line.LineNumber()+1)
	for line := range lineTable.Lines() {
		lineNo := line.LineNumber()
		if lineNo < start.line.LineNumber() || lineNo > end.line.LineNumber() {
			continue
		}

		raw := line.Text()
		startOff := 0
		endOff := len(raw)
		if lineNo == start.line.LineNumber() {
			startOff = start.off
		}
		if lineNo == end.line.LineNumber() {
			endOff = end.off
		}
		if startOff > endOff {
			startOff = endOff
		}
		segments = append(segments, laidOutLineSegment{
			line:   expandTabs(raw, tabWidth),
			lineNo: lineNo,
			col:    displayColumn(raw, startOff, tabWidth),
			width:  spanDisplayWidth(raw, startOff, endOff-startOff, tabWidth),
		})
	}
	return laidOutMultiLineLabel{label: label, segments: segments}
}

func spanIsMultiLine(lineTable *source_code.LineTable, span SourceSpan) bool {
	start := lineOffsetForByte(lineTable, span.StartByte())
	end := lineOffsetForByte(lineTable, span.EndByte())
	return start.line.LineNumber() != end.line.LineNumber()
}

func lineForSpan(lineTable *source_code.LineTable, span SourceSpan) (source_code.Line, int) {
	start := lineOffsetForByte(lineTable, span.StartByte())
	assert.Invariantf(span.EndByte() <= lineTable.TextLen(), "span end %d is past source length %d", span.EndByte(), lineTable.TextLen())
	assert.Invariant(start.off+span.ByteLen() <= len(start.line.Text()), "span should not cross line boundary")
	return start.line, start.off
}

func lineOffsetForByte(lineTable *source_code.LineTable, byteOffset int) lineOffset {
	assert.Invariantf(byteOffset <= lineTable.TextLen(), "byte offset %d is past source length %d", byteOffset, lineTable.TextLen())
	line := lineTable.LineAtByteOffset(byteOffset)
	off := byteOffset - line.Span().Start()
	if off > len(line.Text()) {
		off = len(line.Text())
	}
	return lineOffset{line: line, off: off}
}

func (l laidOutLabel) displayOverlaps(other laidOutLabel) bool {
	end := l.col + l.width
	otherEnd := other.col + other.width
	return l.col < otherEnd && other.col < end
}

func anyOverlap(group []laidOutLabel) bool {
	for i := 0; i+1 < len(group); i++ {
		if group[i].displayOverlaps(group[i+1]) {
			return true
		}
	}
	return false
}

func overlapLabelOrder(labels []laidOutLabel) []laidOutLabel {
	ordered := make([]laidOutLabel, 0, len(labels))
	for _, label := range labels {
		if label.label.Options().IsEmphasized() {
			ordered = append(ordered, label)
		}
	}
	for _, label := range labels {
		if !label.label.Options().IsEmphasized() {
			ordered = append(ordered, label)
		}
	}
	return ordered
}

func snippetDisplayLineNo(snippet *Snippet, snippetLineNo int) option.Option[int] {
	if startLine, ok := snippet.StartLine().Get(); ok {
		return option.Some(startLine + snippetLineNo - 1)
	}
	return option.None[int]()
}

func sourceLinePrefix(lineNo int) string {
	lineNoText := strconv.Itoa(lineNo)
	var b strings.Builder
	b.Grow(len(lineNoText) + len(" | "))
	b.WriteString(lineNoText)
	b.WriteString(" | ")
	return b.String()
}

func sourceCaretPrefix(lineNo int) string {
	lineNoText := strconv.Itoa(lineNo)
	var b strings.Builder
	b.Grow(len(lineNoText) + len(" : "))
	writeSpaces(&b, len(lineNoText))
	b.WriteString(" : ")
	return b.String()
}

func plainFragment(text string) prettyFragment {
	return prettyFragment{role: option.None[Role](), text: text}
}

func roleFragment(role Role, text string) prettyFragment {
	return prettyFragment{role: option.Some(role), text: text}
}

func writeSpaces(b *strings.Builder, n int) {
	b.WriteString(spaceRun(n))
}

const spaceRunCache = "                                                                "

func spaceRun(width int) string {
	if 0 <= width && width <= len(spaceRunCache) {
		return spaceRunCache[:width]
	}
	return strings.Repeat(" ", width)
}

const (
	markerRunCacheHalf = 32
	markerRunCache     = "^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^--------------------------------"
)

func markerRun(l laidOutLabel) string {
	return markerRunFor(l.label, l.width)
}

func markerRunFor(label LabeledSpan, width int) string {
	if 0 <= width && width <= markerRunCacheHalf {
		if label.Options().IsEmphasized() {
			return markerRunCache[:width]
		}
		return markerRunCache[markerRunCacheHalf : markerRunCacheHalf+width]
	}
	return strings.Repeat(markerChar(label), width)
}

func markerChar(l LabeledSpan) string {
	if l.Options().IsEmphasized() {
		return "^"
	}
	return "-"
}

func hintRole(k HintKind) Role {
	switch k {
	case HintKind_Suggestion:
		return Role_HintSuggestion
	case HintKind_Context:
		return Role_HintContext
	default:
		return Role_HintContext
	}
}

func expandTabs(s string, tabWidth int) string {
	if tabWidth <= 0 {
		assert.Preconditionf(false, "non-positive tab width: %d", tabWidth)
	}
	if strings.IndexByte(s, '\t') < 0 {
		return s
	}
	var b strings.Builder
	column := 0
	for cluster := range uniseg.GraphemeClusters(s) {
		text := cluster.Str()
		if text == "\t" {
			spaces := tabWidth - column%tabWidth
			b.WriteString(spaceRun(spaces))
			column += spaces
			continue
		}
		b.WriteString(text)
		column += displayWidth(text)
	}
	return b.String()
}

func spanDisplayWidth(line string, byteOffset int, byteLen int, tabWidth int) int {
	assert.Preconditionf(byteOffset >= 0, "negative byte offset: %d", byteOffset)
	assert.Preconditionf(byteLen >= 0, "negative byte length: %d", byteLen)
	assert.Preconditionf(byteOffset+byteLen <= len(line), "span end %d past line length %d", byteOffset+byteLen, len(line))
	if byteLen == 0 {
		return 1
	}
	end := byteOffset + byteLen
	startCol := displayColumn(line, byteOffset, tabWidth)
	endCol := displayColumn(line, end, tabWidth)
	width := endCol - startCol
	if width == 0 {
		return 1
	}
	return width
}

// displayColumn returns the terminal column (0-based) corresponding to byte
// offset `byteOffset` within s.
//
// TODO: Handle spans whose byte offsets split grapheme clusters; this assumes
// byteOffset is on a grapheme-cluster boundary.
func displayColumn(s string, byteOffset int, tabWidth int) int {
	assert.Preconditionf(byteOffset >= 0, "negative byte offset: %d", byteOffset)
	assert.Preconditionf(byteOffset <= len(s), "byte offset %d past source length %d", byteOffset, len(s))
	return displayWidthWithTabs(s[:byteOffset], tabWidth, 0)
}

// displayWidth returns the terminal-column width of s. It follows Unicode
// grapheme cluster boundaries, so combining characters and zero-width joiner
// sequences are measured as the terminal cells occupied by the rendered cluster.
func displayWidth(s string) int {
	return uniseg.ComputeWidth(s)
}

func displayWidthWithTabs(s string, tabWidth int, startColumn int) int {
	if tabWidth <= 0 {
		assert.Preconditionf(false, "non-positive tab width: %d", tabWidth)
	}
	column := startColumn
	for cluster := range uniseg.GraphemeClusters(s) {
		text := cluster.Str()
		if text == "\t" {
			column += tabWidth - column%tabWidth
			continue
		}
		column += displayWidth(text)
	}
	return column - startColumn
}

// wrapIndented attempts to wrap s to maxWidth, splitting the string
// based on:
//
// - Word boundaries (detected based on whitespace, not CJK-aware)
// - Grapheme cluster boundaries (avoiding splitting in the middle of one).
//
// firstUsed and laterUsed are display columns already occupied by prefixes
// before s starts on the first output line and later output lines.
func wrapIndented(s string, maxWidth option.Option[uint8], firstUsed int, laterUsed int) []string {
	width, ok := maxWidth.Get()
	if !ok {
		return []string{s}
	}
	firstAvail := int(width) - firstUsed
	laterAvail := int(width) - laterUsed
	if firstAvail < 1 || laterAvail < 1 {
		return []string{s}
	}
	var lines []string
	line := s
	avail := firstAvail
	for {
		split, ok := splitLine(line, avail).Get()
		if !ok {
			lines = append(lines, line)
			return lines
		}
		lines = append(lines, split.Before)
		line = split.After
		avail = laterAvail
	}
}

type wrapSplit struct {
	Before string
	After  string
}

func splitLine(s string, width int) option.Option[wrapSplit] {
	if width <= 0 {
		assert.Preconditionf(false, "non-positive split width: %d", width)
	}
	if displayWidth(s) <= width {
		return option.None[wrapSplit]()
	}
	col := 0
	lastBreakStart := -1
	lastBreakEnd := -1
	for cluster := range uniseg.GraphemeClusters(s) {
		text := cluster.Str()
		nextCol := col + displayWidth(text)
		if nextCol > width {
			break
		}
		isWhitespace := text != ""
		for _, r := range text {
			if !unicode.IsSpace(r) {
				isWhitespace = false
				break
			}
		}
		if isWhitespace {
			span := cluster.Span()
			lastBreakStart = span.Start()
			lastBreakEnd = span.End()
		}
		col = nextCol
	}
	if lastBreakStart < 0 {
		return option.None[wrapSplit]()
	}
	return option.Some(wrapSplit{Before: strings.TrimRightFunc(s[:lastBreakStart], unicode.IsSpace), After: strings.TrimLeftFunc(s[lastBreakEnd:], unicode.IsSpace)})
}
