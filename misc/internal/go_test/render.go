// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package go_test

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"code.kibou.tools/base/collections"
	. "code.kibou.tools/base/core"
	"code.kibou.tools/base/errorx"
)

type RenderOutputs struct {
	Pretty io.Writer
	JSONL  io.Writer
}

type Hooks struct {
	OnNonJSONLine func(lineNumber int, raw []byte, err error) error
	OnEvent       func(raw []byte, event TestEvent) error
	BeforeSummary func(w io.Writer) error
	OnFinish      func() error
}

func Render(r io.Reader, outputs RenderOutputs, z Colorizer, hooks Hooks) (retErr error) {
	pretty := bufio.NewWriter(outputs.Pretty)
	jsonl := newNormalizedJSONLWriter(outputs.JSONL)
	defer func() {
		retErr = errorx.Join(retErr, pretty.Flush(), jsonl.flush())
	}()

	reader := bufio.NewReader(r)
	sum := summary{results: nil, passed: 0, skipped: 0}
	renderer := NewPlainTextRenderer(pretty, z)
	lineNumber := 0

	for {
		lineBytes, err := reader.ReadBytes('\n')
		if len(lineBytes) > 0 {
			lineNumber++
			if lineBytes[len(lineBytes)-1] == '\n' {
				lineBytes = lineBytes[:len(lineBytes)-1]
			}
			if len(lineBytes) > 0 && lineBytes[len(lineBytes)-1] == '\r' {
				lineBytes = lineBytes[:len(lineBytes)-1]
			}

			var event TestEvent
			if decodeErr := json.Unmarshal(lineBytes, &event); decodeErr != nil {
				if hookErr := hooks.onNonJSONLine(lineNumber, lineBytes, decodeErr); hookErr != nil {
					return hookErr
				}
				if jsonlErr := jsonl.recordRaw(lineBytes); jsonlErr != nil {
					return jsonlErr
				}
				if _, writeErr := pretty.Write(lineBytes); writeErr != nil {
					return writeErr
				}
				if _, writeErr := io.WriteString(pretty, "\n"); writeErr != nil {
					return writeErr
				}
			} else {
				if hookErr := hooks.onEvent(lineBytes, event); hookErr != nil {
					return hookErr
				}
				if jsonlErr := jsonl.recordEvent(lineBytes, event.Time); jsonlErr != nil {
					return jsonlErr
				}
				sum.record(event)
				if renderErr := renderer.Handle(event); renderErr != nil {
					return renderErr
				}
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			if jsonlErr := jsonl.finish(); jsonlErr != nil {
				return jsonlErr
			}
			if hookErr := hooks.onFinish(); hookErr != nil {
				return hookErr
			}
			if err := renderer.Finish(); err != nil {
				return err
			}
			if hookErr := hooks.beforeSummary(pretty); hookErr != nil {
				return hookErr
			}
			return sum.write(pretty, z)
		}
		return errorx.Wrapf("nostack", err, "read go test JSONL")
	}
}

func (h Hooks) onNonJSONLine(lineNumber int, raw []byte, err error) error {
	if h.OnNonJSONLine == nil {
		return nil
	}
	return h.OnNonJSONLine(lineNumber, raw, err)
}

func (h Hooks) onEvent(raw []byte, event TestEvent) error {
	if h.OnEvent == nil {
		return nil
	}
	return h.OnEvent(raw, event)
}

func (h Hooks) beforeSummary(w io.Writer) error {
	if h.BeforeSummary == nil {
		return nil
	}
	return h.BeforeSummary(w)
}

func (h Hooks) onFinish() error {
	if h.OnFinish == nil {
		return nil
	}
	return h.OnFinish()
}

type PlainTextRenderer struct {
	writer    io.Writer
	colorizer Colorizer
	buffers   collections.OrderedMap[bufKey, []renderLine]
}

type bufKey struct {
	pkg  string
	test string
}

type renderLine struct {
	text  string
	color Option[Color]
}

func NewPlainTextRenderer(w io.Writer, z Colorizer) *PlainTextRenderer {
	return &PlainTextRenderer{
		writer:    w,
		colorizer: z,
		buffers:   collections.NewOrderedMap[bufKey, []renderLine](),
	}
}

func (tr *PlainTextRenderer) Handle(event TestEvent) error {
	switch event.Action {
	case Action_BuildOutput:
		return tr.colorizer.Write(tr.writer, event.Output, None[Color]())
	case Action_Output:
		tr.bufferOutput(event)
		return nil
	case Action_Pass, Action_Fail, Action_Skip, Action_Bench:
		return tr.resolve(event)
	case Action_Start, Action_Run, Action_Pause, Action_Cont,
		Action_Attr, Action_Artifacts, Action_BuildFail:
		return nil
	default:
		return nil
	}
}

func (tr *PlainTextRenderer) Finish() error {
	for _, key := range tr.bufferKeys() {
		if err := tr.dispose(key, true); err != nil {
			return err
		}
	}
	return nil
}

func (tr *PlainTextRenderer) bufferOutput(event TestEvent) {
	key := bufKey{pkg: event.Package, test: event.Test}
	lines := []renderLine(nil)
	if existing, ok := tr.buffers.Lookup(key).Get(); ok {
		lines = existing
	}
	color := None[Color]()
	if event.OutputType == OutputType_Error || event.OutputType == OutputType_ErrorContinue {
		color = Some(ColorRed)
	}
	tr.buffers.InsertOrReplace(key, append(lines, renderLine{text: event.Output, color: color}))
}

func (tr *PlainTextRenderer) resolve(event TestEvent) error {
	show := event.Action == Action_Fail || event.Action == Action_Bench
	if event.Test != "" {
		return tr.dispose(bufKey{pkg: event.Package, test: event.Test}, show)
	}
	for _, key := range tr.bufferKeys() {
		if key.pkg != event.Package {
			continue
		}
		if err := tr.dispose(key, show || key.test == ""); err != nil {
			return err
		}
	}
	return nil
}

func (tr *PlainTextRenderer) bufferKeys() []bufKey {
	keys := make([]bufKey, 0, tr.buffers.Len())
	for key := range tr.buffers.Keys() {
		keys = append(keys, key)
	}
	return keys
}

func (tr *PlainTextRenderer) dispose(key bufKey, show bool) error {
	lines, _ := tr.buffers.Delete(key).Get()
	if !show {
		return nil
	}
	for _, line := range lines {
		if isHiddenFrame(line.text) {
			continue
		}
		if err := tr.colorizer.Write(tr.writer, line.text, line.color); err != nil {
			return err
		}
	}
	return nil
}

func isHiddenFrame(text string) bool {
	t := strings.TrimSuffix(strings.TrimLeft(text, " "), "\n")
	switch {
	case strings.HasPrefix(t, "=== RUN"),
		strings.HasPrefix(t, "=== PAUSE"),
		strings.HasPrefix(t, "=== CONT"),
		strings.HasPrefix(t, "=== NAME"),
		strings.HasPrefix(t, "--- PASS:"),
		strings.HasPrefix(t, "--- SKIP:"),
		t == "PASS":
		return true
	default:
		return false
	}
}
