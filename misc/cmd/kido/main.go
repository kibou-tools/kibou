// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"context" //nolint:depguard // kido's main is the cli boundary; only place allowed to bridge context.Context to cancel.Token
	"io"
	"os"
	"strings"
	"sync"

	"code.kibou.tools/base/core/pathx"
	"github.com/urfave/cli/v3"

	"code.kibou.tools/base/cancel"
	"code.kibou.tools/base/cancel_bridge"
	"code.kibou.tools/base/cmdx"
	"code.kibou.tools/base/collections"
	. "code.kibou.tools/base/core"
	"code.kibou.tools/base/errorx"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/logx"
	"code.kibou.tools/base/syscaps"
	"code.kibou.tools/base/timex"
	"code.kibou.tools/misc/internal/config"
)

const syncBranchPrefix = "merge-bot/sync/"

// Workspace provides operations over the repository root using the repo configuration.
type Workspace struct {
	FS     fsx.FS
	Config config.WorkspaceConfig
	Runner cmdx.Runner
}

func newWorkspaceFromGit(runner cmdx.Runner) (Workspace, error) {
	repoRootCmd := cmdx.New("git", "rev-parse", "--show-toplevel")
	ctx := logx.NewLogCtx(cancel.Never(), logx.NewLogger(io.Discard, logx.ColorSupport_Disable))
	output, err := runner.Run(ctx, repoRootCmd, cmdx.RunOptionsDefault().WithCaptureStdout())
	if err != nil {
		return Workspace{}, errorx.Wrapf("nostack", err, "determine git repository root")
	}
	repoRoot := MustParseAbsPath(strings.TrimSpace(output.Stdout))
	repoFS, err := syscaps.FS(repoRoot)
	if err != nil {
		return Workspace{}, errorx.Wrapf("+stacks", err, "open repo filesystem at %s", repoRoot)
	}
	wsConfig, err := loadWorkspaceConfig(repoFS)
	return Workspace{FS: repoFS, Config: wsConfig, Runner: runner}, err
}

