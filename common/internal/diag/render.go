// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package diag

import (
	"bufio"
	"io"

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/core/option"
)

// RenderPretty writes a multiline rendering of the diagnostic d to
// the writer w using the style s.
//
// The default diagnostic format looks like:
//
//	<severity>: <message> [(<code>)]
//	│          <wrapped message continuation>
//	│
//	│  <source line containing the failure>
//	│  <caret/underline pointing at the failure>
//	│
//	│ <hint label>: <msg>
//	│   <wrapped msg continuation>
//	│
//	│ <see-also-prefix><see-also>
//
// with no newline at the start, and a newline at the end.
func RenderPretty[C Code](w io.Writer, d Diagnostic[C], s Style, opts RenderPrettyOptions) error {
	if maxWidth, ok := opts.MaxWidth.Get(); ok && maxWidth == 0 {
		assert.Preconditionf(false, "MaxWidth must be positive, got %d", maxWidth)
	}
	if opts.TabWidth == 0 {
		assert.Preconditionf(false, "TabWidth must be positive, got %d", opts.TabWidth)
	}

	doc := buildPrettyDoc(d, opts)
	return writeDoc(w, doc, s)
}

type RenderPrettyOptions struct {
	// MaxWidth must be positive.
	MaxWidth option.Option[uint8]
	// TabWidth must be positive.
	TabWidth uint8
	// SeeAlsoPrefix is written verbatim before [Code.SeeAlso]'s value.
	SeeAlsoPrefix string
}

const renderPrettyDefaultSeeAlsoPrefix = "For more details, see "

func RenderPrettyOptionsDefault() RenderPrettyOptions {
	return RenderPrettyOptions{MaxWidth: option.None[uint8](), TabWidth: 4, SeeAlsoPrefix: renderPrettyDefaultSeeAlsoPrefix}
}

func writeDoc(w_ io.Writer, d prettyDoc, s Style) error {
	// A typical diagnostic will have a message, some code snippet,
	// and some hint. So this adds up to ~80 chars per row at most.
	// Considering frame + newlines + rounding up to a power of 2 -> 512.
	w := bufio.NewWriterSize(w_, 512)
	for _, line := range d.lines {
		if line.hasFrame {
			if _, err := io.WriteString(w, s.Wrap(Role_Frame, frameText)); err != nil {
				return err
			}
		}
		for _, fragment := range line.fragments {
			text := fragment.text
			if role, ok := fragment.role.Get(); ok {
				text = s.Wrap(role, fragment.text)
			}
			if _, err := io.WriteString(w, text); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return w.Flush()
}
