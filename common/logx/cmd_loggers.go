// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package logx

import (
	"bytes"
	"fmt"
	"io"
)

type TraceLogger interface {
	Trace(msg string, keyvals ...any)
	GetLevel() Level
}

func CmdLoggers(logger TraceLogger, command fmt.Stringer) (io.Writer, io.Writer) {
	if logger.GetLevel() > Level_Trace {
		return io.Discard, io.Discard
	}
	stdout := newLineLogger(func(msg []byte) {
		logger.Trace(string(msg), "cmd", command, "stream", "stdout")
	})
	stderr := newLineLogger(func(msg []byte) {
		logger.Trace(string(msg), "cmd", command, "stream", "stderr")
	})
	return stdout, stderr
}

// FlushLogWriter flushes a log writer when it supports Flush.
func FlushLogWriter(w io.Writer) {
	if flusher, ok := w.(interface{ Flush() }); ok {
		flusher.Flush()
	}
}

type lineLogger struct {
	logLine func([]byte)
	buf     bytes.Buffer
}

func newLineLogger(logLine func([]byte)) *lineLogger {
	return &lineLogger{logLine: logLine, buf: bytes.Buffer{}}
}

func (l *lineLogger) Write(p []byte) (int, error) {
	// Can technically return io.ErrTooLarge on potential OOM
	if n, err := l.buf.Write(p); err != nil {
		return n, err
	}
	for {
		idx := bytes.IndexByte(l.buf.Bytes(), '\n')
		if idx < 0 {
			break
		}
		line := l.buf.Next(idx)
		_ = l.buf.Next(1) // consume newline
		l.logLine(bytes.TrimSuffix(line, []byte("\r")))
	}
	return len(p), nil
}

func (l *lineLogger) Flush() {
	if l.buf.Len() == 0 {
		return
	}
	l.logLine(bytes.TrimSuffix(l.buf.Bytes(), []byte("\r")))
	l.buf.Reset()
}
