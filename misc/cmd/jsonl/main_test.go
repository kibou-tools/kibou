// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"code.kibou.tools/base/cancel"
	"code.kibou.tools/base/check"
	"code.kibou.tools/base/check/golden"
	"code.kibou.tools/base/cmdx"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/core/result"
	"code.kibou.tools/base/envx"
	"code.kibou.tools/base/logx"
	"code.kibou.tools/base/syscaps"
	"code.kibou.tools/misc/internal/go_test"
)

var snapshots = golden.NewSnapshotDirSet("testdata/snapshots")

func TestMain(m *testing.M) {
	os.Exit(snapshots.Run(m))
}

func TestRenderGoTest(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("SyntheticSchema", testRenderGoTestSyntheticSchema)
	h.Run("PassesThroughDistIssue76024", testRenderGoTestPassesThroughDistIssue76024)
	h.Run("PassesThroughNonJSON", testRenderGoTestPassesThroughNonJSON)
	h.Run("PackageResults", testRenderGoTestPackageResults)
	h.Run("WarnsUnknownEnums", testRenderGoTestWarnsUnknownEnums)
}

func testRenderGoTestSyntheticSchema(h check.Harness) {
	h.Parallel()
	input := strings.Join([]string{
		`{"Action":"start","Package":"example.test/pkg"}`,
		`{"Action":"run","Package":"example.test/pkg","Test":"TestPanic"}`,
		`{"Action":"output","Package":"example.test/pkg","Test":"TestPanic","Output":"=== RUN   TestPanic\n","OutputType":"frame"}`,
		`{"Action":"output","Package":"example.test/pkg","Test":"TestPanic","Output":"panic: boom\n","OutputType":"error"}`,
		`{"Action":"fail","Package":"example.test/pkg","Test":"TestPanic","Elapsed":0.01}`,
		`{"Action":"fail","Package":"example.test/pkg","Elapsed":0.02}`,
		`{"Action":"fail","Package":"example.test/dep","Elapsed":0,"FailedBuild":"example.test/dep"}`,
	}, "\n") + "\n"

	got := renderString(h, input)
	golden.SnapshotAt(snapshots.FS(h, "testdata/snapshots"), pathx.MustParseRelPath("synthetic-schema.txt")).AssertMatch(h, got)
}

// TestGoTestJSONSamples drives the renderer end-to-end against real `go test
// -json` output from fixture packages. Each subtest snapshots the rendered
// (human-readable) output, which is what the tool actually produces. Only
// lifecycle additionally snapshots the normalized JSONL shape, as a single
// representative; snapshotting it per fixture mostly re-tests `go test` and the
// normalizer, not our code, and is brittle across Go versions.
func TestGoTestJSONSamples(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("Lifecycle", testGoTestJSONLifecycleSample)
	h.Run("StdoutStderr", testGoTestJSONStdoutStderrSample)
	h.Run("Panic", testGoTestJSONPanicSample)
	h.Run("Segfault", testGoTestJSONSegfaultSample)
	h.Run("Timeout", testGoTestJSONTimeoutSample)
	h.Run("BuildFail", testGoTestJSONBuildFailSample)
}

func testGoTestJSONLifecycleSample(h check.Harness) {
	h.Parallel()
	normalized := sampleJSONL(h, "lifecycle", "TestLifecycle", true)
	snapshotFS := snapshots.FS(h, "testdata/snapshots")
	golden.SnapshotAt(snapshotFS, pathx.MustParseRelPath("lifecycle.jsonl")).AssertMatch(h, normalized)
	golden.SnapshotAt(snapshotFS, pathx.MustParseRelPath("lifecycle-rendered.txt")).AssertMatch(h, renderString(h, normalized))
	golden.SnapshotAt(snapshotFS, pathx.MustParseRelPath("lifecycle-actions.txt")).AssertMatch(h, actionTrace(h, normalized))
}

func testGoTestJSONStdoutStderrSample(h check.Harness) {
	h.Parallel()
	assertRenderedSnapshot(h, "stdio", "TestStdoutStderr", false, "stdio-rendered.txt")
}

