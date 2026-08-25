// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package go_test

import (
	"fmt"
	"io"
	"strings"

	. "code.kibou.tools/base/core"
)

// summary collects the failures (and a skip count) seen across the stream and
// prints a "# go test summary" block at the end.
type summary struct {
	results []failure
	passed  int
	skipped int
}

func (s *summary) record(event TestEvent) {
	switch event.Action {
	case Action_Fail:
		s.results = append(s.results, failure{
			Action:      event.Action,
			Package:     event.Package,
			Test:        event.Test,
			Elapsed:     event.Elapsed,
			FailedBuild: event.FailedBuild,
		})
	case Action_Pass:
		// event.Test is not set for package-level entries.
		if event.Test != "" {
			s.passed++
		}
	case Action_Skip:
		// Count only test-level skips. Package-level skips ("no test files")
		// are not actionable and, across a std-library run, would otherwise
		// drown the failures they sit beside.
		if event.Test != "" {
			s.skipped++
		}
	case Action_Start, Action_Run, Action_Pause, Action_Cont,
		Action_Bench, Action_Output, Action_Attr,
		Action_Artifacts, Action_BuildOutput, Action_BuildFail:
	default:
	}
}

func (s *summary) write(w io.Writer, z Colorizer) error {
	if len(s.results) == 0 && s.passed == 0 && s.skipped == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# go test summary"); err != nil {
		return err
	}
	for _, result := range s.results {
		if err := z.Write(w, result.line(), Some(ColorRed)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if s.passed > 0 {
		if err := z.Write(w, fmt.Sprintf("(%d passed)", s.passed), Some(ColorGreen)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if s.skipped > 0 {
		if err := z.Write(w, fmt.Sprintf("(%d skipped)", s.skipped), Some(ColorYellow)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

// failure is one failure recorded for the summary.
type failure struct {
	Action      TestAction
	Package     string
	Test        string
	Elapsed     *float64
	FailedBuild string
}

func (r failure) line() string {
	status := strings.ToUpper(string(r.Action))
	if r.Test != "" {
		return fmt.Sprintf("%s %s %s (%s)", status, r.Package, r.Test, formatElapsed(r.Elapsed, 2))
	}
	line := fmt.Sprintf("%s %s (%s)", status, r.Package, formatElapsed(r.Elapsed, 3))
	if r.FailedBuild != "" {
		line += " [build failed: " + r.FailedBuild + "]"
	}
	return line
}

func formatElapsed(elapsed *float64, precision int) string {
	if elapsed == nil {
		return "?s"
	}
	return fmt.Sprintf("%.*fs", precision, *elapsed)
}
