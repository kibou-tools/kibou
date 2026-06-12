// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package diag

import (
	"fmt"
	"iter"
	"path/filepath"
	"runtime"
	"strings"

	"code.kibou.tools/base/assert"
	. "code.kibou.tools/base/core/option"
	"code.kibou.tools/base/core/result"
	"code.kibou.tools/base/errorx"
	"code.kibou.tools/base/fsx/fsx_name"
	"code.kibou.tools/base/internal/uniseg"
	"code.kibou.tools/base/iterx"
	"code.kibou.tools/base/ranges"
	"code.kibou.tools/base/utf8"
)

// Snippet marks the set of snippet types supported by this package.
//
// This is a closed interface; only types defined in this package can implement
// it. Renderers should type switch over the concrete snippet types they support.
type Snippet interface {
	isSnippet()
}

// TextSnippet pairs a relevant textual excerpt with zero or more annotations on
// sub-parts of that excerpt, explaining the containing Diagnostic in more
// detail.
//
// Text snippets may be used to describe fragments of files, command-line
// arguments, environment variables, input read from stdin, data obtained over
// the network etc.
//
// See [TextSnippetBuilder.WithLocation] for examples.
type TextSnippet struct {
	text      string
	location  Option[errorx.Phrase]
	startLine Option[int]
	labels    []LabeledSpan
}

func (TextSnippet) isSnippet() {}

// Text gets the original text associated with this TextSnippet.
//
// Post-condition: The returned string is non-empty.
func (s TextSnippet) Text() string { return s.text }

// Location gets the location set by WithLocation.
func (s TextSnippet) Location() Option[errorx.Phrase] { return s.location }

// StartLine gets the start line set by WithStartLine.
func (s TextSnippet) StartLine() Option[int] { return s.startLine }

// Labels returns the labels attached to this TextSnippet in insertion order.
// The returned slice must not be modified.
func (s TextSnippet) Labels() []LabeledSpan {
	return s.labels
}

// TextSnippetBuilder incrementally constructs a [TextSnippet].
//
// Call [TextSnippetBuilder.Build] to perform validation checks.
type TextSnippetBuilder struct {
	text      string
	location  Option[errorx.Phrase]
	startLine Option[int]
	labels    []snippetBuilderLabel
}

// snippetBuilderLabel stores a label attached while building a snippet.
type snippetBuilderLabel struct {
	span    ranges.Span[int]
	msg     errorx.Phrase
	options LabelOptions
}

// NewTextSnippet starts constructing a text snippet from the associated text.
//
// For example, if you're diagnosing an error in a configuration file, you'd do:
//
//	partial := diag.NewTextSnippet(configLine).
//				WithLocation(errorx.NewPhrase(configFilePath.String())).
//				AtRange(...).Attach(...).
//				Build()
//
// The text is the only required field. Calling other methods is optional.
//
// The input text is expected to be valid UTF-8. Invalid UTF-8 will lead to
// an error later in [TextSnippetBuilder.Build].
//
// Precondition: text must be non-empty.
func NewTextSnippet(text string) *TextSnippetBuilder {
	if text == "" {
		assert.Precondition(false, "diag.NewTextSnippet: text parameter must be non-empty")
	}
	return &TextSnippetBuilder{
		text:      text,
		location:  None[errorx.Phrase](),
		startLine: None[int](),
		labels:    nil,
	}
}

// WithLocation describes where the snippet was sourced from.
//
// Examples of potential values:
//   - A file path.
//   - "standard input"
//   - "<command line argument #1>".
//   - "the MYAPP_HEALTHZ_INTERVAL environment variable"
//
// Precondition: location must be non-empty, and must not already be set on
// the receiver.
func (b *TextSnippetBuilder) WithLocation(location errorx.Phrase) *TextSnippetBuilder {
	assert.Precondition(!location.IsEmpty(), "snippet location should be non-empty")
	assert.Precondition(b.location.IsNone(), "snippet location already set")
	b.location = Some(location)
	return b
}

