// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main_test

import (
	"fmt"
	"internal/diff"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestKibouExperiments_BuildTags(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		Name         string
		Args         []string // go subcommand + args; defaults to {"run", "."}
		EnvAdditions []string
		Want         CmdOutput
		WantErr      bool
	}

	goExe := "go"
	if runtime.GOOS == "windows" {
		goExe += ".exe"
	}

	testCases := []TestCase{
		{
			Name:         "enums_enabled",
			EnvAdditions: []string{"KIBOU_EXPERIMENTS=Enums"},
			Want:         CmdOutput{Stdout: "enums enabled\n"},
		},
		{
			Name:         "enums_disabled",
			EnvAdditions: []string{"KIBOU_EXPERIMENTS=NoEnums"},
			Want:         CmdOutput{Stdout: "enums disabled\n"},
		},
		{
			// KIBOU_EXPERIMENTS=none disables every experiment; since Enums
			// defaults to off, the program still selects the disabled file.
			Name:         "none_disables_all",
			EnvAdditions: []string{"KIBOU_EXPERIMENTS=none"},
			Want:         CmdOutput{Stdout: "enums disabled\n"},
		},
		{
			// With nothing set, we fall back to the baseline (Enums off).
			Name: "default_when_unset",
			Want: CmdOutput{Stdout: "enums disabled\n"},
		},
		{
			Name:         "unsupported_value_is_rejected",
			EnvAdditions: []string{"KIBOU_EXPERIMENTS=Bogus"},
			WantErr:      true,
			// TODO: When the invocation is 'kibou build', this should
			// really be 'kibou:' (or something like 'driver:'), not 'go:'.
			Want: CmdOutput{
				Stderr: fmt.Sprintf(`%v: unsupported value for KIBOU_EXPERIMENTS Bogus
Supported value(s): Enums
`, goExe),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			root := prepareInputs(t, enumProbeModule())
			args := tc.Args
			if len(args) == 0 {
				args = []string{"run", "."}
			}
			cmd := NewRunner(t, root).NewCommand("go", args...)
			cmd.Env = append(cmd.Env, tc.EnvAdditions...)

			out, err := cmd.Run()
			checkResult(t, tc.Want, out, err, tc.WantErr)
		})
	}
}

func TestKibouExperiments_GoEnv(t *testing.T) {
	t.Parallel()

	root := prepareInputs(t, enumProbeModule())
	runner := NewRunner(t, root)

	goEnv := func(extraEnv []string, args ...string) (CmdOutput, error) {
		cmd := runner.NewCommand("go", append([]string{"env"}, args...)...)
		cmd.Env = append(cmd.Env, extraEnv...)
		return cmd.Run()
	}

	// A read reflects the value from the process environment.
	out, err := goEnv([]string{"KIBOU_EXPERIMENTS=Enums"}, "KIBOU_EXPERIMENTS")
	NoErr(t, err)
	checkOutputs(t, CmdOutput{Stdout: "Enums\n"}, out)

	// go env -w validates its input and rejects unsupported values.
	out, err = goEnv(nil, "-w", "KIBOU_EXPERIMENTS=Bogus")
	if err == nil {
		t.Fatalf("go env -w with an unsupported value should fail; stderr:\n%s", out.Stderr)
	}
	checkOutputs(t, CmdOutput{
		Stderr: `go: unsupported value for KIBOU_EXPERIMENTS Bogus
Supported value(s): Enums
`,
	}, out)

	// A valid go env -w persists, and a later read (with nothing in the
	// process environment) picks it up from the env file.
	_, err = goEnv(nil, "-w", "KIBOU_EXPERIMENTS=Enums")
	NoErr(t, err)
	out, err = goEnv(nil, "KIBOU_EXPERIMENTS")
	NoErr(t, err)
	checkOutputs(t, CmdOutput{Stdout: "Enums\n"}, out)
}

// Runner owns the immutable base environment and sandbox for a single test,
// and mints commands that re-exec the cmd/go test binary as `go`.
type Runner struct {
	root    TempRoot
	T       testing.TB
	BaseEnv []string
}

// TempRoot is a sandbox directory together with an [os.Root] rooted at it.
type TempRoot struct {
	TmpDir string
	*os.Root
}

func NewRunner(tb testing.TB, root TempRoot) *Runner {
	baseEnv := []string{
		"CMDGO_TEST_RUN_MAIN=true",
		"GO111MODULE=on",    // Run in module-aware mode
		"GOTOOLCHAIN=local", // Avoid downloading toolchain from itnernet
		"GOWORK=off",        // Avoid accidentally use outer workspace file
		// Keep the env file inside the sandbox so it starts empty and
		// `go env -w` has somewhere writable to persist to.
		fmt.Sprintf("GOENV=%v/go-env", root.TmpDir),
		fmt.Sprintf("GOCACHE=%v/gocache", root.TmpDir),
		fmt.Sprintf("GOMODCACHE=%v/gomodcache", root.TmpDir),
		homeEnvName() + "=/no-home",
		tempEnvName() + "=" + filepath.Join(root.TmpDir, "tmp"),
		"GOPROXY=off",         // No network access
		"GOSUMDB=off",         // Same as above
		"TESTGONETWORK=panic", // Disable network connections
		"CGO_ENABLED=0",       // Tests will default to pure Go
	}
	// See NOTE(id: script-preserve-env-vars); limit to what I think we'll
	// need for now.
	for _, k := range []string{
		"GOCOVERDIR", // If we want to optionally record coverage information from these tests
		"SYSTEMROOT",
		"WINDIR",
		"ComSpec",
		"DYLD_LIBRARY_PATH",
	} {
		if v, ok := os.LookupEnv(k); ok {
			baseEnv = append(baseEnv, k+"="+v)
		}
	}

	// The go command requires $TMPDIR to exist; create the sandbox tmp dir.
	NoErr(tb, root.MkdirAll("tmp", 0777))

	return &Runner{root, tb, baseEnv}
}

