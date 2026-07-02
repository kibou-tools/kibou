// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"fmt"
	"io"
	"maps"
	"runtime"
	"slices"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/cmdx"
	. "code.kibou.tools/base/core"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/envx"
	"code.kibou.tools/base/errorx"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/logx"
	"code.kibou.tools/base/timex"
)

type BenchmarkName string

const (
	Benchmark_MakeBash BenchmarkName = "make.bash"
)

func AllBenchmarks() []BenchmarkName {
	return []BenchmarkName{Benchmark_MakeBash}
}

func (b BenchmarkName) DefaultIters() int {
	switch b {
	case Benchmark_MakeBash:
		return 1
	default:
		return assert.PanicUnknownCase[int](b)
	}
}

type VCS string

const (
	JJ  VCS = "jj"
	Git VCS = "git"
)

type RevSet struct {
	VCS  VCS
	Spec string
}

func NewRevSet(vcs VCS, spec string) RevSet {
	assert.Precondition(spec != "", "spec must be non-empty")
	switch vcs {
	case JJ, Git:
		return RevSet{vcs, spec}
	default:
		return assert.PanicUnknownCase[RevSet](vcs)
	}
}

type BenchmarkOptions struct {
	// None => automatic selection
	Iters    Option[int]
	Config   BenchmarkConfig
	Baseline Option[RevSet]
}

type BenchmarkSetup struct {
	Root  pathx.RelPath
	Label string
}