// WithStartLine sets the 1-based original source line corresponding to the
// first line of the snippet.
//
// Precondition: startLine must be positive, and must not already be set on the
// receiver.
func (b *TextSnippetBuilder) WithStartLine(startLine int) *TextSnippetBuilder {
	if startLine <= 0 {
		assert.Preconditionf(false, "startLine parameter (%d) must be >= 1", startLine)
	}
	assert.Precondition(b.startLine.IsNone(), "snippet start line already set")
	b.startLine = Some(startLine)
	return b
}

// AtRange creates a temporary struct for attaching a label to a snippet
// for the given half-open byte range [startByte, startByte+byteLen).
//
// For example, to label `foo()` in the text `x := foo()`:
//
//	|x| |:|=| |f|o|o|(|)|
//	0 1 2 3 4 5 6 7 8 9 10
//
// pass startByte == 5 and byteLen == 5. The end offset is therefore
// startByte + byteLen == 10, which may equal len(Text()).
//
// Preconditions:
// - startByte ∈ [0, len(s.Text()))
// - byteLen > 0
// - startByte + byteLen ∈ [startByte + 1, len(s.Text())]
//
// If these preconditions are not upheld, AtRange will panic.
func (b *TextSnippetBuilder) AtRange(startByte, byteLen int) LabelAttacher {
	span := checkedLabelSpan(b.text, startByte, startByte+byteLen, false)
	return LabelAttacher{src: b, span: span}
}

// AtPos creates a temporary struct for attaching a zero-width label to a
// snippet at the given byte position.
//
// The position may be len(Text()), which is useful for diagnostics about a
// missing token at the end of the snippet. For example, in `x := foo()`:
//
//	|x| |:|=| |f|o|o|(|)|
//	0 1 2 3 4 5 6 7 8 9 10
//
// passing bytePos == 10 points just after the ')'.
//
// Precondition: bytePos ∈ [0, len(s.Text())]
//
// If this precondition is not upheld, AtPos will panic.
func (b *TextSnippetBuilder) AtPos(bytePos int) LabelAttacher {
	span := checkedLabelSpan(b.text, bytePos, bytePos, true)
	return LabelAttacher{src: b, span: span}
}

// Build attempts to construct a TextSnippet from a TextSnippetBuilder.
//
// If there are any errors, those are recorded in the returned *SnippetError.
// Some kinds of errors that can happen:
//
//   - The snippet text was invalid UTF-8. In this case, interpreting byte
//     offsets correctly is tricky, so no TextSnippet is returned.
//     The corresponding SnippetError visualizes the invalid bytes.
//   - Label offsets pointing to the middle of a codepoint or a grapheme cluster.
//     This is handled by dropping the label, and returning a TextSnippet,
//     along with a non-nil SnippetError.
//
// SnippetError also implements Diagnostic, so that can be printed alongside/after
// the primary Diagnostic which contains the built TextSnippet. See the methods
// on SnippetError for more details.
func (b *TextSnippetBuilder) Build() result.Partial[TextSnippet, SnippetError] {
	text := b.text
	var problems []snippetProblem
	if invalidSpan, ok := utf8.FirstInvalidSpan(text).Get(); ok {
		problems = append(problems, newInvalidUTF8SnippetProblem(invalidSpan))
		return result.NewPartial(None[TextSnippet](), &SnippetError{text, problems, captureSnippetCallSite()})
	}

	segmentedText := uniseg.NewSegmentedText(text)
	labels := make([]LabeledSpan, 0, len(b.labels))
	for _, label := range b.labels {
		if err := segmentedText.CheckSpan(label.span); err != nil {
			problems = append(problems, newBoundarySnippetProblem(label.msg, err))
			continue
		}
		labels = append(labels, LabeledSpan{SourceSpan{label.span}, label.msg, label.options})
	}

	snippet := TextSnippet{
		text:      text,
		location:  b.location,
		startLine: b.startLine,
		labels:    labels,
	}
	var err *SnippetError = nil
	if len(problems) != 0 {
		err = &SnippetError{text, problems, captureSnippetCallSite()}
	}
	return result.NewPartial(Some(snippet), err)
}

