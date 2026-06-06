// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package logx

import (
	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/cancel"
)

// LogCtx carries a logger and cancellation token together.
type LogCtx struct {
	cancel.Token
	// Always non-nil.
	Logger
}

// NewLogCtx constructs a LogCtx from a token and logger.
func NewLogCtx(tok cancel.Token, logger Logger) LogCtx {
	assert.Precondition(logger != nil, "logger must be non-nil")
	return LogCtx{Token: tok, Logger: logger}
}

// IsDebugEnabled reports whether debug-level logs are enabled.
func (ctx LogCtx) IsDebugEnabled() bool {
	return ctx.GetLevel() <= Level_Debug
}