func testGoTestJSONPanicSample(h check.Harness) {
	h.Parallel()
	assertRenderedSnapshot(h, "panic", "TestPanic", true, "panic-rendered.txt")
}

func testGoTestJSONSegfaultSample(h check.Harness) {
	h.Parallel()
	// A segfault drives the same crash-render path as a panic; rather than
	// snapshot a second near-identical goroutine dump, assert only the
	// SIGSEGV-specific detail that sets it apart.
	rendered := renderString(h, sampleJSONL(h, "segfault", "TestSegfault", true))
	h.Assertf(strings.Contains(rendered, "SIGSEGV"), "segfault render should surface the SIGSEGV signal line")
	h.Assertf(strings.Contains(rendered, "FAIL\texample.test/jsonlfixture/segfault"),
		"segfault package should fail")
}

func testGoTestJSONTimeoutSample(h check.Harness) {
	h.Parallel()
	// A timeout's goroutine dump is inherently non-deterministic — it depends on
	// what every goroutine is doing when the alarm fires — so we assert the
	// stable, meaningful behavior instead of snapshotting the dump: the renderer
	// surfaces the timeout, the package fails, and because a timed-out binary
	// emits no per-test result, the failing test's name is lost (only a
	// package-level failure reaches the summary). With the dump no longer
	// snapshotted, the timeout can be tiny again.
	rendered := renderString(h, runSamplePackage(h, "timeout", "TestTimeout", true, "-timeout=10ms"))

	h.Assertf(strings.Contains(rendered, "test timed out"), "render should surface the timeout panic")
	h.Assertf(strings.Contains(rendered, "FAIL\texample.test/jsonlfixture/timeout"),
		"render should show the package failing")

	idx := strings.Index(rendered, "# go test summary")
	h.Assertf(idx >= 0, "render should include a summary")
	summary := rendered[idx:]
	h.Assertf(strings.Contains(summary, "FAIL example.test/jsonlfixture/timeout ("),
		"summary should list the package-level failure")
	h.Assertf(!strings.Contains(summary, "TestTimeout"),
		"a timed-out test produces no per-test result, so its name is absent from the summary")
}

func testGoTestJSONBuildFailSample(h check.Harness) {
	h.Parallel()
	normalized := sampleJSONL(h, "buildfail", "TestBuildFail", true)
	snapshotFS := snapshots.FS(h, "testdata/snapshots")
	golden.SnapshotAt(snapshotFS, pathx.MustParseRelPath("buildfail.jsonl")).AssertMatch(h, normalized)
	// The rendered snapshot must contain the compiler diagnostic (from
	// build-output), not just a "[build failed]" summary line.
	rendered := renderString(h, normalized)
	h.Assertf(strings.Contains(rendered, "undefined: thisSymbolDoesNotExist"),
		"build-output diagnostics should be rendered, not dropped")
	golden.SnapshotAt(snapshotFS, pathx.MustParseRelPath("buildfail-rendered.txt")).AssertMatch(h, rendered)
}

func testRenderGoTestPassesThroughDistIssue76024(h check.Harness) {
	h.Parallel()
	input := readSnapshotText(h, "dist-issue-76024-input.txt")
	snapshotFS := snapshots.FS(h, "testdata/snapshots")
	golden.SnapshotAt(snapshotFS, pathx.MustParseRelPath("dist-issue-76024-input.txt")).AssertMatch(h, input)

	var out, logBuf bytes.Buffer
	logger := logx.NewLogger(&logBuf, logx.ColorSupport_Disable)
	h.NoErrorf(renderGoTest(logger, go_test.NewColorizer(false), strings.NewReader(input), &out, io.Discard),
		"renderGoTest")
	golden.SnapshotAt(snapshotFS, pathx.MustParseRelPath("dist-issue-76024-rendered.txt")).AssertMatch(h, out.String())
	h.Assertf(strings.Contains(logBuf.String(), "passing through non-JSON"), "the raw dist lines should be warned about")

	captured, err := captureStream(h, input)
	h.NoErrorf(err, "captureStream")
	golden.SnapshotAt(snapshotFS, pathx.MustParseRelPath("dist-issue-76024-captured.jsonl")).AssertMatch(h, captured)
}