// LabelAttacher is an intermediate struct to offer a more natural API
// for TextSnippet construction.
type LabelAttacher struct {
	src  *TextSnippetBuilder
	span ranges.Span[int]
}

// Attach appends a label for the span selected by [TextSnippetBuilder.AtRange]
// or [TextSnippetBuilder.AtPos], then returns the underlying builder.
//
// The variadic list of edits allows customizing the label.
//
// Precondition: label must be non-empty.
func (a LabelAttacher) Attach(label errorx.Phrase, edits ...LabelEdit) *TextSnippetBuilder {
	if label.IsEmpty() {
		assert.Precondition(false, "label should be non-empty")
	}
	options := LabelOptions{isEmphasized: false}
	for _, edit := range edits {
		options.Apply(edit)
	}
	a.src.labels = append(a.src.labels, snippetBuilderLabel{span: a.span, msg: label, options: options})
	return a.src
}

// SnippetError reports invalid snippet construction discovered by
// [TextSnippetBuilder.Build].
type SnippetError struct {
	// Original snippet text passed to NewTextSnippet. May be invalid UTF-8.
	text string
	// List of issues found by TextSnippetBuilder.Build.
	//
	// Always non-empty.
	problems []snippetProblem
	// Best-effort source location of the Build call which triggered
	// creation of this SnippetError
	callSite Option[snippetCallSite]
}

var _ Diagnostic = (*SnippetError)(nil)

func (e *SnippetError) Error() string { return e.Message() }

func (e *SnippetError) Severity() Severity       { return Severity_Warning }
func (e *SnippetError) Attribution() Attribution { return Attribution_Internal }
func (e *SnippetError) Message() string {
	if len(e.problems) == 1 {
		return e.problems[0].message()
	}
	return "Invalid source snippet"
}
func (e *SnippetError) Snippets() iter.Seq[Snippet] {
	return func(yield func(Snippet) bool) {
		for _, problem := range e.problems {
			if snippet, ok := e.problemSnippet(problem).Get(); ok {
				if !yield(snippet) {
					return
				}
			}
		}
	}
}
func (e *SnippetError) Notes() iter.Seq[Note] {
	return iterx.Once(NewNote(NoteKind_Context, "this may be a bug in the program constructing this diagnostic"))
}

func (e *SnippetError) problemSnippet(problem snippetProblem) Option[TextSnippet] {
	switch problem.kind {
	case snippetProblemKind_InvalidUTF8:
		data := problem.invalidUTF8Data.Expect("invalid UTF-8 snippet problem should have data")
		return byteRangeSnippet(e.text, data.span, replacementLabel(data.span), e.callSite)
	case snippetProblemKind_SpanNotUTF8Boundary:
		data := problem.boundaryData.Expect("UTF-8 boundary snippet problem should have data")
		offset := data.byteOffset()
		return codePointOffsetSnippet(e.text, offset, spanBoundLabel(data.bound, offset), e.callSite)
	case snippetProblemKind_SpanNotGraphemeBoundary:
		data := problem.boundaryData.Expect("grapheme boundary snippet problem should have data")
		offset := data.byteOffset()
		return graphemeOffsetSnippet(e.text, data.containing, offset, spanBoundLabel(data.bound, offset), e.callSite)
	default:
		assert.PanicUnknownCase[any](problem.kind)
	}
	return None[TextSnippet]()
}

// snippetProblem represents an error discovered during snippet building.
type snippetProblem struct {
	kind snippetProblemKind
	// Some iff kind is snippetProblemKind_InvalidUTF8.
	invalidUTF8Data Option[snippetInvalidUTF8Data]
	// Some iff kind is one of the boundary problem kinds.
	boundaryData Option[snippetBoundaryData]
}

type snippetProblemKind uint8

