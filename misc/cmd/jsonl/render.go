// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"io"

	"code.kibou.tools/base/logx"
	"code.kibou.tools/misc/internal/go_test"
)

// renderGoTest renders a go test -json stream to out, wiring this command's
// warning accounting and normalized JSONL sink into [go_test.Render] as hooks.
func renderGoTest(logger logx.Logger, z go_test.Colorizer, r io.Reader, pretty io.Writer, jsonl io.Writer) error {
	warnings := newRenderWarnings()
	return go_test.Render(r, go_test.RenderOutputs{Pretty: pretty, JSONL: jsonl}, z, go_test.Hooks{
		OnNonJSONLine: func(lineNumber int, _ []byte, err error) error {
			warnings.warnNotJSON(logger, lineNumber, err)
			return nil
		},
		OnEvent: func(_ []byte, event go_test.Event) error {
			warnings.warnUnknownEnums(logger, event)
			return nil
		},
		BeforeSummary: func(w io.Writer) error {
			return warnings.writeSummary(w, z)
		},
		OnFinish: nil,
	})
}