func testRenderGoTestPassesThroughNonJSON(h check.Harness) {
	h.Parallel()
	input := strings.Join([]string{
		`{"Action":"output","Package":"example.test/pkg","Output":"before\n"}`,
		`not json at all`,
		`{"Action":"fail","Package":"example.test/pkg","Elapsed":0.01}`,
	}, "\n") + "\n"

	var out, logBuf bytes.Buffer
	logger := logx.NewLogger(&logBuf, logx.ColorSupport_Disable)
	h.NoErrorf(renderGoTest(logger, go_test.NewColorizer(false), strings.NewReader(input), &out, io.Discard),
		"renderGoTest should not abort on a non-JSON line")
	h.Assertf(strings.Contains(out.String(), "not json at all"), "non-JSON line should be echoed verbatim")
	h.Assertf(strings.Contains(out.String(), "before"), "valid output before the bad line should still render")
	h.Assertf(strings.Contains(out.String(), "FAIL example.test/pkg"), "summary should still render after the bad line")
	h.Assertf(strings.Contains(logBuf.String(), "passing through non-JSON"), "the bad line should be warned about")
}

func TestCapture(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	h.Run("WrapsNonJSONWithCarriedTime", testCaptureWrapsNonJSONWithCarriedTime)
	h.Run("BuffersLeadingNonJSONUntilFirstEvent", testCaptureBuffersLeadingNonJSONUntilFirstEvent)
	h.Run("ErrorsWithoutTimestampedEvent", testCaptureErrorsWithoutTimestampedEvent)
}

func testCaptureWrapsNonJSONWithCarriedTime(h check.Harness) {
	h.Parallel()
	input := strings.Join([]string{
		`{"Action":"start","Package":"p","Time":"2026-01-02T03:04:05Z"}`,
		`stray non-json line`,
		`{"Action":"fail","Package":"p","Elapsed":0.01,"Time":"2026-01-02T03:04:07Z"}`,
	}, "\n") + "\n"

	captured, err := captureStream(h, input)
	h.NoErrorf(err, "captureStream")

	lines := nonEmptyLines(captured)
	h.Assertf(len(lines) == 3, "capture should have 3 lines, got %d", len(lines))
	// Valid lines pass through byte-for-byte.
	h.Assertf(lines[0] == `{"Action":"start","Package":"p","Time":"2026-01-02T03:04:05Z"}`, "first line verbatim")
	h.Assertf(lines[2] == `{"Action":"fail","Package":"p","Elapsed":0.01,"Time":"2026-01-02T03:04:07Z"}`, "third line verbatim")
	// The stray line is wrapped as an output event carrying the previous event's Time.
	wrapped := decodeJSONObject(h, lines[1])
	h.Assertf(wrapped["Action"] == "output", "wrapped Action is output")
	h.Assertf(wrapped["Time"] == "2026-01-02T03:04:05Z", "wrapped carries the previous event's Time")
	h.Assertf(wrapped["Output"] == "stray non-json line\n", "wrapped preserves the raw line")
	h.Assertf(wrapped[go_test.RawOutputMarker] == true, "wrapped is marked as injected raw output")
}

func testCaptureBuffersLeadingNonJSONUntilFirstEvent(h check.Harness) {
	h.Parallel()
	// build-output is valid JSON but carries no Time, so a stray line among the
	// leading build events has nothing to anchor to until the start event.
	input := strings.Join([]string{
		`{"ImportPath":"p","Action":"build-output","Output":"# p\n"}`,
		`stray before any timestamp`,
		`{"Action":"start","Package":"p","Time":"2026-01-02T03:04:05Z"}`,
	}, "\n") + "\n"

	captured, err := captureStream(h, input)
	h.NoErrorf(err, "captureStream")

	lines := nonEmptyLines(captured)
	h.Assertf(len(lines) == 3, "capture should have 3 lines, got %d", len(lines))
	h.Assertf(strings.Contains(lines[0], `"Action":"build-output"`), "build-output passes through first")
	// The buffered raw line is flushed before the start event, back-filled with
	// the first timestamped event's Time.
	wrapped := decodeJSONObject(h, lines[1])
	h.Assertf(wrapped[go_test.RawOutputMarker] == true, "buffered line is wrapped")
	h.Assertf(wrapped["Time"] == "2026-01-02T03:04:05Z", "buffered line back-filled with the first event's Time")
	h.Assertf(wrapped["Output"] == "stray before any timestamp\n", "buffered raw line preserved")
	h.Assertf(strings.Contains(lines[2], `"Action":"start"`), "start event follows the flushed buffer")
}