const (
	snippetProblemKind_InvalidUTF8 snippetProblemKind = iota + 1
	snippetProblemKind_SpanNotUTF8Boundary
	snippetProblemKind_SpanNotGraphemeBoundary
)

type snippetInvalidUTF8Data struct {
	// The bad Span which led to this error.
	span ranges.Span[int]
}

type snippetBoundaryData struct {
	// The label attached to the bad Span.
	label errorx.Phrase
	// The bad Span which led to this error.
	span ranges.Span[int]
	// The edge of span which led to this error.
	bound ranges.Bound
	// containing is optionally a larger Span than span.
	// E.g. if the boundary error corresponds to a bound
	// (start or end) within a grapheme cluster, then containing
	// is the Span for that grapheme cluster.
	containing Option[ranges.Span[int]]
}

// snippetCallSite identifies the source location of the call that produced a
// snippet construction error.
type snippetCallSite struct {
	file fsx_name.Name
	line int
}

// LabeledSpan is the pairing of a SourceSpan and an associated label.
type LabeledSpan struct {
	span    SourceSpan
	msg     errorx.Phrase
	options LabelOptions
}

func (l LabeledSpan) Span() SourceSpan      { return l.span }
func (l LabeledSpan) Msg() errorx.Phrase    { return l.msg }
func (l LabeledSpan) Options() LabelOptions { return l.options }

type LabelOptions struct {
	isEmphasized bool
}

func (o *LabelOptions) IsEmphasized() bool { return o.isEmphasized }

func (o *LabelOptions) Apply(edit LabelEdit) {
	if val, ok := edit.Emphasize.Get(); ok {
		o.isEmphasized = val
	}
}

type LabelEdit struct {
	Emphasize Option[bool]
}

func Emphasize() LabelEdit {
	return LabelEdit{Emphasize: Some(true)}
}

// SourceSpan represents a byte range of a [TextSnippet.Text].
type SourceSpan struct {
	span ranges.Span[int]
}

func (s SourceSpan) StartByte() int { return s.span.Start() }

// ByteLen returns the non-negative length of this SourceSpan.
//
// The zero case can happen for spans created using TextSnippetBuilder.AtPos.
func (s SourceSpan) ByteLen() int {
	return s.span.Length().Expect("overflow checking happens during construction; so value should be in-bounds")
}

func (s SourceSpan) EndByte() int { return s.span.End() }

func (s SourceSpan) CompareStrict(other SourceSpan) int {
	return s.span.CompareStrict(other.span)
}

func newSnippetProblem(kind snippetProblemKind) snippetProblem {
	return snippetProblem{kind, None[snippetInvalidUTF8Data](), None[snippetBoundaryData]()}
}

func newInvalidUTF8SnippetProblem(span ranges.Span[int]) snippetProblem {
	problem := newSnippetProblem(snippetProblemKind_InvalidUTF8)
	problem.invalidUTF8Data = Some(snippetInvalidUTF8Data{span})
	return problem
}

func newBoundarySnippetProblem(label errorx.Phrase, err *uniseg.SpanBoundaryError) snippetProblem {
	var kind snippetProblemKind
	switch err.Kind() {
	case uniseg.SpanBoundaryErrorKind_NotUTF8Boundary:
		kind = snippetProblemKind_SpanNotUTF8Boundary
	case uniseg.SpanBoundaryErrorKind_NotGraphemeBoundary:
		kind = snippetProblemKind_SpanNotGraphemeBoundary
	default:
		assert.PanicUnknownCase[any](err.Kind())
	}
	problem := newSnippetProblem(kind)
	problem.boundaryData = Some(snippetBoundaryData{label, err.Span(), err.Bound(), err.ContainingGraphemeCluster()})
	return problem
}

func (d snippetBoundaryData) byteOffset() int {
	switch d.bound {
	case ranges.Bound_Start:
		return d.span.Start()
	case ranges.Bound_End:
		return d.span.End()
	default:
		return assert.PanicUnknownCase[int](d.bound)
	}
}

