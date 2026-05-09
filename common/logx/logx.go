// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package logx provides a configured structured logger.
// All logging in this project should use a logger obtained from this package.
package logx

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	charmlog "github.com/charmbracelet/log" //nolint:depguard // logx is the designated wrapper
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// Level is a logging severity.
type Level = charmlog.Level

// Re-exported log levels so callers don't need to import charmbracelet/log.
const (
	// Level_Trace is below Level_Debug and is used for very high-volume output
	// such as captured subprocess stdout/stderr.
	Level_Trace Level = -8
	Level_Debug       = charmlog.DebugLevel
	Level_Info        = charmlog.InfoLevel
	Level_Warn        = charmlog.WarnLevel
	Level_Error       = charmlog.ErrorLevel
	Level_Fatal       = charmlog.FatalLevel
)

// Logger is an interface to represent
//
// TODO: This should be refactored to use zap-style concrete types
// for the keyvals instead of ...any.
type Logger interface {
	GetLevel() Level
	With(keyvals ...any) Logger
	Error(msg string, keyvals ...any)
	Warn(msg string, keyvals ...any)
	Info(msg string, keyvals ...any)
	Debug(msg string, keyvals ...any)
	Trace(msg string, keyvals ...any)
}

// ColorSupport controls whether the logger emits ANSI colors.
type ColorSupport int

const (
	ColorSupport_Enable     ColorSupport = iota + 1
	ColorSupport_AutoDetect              // detect based on whether w is a TTY
	ColorSupport_Disable
)

// NewLogger creates a configured logger writing to w.
func NewLogger(w io.Writer, cs ColorSupport) Logger {
	color := false
	switch cs {
	case ColorSupport_Enable:
		color = true
	case ColorSupport_AutoDetect:
		if f, ok := w.(*os.File); ok {
			color = term.IsTerminal(int(f.Fd()))
		}
	case ColorSupport_Disable:
		color = false
	}
	logger := charmlog.NewWithOptions(w, charmlog.Options{ //nolint:exhaustruct // only overriding what we need
		ReportTimestamp: true,
		Level:           charmlog.InfoLevel,
	})

	type levelDef struct {
		level charmlog.Level
		name  string
		fg    string // ANSI color number, only used when color=true
	}
	levels := []levelDef{
		{charmlog.DebugLevel, "DEBUG", "63"},
		{charmlog.InfoLevel, "INFO", "86"},
		{charmlog.WarnLevel, "WARN", "192"},
		{charmlog.ErrorLevel, "ERROR", "204"},
		{charmlog.FatalLevel, "FATAL", "134"},
	}

	styles := charmlog.DefaultStyles()
	for _, l := range levels {
		s := lipgloss.NewStyle().SetString(l.name).MaxWidth(5)
		if color {
			s = s.Bold(true).Foreground(lipgloss.Color(l.fg))
		}
		styles.Levels[l.level] = s
	}
	logger.SetStyles(styles)

	if color {
		logger.SetColorProfile(termenv.ANSI256)
	} else {
		logger.SetColorProfile(termenv.Ascii)
	}

	return &consoleLogger{inner: logger}
}

type consoleLogger struct {
	inner *charmlog.Logger
}

func (l *consoleLogger) GetLevel() Level { return l.inner.GetLevel() }

func (l *consoleLogger) With(keyvals ...any) Logger {
	return &consoleLogger{inner: l.inner.With(keyvals...)}
}

func (l *consoleLogger) Error(msg string, keyvals ...any) { l.inner.Error(msg, keyvals...) }
func (l *consoleLogger) Warn(msg string, keyvals ...any)  { l.inner.Warn(msg, keyvals...) }
func (l *consoleLogger) Info(msg string, keyvals ...any)  { l.inner.Info(msg, keyvals...) }
func (l *consoleLogger) Debug(msg string, keyvals ...any) { l.inner.Debug(msg, keyvals...) }

func (l *consoleLogger) Trace(msg string, keyvals ...any) {
	if l.inner.GetLevel() <= Level_Trace {
		l.inner.Debug(msg, keyvals...)
	}
}