func (ws *Workspace) Benchmark(logCtx logx.LogCtx, clock timex.MonotonicClock, out io.Writer, options BenchmarkOptions) error {
	setups := []BenchmarkSetup{}
	if baseline, ok := options.Baseline.Get(); ok {
		tempDir, err := ws.FS.MkdirTemp(pathx.MustParseRelPath(".cache"), "benchmark-wt")
		if err != nil {
			return err
		}
		defer func() { _ = ws.FS.RemoveAll(tempDir) }()
		// Create new workspace or worktree + defer the cleanup
		switch baseline.VCS {
		case JJ:
			err := runJJ(logCtx, ws.Runner, ws.FS.Root(), "workspace", "add",
				"--revision", baseline.Spec, "--name", "benchmark-baseline", tempDir.String())
			if err != nil {
				return err
			}
			defer func() {
				_ = runJJ(logCtx, ws.Runner, ws.FS.Root(), "workspace", "forget", "benchmark-baseline")
			}()
		case Git:
			err := runGit(logCtx, ws.Runner, ws.FS.Root(), "worktree", "add", tempDir.String(), baseline.Spec)
			if err != nil {
				return err
			}
			defer func() {
				_ = runGit(logCtx, ws.Runner, ws.FS.Root(), "worktree", "remove", tempDir.String())
			}()
		}

		setups = append(setups, BenchmarkSetup{tempDir, "baseline"})
	}
	setups = append(setups, BenchmarkSetup{pathx.Dot(), "latest"})

	_, _ = fmt.Fprintf(out, "Benchmarking %v\n", string(options.Config.Name()))
	for _, setup := range setups {
		iters := options.Iters.ValueOr(options.Config.Name().DefaultIters())
		for i := 1; i <= iters; i++ {
			cacheDir, err := ws.FS.MkdirTemp(pathx.MustParseRelPath(".cache"), "gocache")
			if err != nil {
				return err
			}
			err = func() error {
				defer func() { _ = ws.FS.RemoveAll(cacheDir) }()
				duration, err := options.Config.Run(logCtx, ws, cacheDir, setup.Root, clock)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(out, "%v (iter = %d): t = %v\n", setup.Label, i, duration)
				return nil
			}()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type BenchmarkConfig interface {
	Add(key string, value string) error
	Run(_ cmdx.RunCtx, _ *Workspace, cacheDir, root pathx.RelPath, _ timex.MonotonicClock) (timex.Duration, error)
	Name() BenchmarkName
}

func NewBenchmarkConfig(name BenchmarkName) BenchmarkConfig {
	switch name {
	case Benchmark_MakeBash:
		return &MakeBashBenchmark{TargetGOOS: runtime.GOOS, TargetGOARCH: runtime.GOARCH}
	default:
		return assert.PanicUnknownCase[BenchmarkConfig](name)
	}
}

type MakeBashBenchmark struct {
	TargetGOOS   string
	TargetGOARCH string
}

func (m *MakeBashBenchmark) Add(key string, value string) error {
	allowedOSes := []string{"linux", "darwin", "windows"}
	allowedArchs := []string{"amd64", "arm64"}
	allowedKeys := map[string]Pair[[]string, *string]{
		"TARGET_GOOS":   NewPair(allowedOSes, &m.TargetGOOS),
		"TARGET_GOARCH": NewPair(allowedArchs, &m.TargetGOARCH),
	}
	if want, ok := allowedKeys[key]; ok {
		if slices.Contains(want.First, value) {
			*want.Second = value
			return nil
		}
		return errorx.Newf("nostack", "unsupported value for %s: %s (expected one of %v)", key, value, allowedOSes)
	}
	return errorx.Newf("nostack", "unsupported key: %s (expected one of %v)",
		key, slices.Sorted(maps.Keys(allowedKeys)))
}

func (m *MakeBashBenchmark) Name() BenchmarkName {
	return Benchmark_MakeBash
}

func (m *MakeBashBenchmark) Run(
	ctx cmdx.RunCtx,
	ws *Workspace,
	cacheDir pathx.RelPath,
	root pathx.RelPath,
	clock timex.MonotonicClock,
) (timex.Duration, error) {
	ext := ".bash"
	if runtime.GOOS == "windows" {
		ext = ".bat"
	}
	fs := ws.FS
	runner := ws.Runner
	srcDir := fs.Root().Join(root).Join(pathx.MustParseRelPath("go/src"))

	statOptions := fsx.StatOptions{FollowFinalSymlink: false, OnErrorTraverseParents: true}
	if _, err := fs.Stat(root.Join(pathx.MustParseRelPath("go/bin/go")), statOptions); err == nil {
		output, err := runner.Run(ctx, cmdx.New("./clean"+ext).In(srcDir), commandOpts())
		ctx.Debug("clean output", "stdout", output.Stdout, "stderr", output.Stderr)
		if err != nil {
			return 0, err
		}
	}

	opts := commandOpts()
	opts.TransformEnv = func(env envx.Env) envx.Env {
		env, _ = env.InsertOrReplace("GOOS", m.TargetGOOS)
		env, _ = env.InsertOrReplace("GOARCH", m.TargetGOARCH)
		env, _ = env.InsertOrReplace("GOCACHE", fs.Root().Join(cacheDir).String())
		return env
	}
	start := clock.GetInstant()
	output, err := runner.Run(ctx, cmdx.New("./make"+ext).In(srcDir), opts)
	elapsed := clock.GetInstant().Sub(start)
	ctx.Debug("make output", "stdout", output.Stdout, "stderr", output.Stderr)
	if err != nil {
		return 0, err
	}

	return elapsed, nil
}

func runJJ(ctx cmdx.RunCtx, runner cmdx.BaseRunner, dir pathx.AbsPath, args ...string) error {
	output, err := runner.Run(ctx,
		cmdx.New(append([]string{"jj"}, args...)...).In(dir),
		commandOpts())
	ctx.Debug("jj output", "stdout", output.Stdout, "stderr", output.Stderr)
	return err
}

func runGit(ctx cmdx.RunCtx, runner cmdx.BaseRunner, dir pathx.AbsPath, args ...string) error {
	output, err := runner.Run(ctx,
		cmdx.New(append([]string{"git"}, args...)...).In(dir),
		commandOpts())
	ctx.Debug("git output", "stdout", output.Stdout, "stderr", output.Stderr)
	return err
}

func commandOpts() cmdx.RunOptions {
	return cmdx.RunOptionsDefault().WithCaptureStdout().WithCaptureStderr()
}