func (p snippetProblem) message() string {
	switch p.kind {
	case snippetProblemKind_InvalidUTF8:
		return "Invalid UTF-8; replacing invalid bytes with U+FFFD"
	case snippetProblemKind_SpanNotUTF8Boundary:
		return fmt.Sprintf("Label span %s inside a UTF-8 codepoint; dropping label %q", p.boundVerb(), p.labelText())
	case snippetProblemKind_SpanNotGraphemeBoundary:
		return fmt.Sprintf("Label span %s inside a grapheme cluster; dropping label %q", p.boundVerb(), p.labelText())
	default:
		return assert.PanicUnknownCase[string](p.kind)
	}
}

func (p snippetProblem) labelText() string {
	data := p.boundaryData.Expect("snippet boundary problem should have data")
	text, ok := data.label.Get()
	assert.Invariant(ok, "snippet boundary problem should have label text")
	return text
}

func (p snippetProblem) boundVerb() string {
	bound := p.boundaryData.Expect("snippet boundary problem should have data").bound
	switch bound {
	case ranges.Bound_Start:
		return "starts"
	case ranges.Bound_End:
		return "ends"
	default:
		return assert.PanicUnknownCase[string](bound)
	}
}

// checkedLabelSpan constructs a label span after checking byte-range bounds.
//
// Preconditions:
// - start ∈ [0, len(text)]
// - end ∈ [start, len(text)]
// - if allowEmpty is false, start < end
func checkedLabelSpan(text string, start int, end int, allowEmpty bool) ranges.Span[int] {
	if end < start {
		assert.Preconditionf(false, "label span end %d before start %d", end, start)
	}
	if start < 0 {
		assert.Preconditionf(false, "label span start %d before 0", start)
	}
	if len(text) < start {
		assert.Preconditionf(false, "label span start %d after snippet end %d", start, len(text))
	}
	if len(text) < end {
		assert.Preconditionf(false, "label span end %d after snippet end %d", end, len(text))
	}
	span := ranges.NewSpan(start, end)
	if !allowEmpty && span.IsEmpty() {
		assert.Precondition(false, "label range should be non-empty")
	}
	return span
}

func byteRangeSnippet(text string, span ranges.Span[int], label string, callSite Option[snippetCallSite]) Option[TextSnippet] {
	return byteVisualizationSnippet(text, span.Start(), span.End(), callSite, []byteVisualizationLabel{{
		span:         span,
		label:        label,
		isEmphasized: true,
	}})
}

func byteOffsetSnippet(text string, offset int, label string, callSite Option[snippetCallSite]) Option[TextSnippet] {
	end := offset + 1
	if len(text) < end {
		end = offset
	}
	return byteRangeSnippet(text, ranges.NewSpan(offset, end), label, callSite)
}

func codePointOffsetSnippet(text string, offset int, label string, callSite Option[snippetCallSite]) Option[TextSnippet] {
	codePoint, ok := utf8.CodePointContaining(text, offset).Get()
	if !ok {
		return byteOffsetSnippet(text, offset, label, callSite)
	}
	end := offset + 1
	if len(text) < end {
		end = offset
	}
	return byteVisualizationSnippet(text, codePoint.Start(), codePoint.End(), callSite, []byteVisualizationLabel{
		{span: codePoint, label: fmt.Sprintf("codepoint %q", text[codePoint.Start():codePoint.End()]), isEmphasized: false},
		{span: ranges.NewSpan(offset, end), label: label, isEmphasized: true},
	})
}

func graphemeOffsetSnippet(text string, containing Option[ranges.Span[int]], offset int, label string, callSite Option[snippetCallSite]) Option[TextSnippet] {
	cluster, ok := containing.Get()
	if !ok {
		return byteOffsetSnippet(text, offset, label, callSite)
	}
	end := offset + 1
	if len(text) < end {
		end = offset
	}
	return byteVisualizationSnippet(text, cluster.Start(), cluster.End(), callSite, []byteVisualizationLabel{
		{span: cluster, label: fmt.Sprintf("grapheme cluster \"%s\"", text[cluster.Start():cluster.End()]), isEmphasized: false},
		{span: ranges.NewSpan(offset, end), label: label, isEmphasized: true},
	})
}

