// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package go_test

import (
	"fmt"
	"io"

	. "code.kibou.tools/base/core"
)

// Color is an ANSI text color the renderer can apply to a line.
type Color int

const (
	ColorRed Color = iota
	ColorYellow
	ColorGreen
)

const colorReset = "\x1b[0m"

func (c Color) ansi() string {
	switch c {
	case ColorRed:
		return "\x1b[31m"
	case ColorYellow:
		return "\x1b[33m"
	case ColorGreen:
		return "\x1b[32m"
	default:
		return ""
	}
}

// Colorizer writes text with an optional color, honoring whether color output
// is enabled for the destination stream.
type Colorizer struct {
	enabled bool
}

// NewColorizer returns a Colorizer that emits ANSI color codes only when
// enabled is true.
func NewColorizer(enabled bool) Colorizer {
	return Colorizer{enabled: enabled}
}

func (z Colorizer) Write(w io.Writer, text string, color Option[Color]) error {
	if c, ok := color.Get(); z.enabled && ok {
		_, err := fmt.Fprint(w, c.ansi(), text, colorReset)
		return err
	}
	_, err := fmt.Fprint(w, text)
	return err
}