func testCaptureErrorsWithoutTimestampedEvent(h check.Harness) {
	h.Parallel()

	// A stray line among Timeless build events, with the stream ending before
	// any timestamped event: nothing can anchor the buffered line.
	noAnchor := strings.Join([]string{
		`{"ImportPath":"p","Action":"build-output","Output":"# p\n"}`,
		`stray with no anchor`,
		`{"ImportPath":"p","Action":"build-fail"}`,
	}, "\n") + "\n"
	_, err := captureStream(h, noAnchor)
	h.Assertf(err != nil, "capture should fail when no timestamped event anchors a buffered line")

	// Not even one JSON line.
	_, err = captureStream(h, "not json at all\nstill not json\n")
	h.Assertf(err != nil, "capture should fail on a stream with no JSON events")
}

func testRenderGoTestPackageResults(h check.Harness) {
	h.Parallel()
	input := strings.Join([]string{
		`{"Action":"pass","Package":"example.test/pkg","Test":"TestPass","Elapsed":0.01}`,
		`{"Action":"pass","Package":"example.test/pkg","Elapsed":0.02}`,
		`{"Action":"skip","Package":"example.test/pkg","Test":"TestSkip","Elapsed":0.03}`,
		`{"Action":"skip","Package":"example.test/no-tests","Elapsed":0}`,
	}, "\n") + "\n"

	rendered := renderString(h, input)
	h.Assertf(strings.Contains(rendered, "(1 passed)"), "test-level pass should be counted")
	h.Assertf(!strings.Contains(rendered, "(2 passed)"), "package-level pass should not be counted")
	h.Assertf(strings.Contains(rendered, "(1 skipped)"), "test-level skip should be counted")
	h.Assertf(!strings.Contains(rendered, "(2 skipped)"), "package-level skip should not be counted")
}

func testRenderGoTestWarnsUnknownEnums(h check.Harness) {
	h.Parallel()
	input := strings.Join([]string{
		`{"Action":"teleport","Package":"example.test/pkg"}`,
		`{"Action":"teleport","Package":"example.test/pkg"}`,
		`{"Action":"teleport","Package":"example.test/pkg"}`,
		`{"Action":"teleport","Package":"example.test/pkg"}`,
		`{"Action":"teleport","Package":"example.test/pkg"}`,
		`{"Action":"output","Package":"example.test/pkg","Output":"x\n","OutputType":"hologram"}`,
		`{"Action":"output","Package":"example.test/pkg","Output":"x\n","OutputType":"hologram"}`,
		`{"Action":"output","Package":"example.test/pkg","Output":"x\n","OutputType":"hologram"}`,
		`{"Action":"output","Package":"example.test/pkg","Output":"x\n","OutputType":"hologram"}`,
		`{"Action":"output","Package":"example.test/pkg","Output":"x\n","OutputType":"hologram"}`,
	}, "\n") + "\n"

	var out, logBuf bytes.Buffer
	logger := logx.NewLogger(&logBuf, logx.ColorSupport_Disable)
	h.NoErrorf(renderGoTest(logger, go_test.NewColorizer(false), strings.NewReader(input), &out, io.Discard),
		"renderGoTest")
	logs := logBuf.String()
	h.Assertf(strings.Count(logs, "unknown go test JSON action") == 3, "should limit unknown action warnings")
	h.Assertf(strings.Count(logs, "unknown go test JSON output type") == 3, "should limit unknown output type warnings")
	h.Assertf(strings.Contains(out.String(), "- Unknown go test JSON actions (logged: 3/5, omitted: 2/5 for brevity)"),
		"human-readable output should summarize unknown action warnings")
	h.Assertf(strings.Contains(out.String(), "- Unknown go test JSON output types (logged: 3/5, omitted: 2/5 for brevity)"),
		"human-readable output should summarize unknown output type warnings")
}