type byteVisualizationLabel struct {
	span         ranges.Span[int]
	label        string
	isEmphasized bool
}

type byteVisualization struct {
	text       string
	starts     []int
	ends       []int
	sourceBase int
}

func byteVisualizationSnippet(text string, focusStart int, focusEnd int, callSite Option[snippetCallSite], labels []byteVisualizationLabel) Option[TextSnippet] {
	viz := newByteVisualization(text, focusStart, focusEnd)
	snippetLabels := make([]LabeledSpan, 0, len(labels))
	for _, label := range labels {
		start := viz.position(label.span.Start())
		end := viz.endPosition(label.span.End() - 1)
		if start == end {
			continue
		}
		snippetLabels = append(snippetLabels, LabeledSpan{
			span:    SourceSpan{span: ranges.NewSpan(start, end)},
			msg:     errorx.NewPhrase(label.label),
			options: LabelOptions{isEmphasized: label.isEmphasized},
		})
	}
	if len(snippetLabels) == 0 {
		return None[TextSnippet]()
	}
	return Some(TextSnippet{
		text:      viz.text,
		location:  Some(snippetTextLocation(callSite)),
		startLine: None[int](),
		labels:    snippetLabels,
	})
}

func newByteVisualization(text string, start int, end int) byteVisualization {
	const contextBytes = 3
	lo := max(0, start-contextBytes)
	hi := min(len(text), end+contextBytes)

	var b strings.Builder
	starts := make([]int, hi-lo)
	ends := make([]int, hi-lo)
	if lo > 0 {
		b.WriteString("..., ")
	}
	for i := lo; i < hi; i++ {
		if i > lo {
			b.WriteString(", ")
		}
		starts[i-lo] = b.Len()
		b.WriteString(byteToken(text[i]))
		ends[i-lo] = b.Len()
	}
	if hi < len(text) {
		b.WriteString(", ...")
	}
	return byteVisualization{text: b.String(), starts: starts, ends: ends, sourceBase: lo}
}

func (v byteVisualization) position(sourceOffset int) int {
	return v.starts[sourceOffset-v.sourceBase]
}

func (v byteVisualization) endPosition(sourceOffset int) int {
	return v.ends[sourceOffset-v.sourceBase]
}

func replacementLabel(span ranges.Span[int]) string {
	if span.End() == span.Start()+1 {
		return fmt.Sprintf("replacing index %d", span.Start())
	}
	return fmt.Sprintf("replacing indexes %d-%d", span.Start(), span.End()-1)
}

func byteToken(b byte) string {
	if 0x20 <= b && b <= 0x7e && b != '\'' && b != '\\' {
		return fmt.Sprintf("'%c' (0x%02x)", b, b)
	}
	return fmt.Sprintf("0x%02X", b)
}

// spanBoundLabel returns the label denoting a [Start n | End n] value.
func spanBoundLabel(bound ranges.Bound, offset int) string {
	switch bound {
	case ranges.Bound_Start:
		return fmt.Sprintf("label.span.start = %d", offset)
	case ranges.Bound_End:
		return fmt.Sprintf("label.span.end = %d", offset)
	default:
		return assert.PanicUnknownCase[string](bound)
	}
}

func snippetTextLocation(callSite Option[snippetCallSite]) errorx.Phrase {
	if callSite, ok := callSite.Get(); ok {
		return errorx.NewPhrase(fmt.Sprintf("snippet text (for TextSnippetBuilder.Build at %s:%d)", callSite.file.String(), callSite.line))
	}
	return errorx.NewPhrase("snippet text")
}

func captureSnippetCallSite() Option[snippetCallSite] {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return None[snippetCallSite]()
	}
	return Some(snippetCallSite{file: fsx_name.New(filepath.Base(file)), line: line})
}
