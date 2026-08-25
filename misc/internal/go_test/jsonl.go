// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package go_test

import (
	"bufio"
	"encoding/json"
	"io"

	"code.kibou.tools/base/errorx"
)

// RawOutputMarker is the key added to a normalized JSONL event synthesized
// from an input line that was not valid JSON.
const RawOutputMarker = "_raw_output"

type normalizedJSONLWriter struct {
	writer   *bufio.Writer
	lastTime string
	pending  []string
}

func newNormalizedJSONLWriter(w io.Writer) *normalizedJSONLWriter {
	if w == io.Discard {
		return &normalizedJSONLWriter{writer: nil, lastTime: "", pending: nil}
	}
	return &normalizedJSONLWriter{writer: bufio.NewWriter(w), lastTime: "", pending: nil}
}

func (w *normalizedJSONLWriter) recordEvent(line []byte, eventTime string) error {
	if w.writer == nil {
		return nil
	}
	if w.lastTime == "" && eventTime != "" && len(w.pending) > 0 {
		for _, raw := range w.pending {
			if err := w.writeWrapped(raw, eventTime); err != nil {
				return err
			}
		}
		w.pending = nil
	}
	if eventTime != "" {
		w.lastTime = eventTime
	}
	return w.writeLineBytes(line)
}

func (w *normalizedJSONLWriter) recordRaw(raw []byte) error {
	if w.writer == nil {
		return nil
	}
	rawString := string(raw)
	if w.lastTime == "" {
		w.pending = append(w.pending, rawString)
		return nil
	}
	return w.writeWrapped(rawString, w.lastTime)
}

func (w *normalizedJSONLWriter) finish() error {
	if w.writer == nil {
		return nil
	}
	if len(w.pending) > 0 {
		return errorx.Newf("nostack",
			"jsonl output: stream produced no timestamped go test event to anchor %d leading output line(s)", len(w.pending))
	}
	return nil
}

func (w *normalizedJSONLWriter) flush() error {
	if w.writer == nil {
		return nil
	}
	return w.writer.Flush()
}

func (w *normalizedJSONLWriter) writeWrapped(raw string, eventTime string) error {
	encoded, err := json.Marshal(map[string]any{
		"Action":        string(Action_Output),
		"Time":          eventTime,
		"Output":        raw + "\n",
		RawOutputMarker: true,
	})
	if err != nil {
		return errorx.Wrapf("nostack", err, "marshal raw JSONL output")
	}
	return w.writeLine(string(encoded))
}

func (w *normalizedJSONLWriter) writeLine(line string) error {
	return w.writeLineBytes([]byte(line))
}

func (w *normalizedJSONLWriter) writeLineBytes(line []byte) error {
	if _, err := w.writer.Write(line); err != nil {
		return errorx.Wrapf("nostack", err, "write JSONL output")
	}
	if err := w.writer.WriteByte('\n'); err != nil {
		return errorx.Wrapf("nostack", err, "write JSONL output")
	}
	return nil
}