func TestColorModes(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	for _, name := range []string{"auto", "always", "never"} {
		mode, err := parseColorMode(name)
		h.NoErrorf(err, "parseColorMode(%q)", name)
		h.Assertf(string(mode) == name, "parseColorMode(%q) round-trips", name)
	}
	_, err := parseColorMode("rainbow")
	h.Assertf(err != nil, "parseColorMode should reject unknown modes")

	// A failing stream exercises both red (error output and failure lines) and
	// yellow (the passed/skipped counts). Snapshot the colorized render so the
	// exact ANSI is reviewable in a .ansi file.
	input := strings.Join([]string{
		`{"Action":"output","Package":"example.test/pkg","Test":"TestFoo","Output":"    foo_test.go:10: boom\n","OutputType":"error"}`,
		`{"Action":"output","Package":"example.test/pkg","Test":"TestFoo","Output":"--- FAIL: TestFoo (0.00s)\n","OutputType":"frame"}`,
		`{"Action":"fail","Package":"example.test/pkg","Test":"TestFoo","Elapsed":0.01}`,
		`{"Action":"pass","Package":"example.test/pkg","Test":"TestBar","Elapsed":0.02}`,
		`{"Action":"skip","Package":"example.test/pkg","Test":"TestBaz","Elapsed":0.03}`,
		`{"Action":"fail","Package":"example.test/pkg","Elapsed":0.04}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	h.NoErrorf(renderGoTest(logx.NewLogger(io.Discard, logx.ColorSupport_Disable),
		go_test.NewColorizer(true), strings.NewReader(input), &out, io.Discard),
		"renderGoTest")
	golden.SnapshotAt(snapshots.FS(h, "testdata/snapshots"), pathx.MustParseRelPath("color-modes.ansi")).AssertMatch(h, out.String())
}

func TestPathSpecialForms(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	input, err := parseInputPathOrSpecial("plain.jsonl")
	h.NoErrorf(err, "plain input path")
	h.Assertf(input.kind == InputPath_FilePath && input.path == "plain.jsonl", "plain input path should pass through")

	input, err = parseInputPathOrSpecial(":stdin")
	h.NoErrorf(err, "stdin input special path")
	h.Assertf(input.kind == InputPath_Stdin, ":stdin should parse as stdin")

	input, err = parseInputPathOrSpecial("::leading-colon.jsonl")
	h.NoErrorf(err, "escaped input path")
	h.Assertf(input.kind == InputPath_FilePath && input.path == ":leading-colon.jsonl",
		"escaped input path should lose one colon")

	_, err = parseInputPathOrSpecial(":future-special")
	h.Assertf(err != nil, "unknown input special form should be rejected")

	output, err := parseOutputPathOrSpecial("plain.jsonl")
	h.NoErrorf(err, "plain output path")
	h.Assertf(output.kind == OutputPath_FilePath && output.path == "plain.jsonl", "plain output path should pass through")

	output, err = parseOutputPathOrSpecial(":stdout")
	h.NoErrorf(err, "stdout output special path")
	h.Assertf(output.kind == OutputPath_Stdout, ":stdout should parse as stdout")

	output, err = parseOutputPathOrSpecial(":discard")
	h.NoErrorf(err, "discard output special path")
	h.Assertf(output.kind == OutputPath_Discard, ":discard should parse as discard")

	output, err = parseOutputPathOrSpecial("::leading-colon.jsonl")
	h.NoErrorf(err, "escaped output path")
	h.Assertf(output.kind == OutputPath_FilePath && output.path == ":leading-colon.jsonl",
		"escaped output path should lose one colon")

	_, err = parseOutputPathOrSpecial(":future-special")
	h.Assertf(err != nil, "unknown output special form should be rejected")

	stdout := OutputPath{kind: OutputPath_Stdout, path: ""}
	discard := OutputPath{kind: OutputPath_Discard, path: ""}
	h.NoErrorf(validateOutputPaths(stdout, discard), "stdout may be used by one output")
	h.Assertf(validateOutputPaths(stdout, stdout) != nil, "both outputs must not write to stdout")
}

func TestOpenInput(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	stdin, err := parseInputPathOrSpecial(":stdin")
	h.NoErrorf(err, "parse :stdin")
	r, err := openInput(stdin)
	h.NoErrorf(err, "openInput(:stdin)")
	h.Assertf(r != nil, "openInput(:stdin) returns a reader")
	h.NoErrorf(r.Close(), "close openInput(:stdin)")

	fileInput, err := parseInputPathOrSpecial("testdata/snapshots/lifecycle.jsonl")
	h.NoErrorf(err, "parse input file")
	r, err = openInput(fileInput)
	h.NoErrorf(err, "openInput on an existing file")
	h.NoErrorf(r.Close(), "close input file")
	h.Assertf(r != nil, "openInput returns a reader for an existing file")

	missingInput, err := parseInputPathOrSpecial("testdata/snapshots/does-not-exist.jsonl")
	h.NoErrorf(err, "parse missing input file")
	_, err = openInput(missingInput)
	h.Assertf(err != nil, "openInput reports an error for a missing file")
}

func TestOpenOutput(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	newPath, err := parseOutputPathOrSpecial(filepath.Join(h.T().TempDir(), "output.jsonl"))
	h.NoErrorf(err, "parse new output path")
	out, err := openOutput(newPath, "jsonl-output")
	h.NoErrorf(err, "openOutput on a new file")
	h.Assertf(out.writer != nil, "openOutput returns a writer")
	h.NoErrorf(out.Close(result.Status_Success), "close output file")

	existingPath := filepath.Join(h.T().TempDir(), "existing.jsonl")
	h.NoErrorf(os.WriteFile(existingPath, []byte("old\n"), 0o644), "write existing output file")
	existingOutput, err := parseOutputPathOrSpecial(existingPath)
	h.NoErrorf(err, "parse existing output path")
	out, err = openOutput(existingOutput, "jsonl-output")
	h.NoErrorf(err, "openOutput on an existing file")
	_, err = io.WriteString(out.writer, "new\n")
	h.NoErrorf(err, "write temporary output file")
	data, err := os.ReadFile(existingPath)
	h.NoErrorf(err, "read existing output before commit")
	h.Assertf(string(data) == "old\n", "existing output should not be overwritten before commit, got %q", data)
	h.NoErrorf(out.Close(result.Status_Success), "commit output")
	data, err = os.ReadFile(existingPath)
	h.NoErrorf(err, "read existing output after commit")
	h.Assertf(string(data) == "new\n", "existing output should be overwritten at commit, got %q", data)

	// When jsonl's own processing fails partway (Status_Failure, not a test
	// failure in the stream), the temporary is discarded and the prior output
	// is left intact.
	out, err = openOutput(existingOutput, "jsonl-output")
	h.NoErrorf(err, "reopen existing output")
	_, err = io.WriteString(out.writer, "partial")
	h.NoErrorf(err, "write partial output")
	h.NoErrorf(out.Close(result.Status_Failure), "discard output when processing failed")
	data, err = os.ReadFile(existingPath)
	h.NoErrorf(err, "read existing output after failed processing")
	h.Assertf(string(data) == "new\n", "failed processing must not overwrite the existing output, got %q", data)

	discard, err := parseOutputPathOrSpecial(":discard")
	h.NoErrorf(err, "parse discard output")
	out, err = openOutput(discard, "jsonl-output")
	h.NoErrorf(err, "openOutput(:discard)")
	h.Assertf(out.writer == io.Discard, ":discard should use io.Discard")
	h.NoErrorf(out.Close(result.Status_Success), "close discard output")
}

func runSamplePackage(h check.Harness, name string, run string, wantFail bool, extraArgs ...string) string {
	h.T().Helper()
	cwd, err := syscaps.WorkingDirectory()
	h.NoErrorf(err, "determine package directory")
	sampleDir := cwd.Join(pathx.MustParseRelPath("testdata/packages")).JoinComponents(name)

	ctx := logx.NewLogCtx(cancel.Never(), logx.NewLogger(io.Discard, logx.ColorSupport_Disable))
	options := cmdx.RunOptionsDefault().WithCaptureStdout().WithCaptureStderr()
	options.TransformEnv = func(env envx.Env) envx.Env {
		env, _ = env.InsertOrReplace("GOWORK", "off")
		// Fixtures that panic, crash, or hang are gated behind this variable so
		// they only misbehave when this harness drives them, never under an
		// ordinary `go test ./...`.
		env, _ = env.InsertOrReplace("JSONL_FIXTURE", name)
		return env
	}
	argv := append([]string{"go", "test", "-json", "-count=1", "-run", run}, extraArgs...)
	argv = append(argv, ".")
	output, err := (syscaps.CmdRunner{Env: syscaps.Env()}).Run(ctx,
		cmdx.New(argv...).In(sampleDir), options)
	if wantFail {
		h.Assertf(err != nil, "go test sample package %q should fail", name)
	} else {
		h.NoErrorf(err, "go test sample package %q", name)
	}
	h.Assertf(output.Stdout != "", "go test sample package %q should produce JSON output", name)
	return output.Stdout
}

// sampleJSONL runs the named fixture package and returns its normalized go test
// JSONL stream.
func sampleJSONL(h check.Harness, pkg, run string, wantFail bool) string {
	h.T().Helper()
	return goTestNormalizer.NormalizeJSONL(h, runSamplePackage(h, pkg, run, wantFail))
}

// assertRenderedSnapshot runs the fixture and asserts its rendered output
// matches the named snapshot.
func assertRenderedSnapshot(h check.Harness, pkg, run string, wantFail bool, snapshot string) {
	h.T().Helper()
	rendered := renderString(h, sampleJSONL(h, pkg, run, wantFail))
	golden.SnapshotAt(snapshots.FS(h, "testdata/snapshots"), pathx.MustParseRelPath(snapshot)).AssertMatch(h, rendered)
}

func readSnapshotText(h check.Harness, name string) string {
	h.T().Helper()
	data, err := os.ReadFile("testdata/snapshots/" + name)
	h.NoErrorf(err, "read snapshot input %q", name)
	return string(data)
}

func renderString(h check.Harness, input string) string {
	h.T().Helper()
	var out bytes.Buffer
	logger := logx.NewLogger(io.Discard, logx.ColorSupport_Disable)
	h.NoErrorf(renderGoTest(logger, go_test.NewColorizer(false), strings.NewReader(input), &out, io.Discard),
		"renderGoTest")
	return out.String()
}

// captureStream runs the renderer with a capture sink and returns the sanitized
// JSONL it wrote, along with any error from the capture (e.g. an unanchored
// stream). The human-readable render is discarded.
func captureStream(h check.Harness, input string) (string, error) {
	h.T().Helper()
	var render, captured bytes.Buffer
	logger := logx.NewLogger(io.Discard, logx.ColorSupport_Disable)
	err := renderGoTest(logger, go_test.NewColorizer(false), strings.NewReader(input), &render, &captured)
	return captured.String(), err
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func decodeJSONObject(h check.Harness, line string) map[string]any {
	h.T().Helper()
	var obj map[string]any
	h.NoErrorf(json.Unmarshal([]byte(line), &obj), "decode captured line %q", line)
	return obj
}

// goTestNormalizer is the shared, compiled-once instance. Construct one and
// reuse it rather than building a normalizer inside a per-line loop.
var goTestNormalizer = newGoTestJSONNormalizer()

// goTestJSONNormalizer rewrites a go test -json stream into a deterministic
// form for golden snapshots: fixed timestamps, a constant non-zero elapsed
// time, and output text scrubbed of pointer addresses, goroutine ids, absolute
// source paths, and line numbers. Its regexps are compiled once by
// newGoTestJSONNormalizer, never per line.
type goTestJSONNormalizer struct {
	elapsedParen *regexp.Regexp
	elapsedTab   *regexp.Regexp
	timedOut     *regexp.Regexp
	hexAddr      *regexp.Regexp
	goroutineID  *regexp.Regexp
	pathLine     *regexp.Regexp
	goLine       *regexp.Regexp
}

func newGoTestJSONNormalizer() goTestJSONNormalizer {
	return goTestJSONNormalizer{
		elapsedParen: regexp.MustCompile(`\([0-9]+(?:\.[0-9]+)?s\)`),
		elapsedTab:   regexp.MustCompile(`\t[0-9]+(?:\.[0-9]+)?s`),
		timedOut:     regexp.MustCompile(`timed out after [0-9.]+(?:ns|µs|us|ms|s|m|h)+`),
		// The trailing "?" is Go's marker for a traceback argument whose value
		// may be inaccurate; whether it appears depends on the calling
		// convention, so dropping it keeps snapshots portable across GOARCH.
		hexAddr:     regexp.MustCompile(`0x[0-9a-fA-F]+\??`),
		goroutineID: regexp.MustCompile(`goroutine [0-9]+`),
		// pathLine strips the directory from an absolute "<dir>/<file>.go:<n>"
		// reference, keeping just "<file>.go:N"; goLine handles any remaining
		// relative reference such as "_testmain.go:46". The optional ":<col>"
		// collapses compiler diagnostics like "foo.go:3:11: undefined: y".
		pathLine: regexp.MustCompile(`\S*/([\w.@~+-]+\.go):[0-9]+(?::[0-9]+)?`),
		goLine:   regexp.MustCompile(`([\w.@~+-]+\.go):[0-9]+(?::[0-9]+)?`),
	}
}

// NormalizeJSONL rewrites a full go test JSONL stream line by line.
func (n goTestJSONNormalizer) NormalizeJSONL(h check.Harness, input string) string {
	h.T().Helper()
	var out strings.Builder
	lineNumber := 0
	for _, line := range strings.Split(input, "\n") {
		if line == "" {
			continue
		}
		lineNumber++
		var event map[string]any
		h.NoErrorf(json.Unmarshal([]byte(line), &event), "decode go test JSONL line %d", lineNumber)
		if _, ok := event["Time"].(string); ok {
			event["Time"] = fixedTimestamp(lineNumber)
		}
		if _, ok := event["Elapsed"].(float64); ok {
			// Collapse every elapsed value, including an exact 0: whether a
			// sub-millisecond span rounds to 0.00 or to a tiny non-zero value is
			// itself timing-dependent (and differs on a slow or emulated
			// runner), so it must not leak into snapshots.
			event["Elapsed"] = 0.69
		}
		if output, ok := event["Output"].(string); ok {
			event["Output"] = n.normalizeOutput(output)
		}
		encoded, err := json.Marshal(event)
		h.NoErrorf(err, "encode normalized go test JSONL line %d", lineNumber)
		out.Write(encoded)
		out.WriteByte('\n')
	}
	return out.String()
}

func (n goTestJSONNormalizer) normalizeOutput(output string) string {
	output = n.elapsedParen.ReplaceAllString(output, "(Xs)")
	output = n.elapsedTab.ReplaceAllString(output, "\tXs")
	output = n.timedOut.ReplaceAllString(output, "timed out after Xs")
	output = n.hexAddr.ReplaceAllString(output, "0xADDR")
	output = n.goroutineID.ReplaceAllString(output, "goroutine N")
	output = n.pathLine.ReplaceAllString(output, "$1:N")
	output = n.goLine.ReplaceAllString(output, "$1:N")
	return output
}

func fixedTimestamp(index int) string {
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return base.Add(time.Duration(index-1) * time.Second).Format(time.RFC3339)
}

func actionTrace(h check.Harness, input string) string {
	h.T().Helper()
	var out strings.Builder
	for _, line := range strings.Split(input, "\n") {
		if line == "" {
			continue
		}
		var event map[string]any
		h.NoErrorf(json.Unmarshal([]byte(line), &event), "decode normalized go test JSONL")
		action, _ := event["Action"].(string)
		test, _ := event["Test"].(string)
		if test == "" {
			pkg, _ := event["Package"].(string)
			test = pkg
		}
		if action != "output" {
			out.WriteString(action)
			out.WriteByte('\t')
			out.WriteString(test)
			out.WriteByte('\n')
		}
	}
	return out.String()
}
