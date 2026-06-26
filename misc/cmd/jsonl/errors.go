// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"fmt"
	"io"

	. "code.kibou.tools/base/core"
	"code.kibou.tools/base/core/limited"
	"code.kibou.tools/base/core/op"
	"code.kibou.tools/base/logx"
	"code.kibou.tools/misc/internal/go_test"
)

type renderWarnings [3]Warning

type Warning struct {
	kind WarningKind
	limited.Counter[uint32]
}

type WarningKind int

// These values are used directly as indexes into renderWarnings, so they
// deliberately start at iota (0) rather than iota+1.
const (
	Warn_NotJSON WarningKind = iota
	Warn_UnknownAction
	Warn_UnknownOutputType
)

const renderWarningLimit uint32 = 3

func newRenderWarnings() renderWarnings {
	var warnings renderWarnings
	for i := range len(warnings) {
		warnings[i] = Warning{kind: WarningKind(i), Counter: limited.NewCounter(renderWarningLimit)}
	}
	return warnings
}

func (kind WarningKind) logMessage() string {
	switch kind {
	case Warn_NotJSON:
		return "passing through non-JSON go test line"
	case Warn_UnknownAction:
		return "unknown go test JSON action"
	case Warn_UnknownOutputType:
		return "unknown go test JSON output type"
	default:
		return "unknown jsonl warning"
	}
}

func (kind WarningKind) summaryLabel() string {
	switch kind {
	case Warn_NotJSON:
		return "Non-JSON lines"
	case Warn_UnknownAction:
		return "Unknown go test JSON actions"
	case Warn_UnknownOutputType:
		return "Unknown go test JSON output types"
	default:
		return "Unknown jsonl warnings"
	}
}

func (warning Warning) summaryStats() string {
	total := warning.Hits()
	omitted := warning.Dropped()
	logged := total - omitted
	if omitted == 0 {
		return fmt.Sprintf("(logged: %d/%d)", logged, total)
	}
	return fmt.Sprintf("(logged: %d/%d, omitted: %d/%d for brevity)", logged, total, omitted, total)
}

func (warnings *renderWarnings) warnNotJSON(logger logx.Logger, lineNumber int, err error) {
	if warnings[Warn_NotJSON].Inc() == op.KeepGoing {
		logger.Warn(Warn_NotJSON.logMessage(), "line", lineNumber, "err", err.Error())
	}
}

func (warnings *renderWarnings) warnUnknownEnums(logger logx.Logger, event go_test.Event) {
	if !event.Action.Known() {
		if warnings[Warn_UnknownAction].Inc() == op.KeepGoing {
			logger.Warn(Warn_UnknownAction.logMessage(), "action", string(event.Action))
		}
	}
	if !event.OutputType.Known() {
		if warnings[Warn_UnknownOutputType].Inc() == op.KeepGoing {
			logger.Warn(Warn_UnknownOutputType.logMessage(), "output_type", string(event.OutputType))
		}
	}
}

func (warnings *renderWarnings) writeSummary(w io.Writer, z go_test.Colorizer) error {
	wroteHeader := false
	for _, warning := range warnings {
		if warning.Hits() == 0 {
			continue
		}
		if !wroteHeader {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, "jsonl warnings:"); err != nil {
				return err
			}
			wroteHeader = true
		}
		if _, err := fmt.Fprint(w, "- "); err != nil {
			return err
		}
		if err := z.Write(w, warning.kind.summaryLabel(), Some(go_test.ColorYellow)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, " %s\n", warning.summaryStats()); err != nil {
			return err
		}
	}
	return nil
}