func main() {
	logger := logx.NewLogger(os.Stderr, logx.ColorSupport_AutoDetect)
	runner := syscaps.CmdRunner{Env: syscaps.Env()}
	getWorkspace := sync.OnceValues(func() (Workspace, error) {
		return newWorkspaceFromGit(runner)
	})
	app := &cli.Command{
		Name:  "kido",
		Usage: "Perform workspace-related administrative tasks",
		Commands: []*cli.Command{
			{
				Name: "sync-branch",
				Usage: "update and optionally push " + syncBranchPrefix +
					"<project> for --project=go|tools|delve|all using a separate worktree",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "project", Required: true},
					&cli.StringFlag{Name: "base"},
					&cli.BoolFlag{Name: "push"},
					&cli.BoolFlag{Name: "persist"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					projectArg, parseErr := pathx.ParseRelPath(cmd.String("project"))
					if parseErr != nil {
						return errorx.Wrapf("nostack", parseErr, "in argument for --project")
					}

					tok, cancel := withTimeout(cancel_bridge.Extract(ctx), 5*timex.Minute, cmd.Name)
					defer cancel()
					ws, projects, err := resolveProjects(getWorkspace, projectArg)
					if err != nil {
						return err
					}
					logCtx := logx.NewLogCtx(tok, logger)
					return ws.runSyncBranch(logCtx, projects, RunSyncBranchOptions{
						Base:    NewOption(cmd.String("base"), cmd.IsSet("base")),
						Push:    cmd.Bool("push"),
						Persist: cmd.Bool("persist"),
					})
				},
			},
			{
				Name: "sync-pr",
				Usage: "create/update PRs for " + syncBranchPrefix +
					"<project> with labels and auto-merge",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "project", Required: true},
					&cli.StringFlag{Name: "base"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					projectArg, parseErr := pathx.ParseRelPath(cmd.String("project"))
					if parseErr != nil {
						return errorx.Wrapf("nostack", parseErr, "in argument for --project")
					}

					tok, cancel := withTimeout(cancel_bridge.Extract(ctx), 5*timex.Minute, cmd.Name)
					defer cancel()
					ws, projects, err := resolveProjects(getWorkspace, projectArg)
					if err != nil {
						return err
					}
					logCtx := logx.NewLogCtx(tok, logger)
					clock := syscaps.TimestampClock()
					return ws.runSyncPR(logCtx, clock, projects, RunSyncPROptions{
						Base: NewOption(cmd.String("base"), cmd.IsSet("base")),
					})
				},
			},
			{
				Name:  "list",
				Usage: "list workspace items",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name: "type", Required: true,
						Usage: "item type: go-module",
					},
					&cli.StringFlag{
						Name:  "provenance",
						Usage: "provenance filter: first-party, forked (default: all)",
					},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					ws, err := getWorkspace()
					if err != nil {
						return err
					}
					var type_ ListType
					switch cmd.String("type") {
					case "go-module":
						type_ = ListType_GoModules
					default:
						return errorx.Newf("nostack",
							"unknown --type %q, want go-module", cmd.String("type"))
					}
					provenance := ListProvenance_All
					if cmd.IsSet("provenance") {
						switch cmd.String("provenance") {
						case "first-party":
							provenance = ListProvenance_FirstParty
						case "forked":
							provenance = ListProvenance_Forked
						default:
							return errorx.Newf("nostack",
								"unknown --provenance %q, want first-party|forked", cmd.String("provenance"))
						}
					}
					return ws.List(logger, os.Stdout, ListOptions{Type: type_, Provenance: provenance})
				},
			},
			{
				Name:  "benchmark",
				Usage: "Run some benchmark for the workspace",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name: "name", Required: true,
						Usage: "Possible values: make.bash",
					},
					&cli.IntFlag{
						Name: "iters", Required: false,
						Usage: "Number of iterations. 0 implies automatic selection.",
					},
					&cli.StringSliceFlag{
						Name: "config", Required: false,
						Usage: "Benchmark specific config option as key=value (repeatable).",
					},
					&cli.StringFlag{
						Name: "baseline-rev", Required: false,
						Usage: "jj:<revset> or git:<sha>",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					ws, err := getWorkspace()
					if err != nil {
						return err
					}
					var name BenchmarkName
					switch cmd.String("name") {
					case "make.bash":
						name = "make.bash"
					default:
						return errorx.Newf("nostack",
							"unknown --name %q, want one of %v", cmd.String("name"),
							AllBenchmarks())
					}
					var iters Option[int]
					if cmd.IsSet("iters") {
						val := cmd.Int("iters")
						if val < 0 {
							return errorx.Newf("nostack", "negative --iters %d, wanted value >= 0", val)
						}
						if val > 0 {
							iters = Some(val)
						}
					}
					config := NewBenchmarkConfig(name)
					for _, kv := range cmd.StringSlice("config") {
						before, after, ok := strings.Cut(kv, "=")
						if !ok {
							return errorx.Newf("nostack", "invalid --config %q; missing = separator", kv)
						}
						if before == "" {
							return errorx.Newf("nostack", "invalid --config %q; missing key before =", kv)
						}
						if after == "" {
							return errorx.Newf("nostack", "invalid --config %q; missing value after =", kv)
						}
						if err := config.Add(before, after); err != nil {
							return err
						}
					}
					var baseline Option[RevSet]
					if cmd.IsSet("baseline-rev") {
						val := cmd.String("baseline-rev")
						before, after, ok := strings.Cut(val, ":")
						if !ok {
							return errorx.Newf("nostack", "invalid --baseline-rev %q; missing : separator", val)
						}
						if before != "jj" && before != "git" {
							return errorx.Newf("nostack", "invalid --baseline-rev %q; expected jj: or git: prefix", val)
						}
						if after == "" {
							return errorx.Newf("nostack", "invalid --baseline-rev %q; missing rev spec after :", val)
						}
						baseline = Some(NewRevSet(VCS(before), after))
					}
					tok, cancel := withTimeout(cancel.Never(), 10*timex.Minute, cmd.Name)
					defer cancel()
					logCtx := logx.NewLogCtx(tok, logger)
					clock := syscaps.MonotonicClock()
					return ws.Benchmark(logCtx, clock, os.Stdout, BenchmarkOptions{iters, config, baseline})
				},
			},
		},
	}

	rootTok := cancel.Never()
	rootCtx := cancel_bridge.Inject(context.Background(), rootTok)
	if err := app.Run(rootCtx, os.Args); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

// resolveProjects maps "all" to the full forked folder list from workspace config,
// or validates that a single project name exists in the config.
func resolveProjects(getWorkspace func() (Workspace, error), project pathx.RelPath) (Workspace, []pathx.RelPath, error) {
	ws, err := getWorkspace()
	if err != nil {
		return Workspace{}, nil, err
	}
	if project.String() == "all" {
		return ws, collections.SortedMapKeysFunc(ws.Config.ForkedFolders, pathx.RelPath.Compare), nil
	}
	if _, ok := ws.Config.ForkedFolders[project]; !ok {
		return Workspace{}, nil, errorx.Newf("nostack", "invalid --project %q, not a forked folder", project)
	}
	return ws, []pathx.RelPath{project}, nil
}

func withTimeout(parent cancel.Token, duration timex.Duration, cmdName string) (cancel.ChildClockToken, func()) {
	tok := cancel.NewClockToken(parent, syscaps.Scheduler(), cancel.OnTimeout(duration))
	return tok, func() {
		tok.Cancel(errorx.Newf("nostack", "%s completed", cmdName))
	}
}
