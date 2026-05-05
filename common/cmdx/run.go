// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cmdx

import (
	"github.com/typesanitizer/happygo/common/envx"
	"github.com/typesanitizer/happygo/common/logx"
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

// BaseRunner executes a single command.
//
// Run is intended for non-streaming use cases. If we later need streaming
// capture, we can add a lower-level API and implement Run on top of it.
type BaseRunner interface {
	// Run runs a command.
	//
	// The first return value contains captured output from enabled streams.
	// There may be captured output even in the presence of errors. Stdout and
	// stderr are only populated when their corresponding capture option is true.
	Run(_ logx.LogCtx, _ Cmd, options RunOptions) (RunOutput, error)
}

// Runner executes single commands and sequential command lists.
type Runner interface {
	BaseRunner
	// ExecAll runs cmds sequentially with default options, stopping at
	// the first error.
	ExecAll(_ logx.LogCtx, cmds ...Cmd) error
}

func BaseRunnerExecAll(runner BaseRunner, ctx logx.LogCtx, cmds ...Cmd) error {
	for _, cmd := range cmds {
		if _, err := runner.Run(ctx, cmd, RunOptionsDefault()); err != nil {
			return err
		}
	}
	return nil
}