func (r *Runner) NewCommand(name string, args ...string) *Cmd {
	return &Cmd{Argv: append([]string{name}, args...), Env: slices.Clone(r.BaseEnv), WorkingDirectory: r.root.TmpDir, runner: r}
}

// Cmd is a single command invocation. Modify Env before calling Run; the
// Runner's BaseEnv is never mutated.
type Cmd struct {
	Argv             []string
	Env              []string
	WorkingDirectory string
	runner           *Runner
}

// CmdOutput holds the separately-captured streams of a finished command.
type CmdOutput struct {
	Stdout string
	Stderr string
}

func (c *Cmd) Run() (CmdOutput, error) {
	if c.Argv[0] != "go" {
		c.runner.T.Fatalf("argv[0] must be 'go', got %q", c.Argv[0])
	}
	cmd := exec.Command(testGo, c.Argv[1:]...) // testGo is the cmd/go test global
	cmd.Dir = c.WorkingDirectory
	cmd.Env = c.Env
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return CmdOutput{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

// Module is the source module written into a sandbox by prepareInputs.
type Module struct {
	Name  string
	Files []File
}

type File struct {
	Path     string
	Contents string
}

// enumProbeModule returns a module that builds under any KIBOU_EXPERIMENTS
// setting: exactly one of the two mutually-exclusive build-tagged files
// defines main, so the stdout reports whether Enums is enabled.
func enumProbeModule() Module {
	return Module{
		Name: "enumprobe",
		Files: []File{
			{Path: "enabled.go", Contents: buildTagged("kibou_expt.Enums", "enums enabled")},
			{Path: "disabled.go", Contents: buildTagged("!kibou_expt.Enums", "enums disabled")},
		},
	}
}

func buildTagged(tag, msg string) string {
	return fmt.Sprintf(`//go:build %s

package main

import "fmt"

func main() {
	fmt.Println(%q)
}
`, tag, msg)
}

func prepareInputs(t testing.TB, mod Module) TempRoot {
	tmpDir := t.TempDir()
	root := NoErr2(os.OpenRoot(tmpDir))(t)
	// Make sure to close the directory so that deletion on Windows works.
	t.Cleanup(func() { _ = root.Close() })

	NoErr(t, root.WriteFile("go.mod", fmt.Appendf(nil, "module %s\n", mod.Name), 0644))
	GtEq(t, len(mod.Files), 1)
	for _, f := range mod.Files {
		Assert(t, filepath.IsLocal(f.Path))
		if dir := filepath.Dir(f.Path); dir != "." {
			NoErr(t, root.MkdirAll(dir, 0644))
		}
		NoErr(t, root.WriteFile(f.Path, []byte(f.Contents), 0644))
	}

	return TempRoot{tmpDir, root}
}

// checkResult verifies the command's exit status matches wantErr and its
// captured output matches want.
func checkResult(t *testing.T, want CmdOutput, got CmdOutput, err error, wantErr bool) {
	t.Helper()
	switch {
	case wantErr && err == nil:
		t.Errorf("expected command to fail, but it succeeded\nstdout:\n%s\nstderr:\n%s", got.Stdout, got.Stderr)
	case !wantErr && err != nil:
		t.Errorf("command failed unexpectedly: %v\nstderr:\n%s", err, got.Stderr)
	}
	checkOutputs(t, want, got)
}

func checkOutputs(t *testing.T, expected CmdOutput, actual CmdOutput) {
	t.Helper()

	if d := diff.Diff("want", []byte(expected.Stdout), "got", []byte(actual.Stdout)); d != nil {
		t.Errorf("stdout differs:\n%s", d)
	}
	if d := diff.Diff("want", []byte(expected.Stderr), "got", []byte(actual.Stderr)); d != nil {
		t.Errorf("stderr differs:\n%s", d)
	}
}

func NoErr(tb testing.TB, err error) {
	tb.Helper()
	if err != nil {
		tb.Fatal(err)
	}
}

func NoErr2[T any](t T, err error) func(tb testing.TB) T {
	return func(tb testing.TB) T {
		tb.Helper()
		if err != nil {
			tb.Fatal(err)
		}
		return t
	}
}

func GtEq[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64](tb testing.TB, got T, min T) {
	tb.Helper()
	if got >= min {
		return
	}
	tb.Fatalf("got %v, want >= %v", got, min)
}

func Assert(tb testing.TB, b bool) {
	tb.Helper()
	if !b {
		tb.Fatal()
	}
}
