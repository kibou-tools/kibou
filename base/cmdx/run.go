// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cmdx

import (
	"code.kibou.tools/base/cancel"
	"code.kibou.tools/base/envx"
	"code.kibou.tools/base/logx"
)

// RunOutput contains output captured from a command invocation.
type RunOutput struct {
	Stdout string
	Stderr string
}

// RunOptions configures BaseRunner.Run behavior.
type RunOptions struct {
	CaptureStdout bool
	CaptureStderr bool
	TransformEnv  func(envx.Env) envx.Env
}

// RunOptionsDefault returns default options for BaseRunner.Run.
func RunOptionsDefault() RunOptions {
	return RunOptions{CaptureStdout: false, CaptureStderr: false, TransformEnv: nil}
}

// WithCaptureStdout returns a copy with CaptureStdout set.
func (o RunOptions) WithCaptureStdout() RunOptions {
	o.CaptureStdout = true
	return o
}

// WithCaptureStderr returns a copy with CaptureStderr set.
func (o RunOptions) WithCaptureStderr() RunOptions {
	o.CaptureStderr = true
	return o
}

type RunCtx interface {
	cancel.Token
	Debug(msg string, keyvals ...any)
	Trace(msg string, keyvals ...any)
	GetLevel() logx.Level
}

// BaseRunner executes a single command.
//
// Run is intended for non-streaming use cases. If we later need streaming
// capture, we can add a lower-level API and implement Run on top of it.
type BaseRunner interface {
	// Run runs a command.
	//
	// The return value contains captured stdout and/or stderr,
	// based on options.
	//
	// CAUTION: If options.CaptureStdout is true, there may be
	// the returned RunOutput.Stdout may be non-empty even if
	// err != nil.
	Run(_ RunCtx, _ Cmd, options RunOptions) (RunOutput, error)
}

// Runner executes single commands and sequential command lists.
type Runner interface {
	BaseRunner
	// ExecAll runs cmds sequentially with default options, stopping at
	// the first error.
	ExecAll(_ RunCtx, cmds ...Cmd) error
}

func BaseRunnerExecAll(runner BaseRunner, ctx RunCtx, cmds ...Cmd) error {
	for _, cmd := range cmds {
		if _, err := runner.Run(ctx, cmd, RunOptionsDefault()); err != nil {
			return err
		}
	}
	return nil
}
