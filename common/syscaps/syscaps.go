// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package syscaps provides controlled access to ambient system capabilities.
package syscaps

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	stdlib_time "time"

	"github.com/spf13/afero" //nolint:depguard // syscaps is the ambient-authority boundary

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/cmdx"
	"code.kibou.tools/common/core/pair"
	"code.kibou.tools/common/core/pathx"
	"code.kibou.tools/common/envx"
	"code.kibou.tools/common/errorx"
	"code.kibou.tools/common/fsx"
	"code.kibou.tools/common/logx"
	"code.kibou.tools/common/timex"
)

// Env returns the current process environment.
func Env() envx.Env {
	return envx.NewIgnoringDupes(func(yield func(pair.KeyValue[string, string]) bool) {
		for _, entry := range os.Environ() { //nolint:forbidigo // syscaps is the ambient-authority boundary
			key, value, ok := strings.Cut(entry, "=")
			assert.Postconditionf(ok, "os.Environ entry missing '=': %q", entry)
			if !yield(pair.NewKeyValue(key, value)) {
				return
			}
		}
	})
}

// WorkingDirectory returns the process working directory.
func WorkingDirectory() (pathx.AbsPath, error) {
	wd, err := os.Getwd()
	if err != nil {
		return pathx.AbsPath{}, err
	}
	return pathx.MustParseAbsPath(wd), nil
}

// FS returns a rooted filesystem backed by the host operating system.
func FS(root pathx.AbsPath) (fsx.FS, error) {
	base, ok := afero.NewOsFs().(fsx.BaseFS)
	assert.Invariantf(ok, "NewOsFs return value should implement fsx.BaseFS, but got type %T", base)
	return fsx.NewRootedFS(root, base)
}

// CmdRunner executes commands using ambient system capabilities.
type CmdRunner struct {
	Env envx.Env
}

func (runner CmdRunner) Run(ctx cmdx.RunCtx, cmd cmdx.Cmd, options cmdx.RunOptions) (cmdx.RunOutput, error) {
	dir, hasDir := cmd.Dir().Get()
	if hasDir {
		ctx.Debug("running command", "cmd", cmd, "dir", dir.String())
	} else {
		ctx.Debug("running command", "cmd", cmd)
	}

	stdout, stderr := logx.CmdLoggers(ctx, cmd)
	defer logx.FlushLogWriter(stdout)
	defer logx.FlushLogWriter(stderr)

	argv := cmd.Argv()
	execCmd := exec.CommandContext(ctx.AsStdlibContext(), argv[0], argv[1:]...)
	if hasDir {
		execCmd.Dir = dir.String()
	}
	if options.TransformEnv != nil {
		execCmd.Env = options.TransformEnv(runner.Env).Entries()
	} else {
		execCmd.Env = runner.Env.Entries()
	}

	var capturedStdout bytes.Buffer
	var capturedStderr bytes.Buffer
	if options.CaptureStdout {
		execCmd.Stdout = io.MultiWriter(stdout, &capturedStdout)
	} else {
		execCmd.Stdout = stdout
	}
	if options.CaptureStderr {
		execCmd.Stderr = io.MultiWriter(stderr, &capturedStderr)
	} else {
		execCmd.Stderr = stderr
	}

	capturedOutput := func() cmdx.RunOutput {
		return cmdx.RunOutput{Stdout: capturedStdout.String(), Stderr: capturedStderr.String()}
	}
	if err := execCmd.Run(); err != nil {
		return capturedOutput(), errorx.Wrapf("+stacks", err, "%s", cmd)
	}
	return capturedOutput(), nil
}

func (runner CmdRunner) ExecAll(ctx cmdx.RunCtx, cmds ...cmdx.Cmd) error {
	return cmdx.BaseRunnerExecAll(runner, ctx, cmds...)
}

func TimestampClock() timex.TimestampClock {
	return systemClock{}
}

func MonotonicClock() timex.MonotonicClock {
	return systemClock{}
}

func Scheduler() timex.Scheduler {
	return systemClock{}
}

type systemClock struct{}

type systemScheduledFunc struct {
	// Always non-nil.
	timer *stdlib_time.Timer
}

func (s systemClock) GetTimestamp() timex.Timestamp {
	return timex.NewTimestamp(stdlib_time.Now().UTC())
}

func (s systemClock) GetInstant() timex.Instant {
	return timex.NewInstant(stdlib_time.Now())
}

func (s systemClock) RunAfter(d timex.Duration, f func()) timex.ScheduledFunc {
	assert.Precondition(f != nil, "scheduled function must be non-nil")
	return systemScheduledFunc{timer: stdlib_time.AfterFunc(d, f)} //nolint:forbidigo // syscaps is the ambient scheduler boundary
}

func (s systemScheduledFunc) Stop() timex.StopResult {
	if s.timer.Stop() {
		return timex.StopResult_Stopped
	}
	return timex.StopResult_TooLate
}
