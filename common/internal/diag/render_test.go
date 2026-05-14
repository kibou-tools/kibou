// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package diag_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"code.kibou.tools/common/check"
	"code.kibou.tools/common/check/golden"
	. "code.kibou.tools/common/core/option"
	"code.kibou.tools/common/core/pathx"
	"code.kibou.tools/common/errorx"
	"code.kibou.tools/common/internal/diag"
)

var snapshots = golden.NewSnapshotDirSet("testdata")

func TestMain(m *testing.M) {
	os.Exit(snapshots.Run(m))
}

type testCode string

func (c testCode) String() string { return string(c) }
func (c testCode) SeeAlso() Option[string] {
	if c == "SRV002" {
		return None[string]()
	}
	return Some("https://docs.example.test/errors/" + string(c))
}

func TestRenderPretty(t *testing.T) {
	h := check.New(t)
	p := errorx.NewPhrase
	snippet := diag.NewSnippet
	pos := strings.Index
	span := func(outer string, sub string) (int, int) { return pos(outer, sub), len(sub) }
	emph := diag.Emphasize()
	hint := func(msg string) diag.Hint { return diag.NewHint(diag.HintKind_Suggestion, msg) }

	var plainSnapshot strings.Builder
	var ansiSnapshot strings.Builder
	maxWidth80 := diag.RenderPrettyOptionsDefault()
	maxWidth80.MaxWidth = Some[uint8](80)
	buildFile := strings.Join([]string{
		`load("@rules_go//go:def.bzl", "go_binary")`,
		``,
		`go_binary(`,
		`    name = "server",`,
		`    deps = [`,
		`        "//libs:payemnts",`,
		`    ],`,
		`)`,
	}, "\n")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "build target contains a misspelled package", maxWidth80,
		diag.NewReport[testCode](diag.Severity_Error, "Failed to configure build target //server:server").
			WithCode("BUILD001").
			WithSnippet(buildSnippet(h, snippet(buildFile).
				WithLocation(p("server/BUILD.bazel")).
				WithStartLine(1).
				AtRange(span(buildFile, "payemnts")).Attach(p("this target is not defined in //libs"), emph))).
			WithHint(hint("Did you mean '//libs:payments'?")))
	renderCase(h, &plainSnapshot, &ansiSnapshot, "server startup error", maxWidth80,
		diag.NewReport[testCode](diag.Severity_Error, "Server failed to start").
			WithCode("SRV002").
			WithHint(diag.NewHint(diag.HintKind_Context, "The address 127.0.0.1:8080 is already in use by api-dev-server (pid: 41812)")).
			WithHint(hint("Set API_SERVER_PORT or pass --port to choose a different port")))
	customSeeAlso := diag.RenderPrettyOptionsDefault()
	customSeeAlso.SeeAlsoPrefix = "Explain with: "
	renderCase(h, &plainSnapshot, &ansiSnapshot, "custom see-also prefix", customSeeAlso,
		diag.NewReport[testCode](diag.Severity_Error, "Configuration file is invalid").
			WithCode("CFG123"))
	cmdJobs := "--jobs=0"
	renderCase(h, &plainSnapshot, &ansiSnapshot, "command-line argument error", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Invalid value for --jobs").
			WithSnippet(buildSnippet(h, snippet(cmdJobs).
				WithLocation(p("command line")).
				AtRange(span(cmdJobs, "0")).Attach(p("must be at least 1"), emph))).
			WithHint(hint("If you want maximum parallelism, use --jobs=:ncpu")))
	envVar := "BUILDKIT_REMOTE_CACHE=1"
	renderCase(h, &plainSnapshot, &ansiSnapshot, "environment warning", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Warning, "Remote cache is disabled because the auth token is missing").
			WithSnippet(buildSnippet(h, snippet(envVar).
				WithLocation(p("environment")).
				AtRange(span(envVar, "BUILDKIT_REMOTE_CACHE")).Attach(p("requires BUILDKIT_CACHE_TOKEN"), emph))).
			WithHint(hint("Set BUILDKIT_CACHE_TOKEN or unset BUILDKIT_REMOTE_CACHE")))
	tabWidth2 := diag.RenderPrettyOptionsDefault()
	tabWidth2.TabWidth = 2
	tabSrc := "x\tvalue = 1"
	renderCase(h, &plainSnapshot, &ansiSnapshot, "source line with tabs", tabWidth2,
		diag.NewReport[testCode](diag.Severity_Error, "Tab-aligned source").
			WithSnippet(buildSnippet(h, snippet(tabSrc).
				WithLocation(p("tabs.txt")).
				AtRange(span(tabSrc, "value")).Attach(p("aligned after tab"), emph))))
	maxWidth24 := diag.RenderPrettyOptionsDefault()
	maxWidth24.MaxWidth = Some[uint8](24)
	renderCase(h, &plainSnapshot, &ansiSnapshot, "wrapping overflows unbreakable words", maxWidth24,
		diag.NewReport[testCode](diag.Severity_Error, "supercalifragilisticexpialidocious"))
	renderCase(h, &plainSnapshot, &ansiSnapshot, "wrapping preserves internal whitespace", maxWidth24,
		diag.NewReport[testCode](diag.Severity_Error, "alpha  beta gamma"))
	renderCase(h, &plainSnapshot, &ansiSnapshot, "wrapping trims leading whitespace after split", maxWidth24,
		diag.NewReport[testCode](diag.Severity_Error, "alpha      beta gamma"))

	typeMismatchSrc := strings.Join([]string{
		`package config`,
		``,
		`var MaxAttempts int = "five"`,
		``,
		`func init() { _ = MaxAttempts }`,
	}, "\n")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "two annotations on same line", maxWidth80,
		diag.NewReport[testCode](diag.Severity_Error, "Cannot use untyped string constant as int value").
			WithCode("TYP041").
			WithSnippet(buildSnippet(h, snippet(typeMismatchSrc).
				WithLocation(p("config/limits.go")).
				WithStartLine(1).
				AtRange(span(typeMismatchSrc, "int")).Attach(p("declared type"), emph).
				AtRange(span(typeMismatchSrc, `"five"`)).Attach(p("untyped string constant"), emph))))

	retTypeSrc := strings.Join([]string{
		`package timeout`,
		``,
		`func Parse(s string) time.Duration {`,
		`    return s`,
		`}`,
	}, "\n")
	retValueStart := strings.LastIndex(retTypeSrc, "return ") + len("return ")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "annotations on different lines", maxWidth80,
		diag.NewReport[testCode](diag.Severity_Error, "Return value has wrong type").
			WithCode("TYP073").
			WithSnippet(buildSnippet(h, snippet(retTypeSrc).
				WithLocation(p("timeout/parse.go")).
				WithStartLine(1).
				AtRange(span(retTypeSrc, "time.Duration")).Attach(p("declared as time.Duration"), emph).
				AtRange(retValueStart, 1).Attach(p("but a string value is returned"), emph))))

	cmd := "myapp --input=build.log --output=build.log"
	inputStart := pos(cmd, "--input=") + len("--input=")
	outputStart := pos(cmd, "--output=") + len("--output=")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "two annotations on single-line source", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "The --input and --output flags must refer to different paths").
			WithSnippet(buildSnippet(h, snippet(cmd).
				WithLocation(p("command line")).
				AtRange(inputStart, len("build.log")).Attach(p("read from here"), emph).
				AtRange(outputStart, len("build.log")).Attach(p("would be overwritten"), emph))))

	multilineUnlabeledSrc := strings.Join([]string{
		`line one`,
		`line two`,
	}, "\n")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "multiline source without annotations", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Multiline source is attached without a span").
			WithSnippet(buildSnippet(h, snippet(multilineUnlabeledSrc).
				WithLocation(p("input.txt")).
				WithStartLine(41))))

	multiSpanSrc := strings.Join([]string{
		`single := "before"`,
		`first line`,
		`second line`,
		`single := "after"`,
	}, "\n")
	multiSpanStart := strings.Index(multiSpanSrc, "first")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "multi-line span renders as a separate block", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Multiline span has surrounding single-line labels").
			WithSnippet(buildSnippet(h, snippet(multiSpanSrc).
				WithLocation(p("blocks.txt")).
				WithStartLine(50).
				AtRange(span(multiSpanSrc, `"before"`)).Attach(p("single-line label before"), emph).
				AtRange(multiSpanStart, len("first line\nsecond")).Attach(p("multi-line label"), emph).
				AtRange(span(multiSpanSrc, `"after"`)).Attach(p("single-line label after"), emph))))

	sameLineBeforeSrc := strings.Join([]string{
		`before middle`,
		`finish`,
	}, "\n")
	sameLineBeforeMultiStart := pos(sameLineBeforeSrc, "middle")
	sameLineBeforeMultiLen := pos(sameLineBeforeSrc, "finish") + len("finish") - sameLineBeforeMultiStart
	renderCase(h, &plainSnapshot, &ansiSnapshot, "single-line label before multi-line span on same line", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Single-line label starts before a multiline span on the same line").
			WithSnippet(buildSnippet(h, snippet(sameLineBeforeSrc).
				WithLocation(p("same-line-before.txt")).
				WithStartLine(60).
				AtRange(span(sameLineBeforeSrc, "before")).Attach(p("before the multiline span"), emph).
				AtRange(sameLineBeforeMultiStart, sameLineBeforeMultiLen).Attach(p("multi-line label"), emph))))

	sameLineAfterSrc := strings.Join([]string{
		`begin`,
		`middle after`,
	}, "\n")
	sameLineAfterMultiLen := pos(sameLineAfterSrc, " after")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "single-line label after multi-line span on same line", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Single-line label starts after a multiline span on the same line").
			WithSnippet(buildSnippet(h, snippet(sameLineAfterSrc).
				WithLocation(p("same-line-after.txt")).
				WithStartLine(70).
				AtRange(0, sameLineAfterMultiLen).Attach(p("multi-line label"), emph).
				AtRange(span(sameLineAfterSrc, "after")).Attach(p("after the multiline span"), emph))))

	insideMultiSrc := strings.Join([]string{
		`begin and inside`,
		`finish`,
	}, "\n")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "single-line label inside multi-line span renders as a separate block", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Single-line label is contained in a multiline span").
			WithSnippet(buildSnippet(h, snippet(insideMultiSrc).
				WithLocation(p("inside-multiline.txt")).
				WithStartLine(80).
				AtRange(0, len(insideMultiSrc)).Attach(p("multi-line label"), emph).
				AtRange(span(insideMultiSrc, "inside")).Attach(p("inside the multiline span"), emph))))

	bracketSrc := "filter(items[ where x > 0)"
	renderCase(h, &plainSnapshot, &ansiSnapshot, "mixed labeled and unlabeled annotations", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Mismatched delimiters").
			WithSnippet(buildSnippet(h, snippet(bracketSrc).
				WithLocation(p("query.txt")).
				AtRange(span(bracketSrc, "[")).Attach(p("opening bracket"), emph).
				AtRange(span(bracketSrc, ")")).Attach(p("expected ']' to close the bracket above"), emph))))

	emojiSrc := `status: "👩‍💻 ready"`
	renderCase(h, &plainSnapshot, &ansiSnapshot, "annotations around emoji grapheme clusters", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Status text uses unsupported glyphs").
			WithSnippet(buildSnippet(h, snippet(emojiSrc).
				WithLocation(p("status.txt")).
				AtRange(span(emojiSrc, "👩‍💻")).Attach(p("emoji is one grapheme cluster"), emph).
				AtRange(span(emojiSrc, "ready")).Attach(p("text after emoji stays aligned"), emph))))

	oldConfig := `timeout_ms = 1000`
	newConfig := `timeout_ms = "fast"`
	renderCase(h, &plainSnapshot, &ansiSnapshot, "annotations across multiple snippets", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Configuration value changed type").
			WithSnippet(buildSnippet(h, snippet(oldConfig).
				WithLocation(p("config.old")).
				AtRange(span(oldConfig, "1000")).Attach(p("previously an integer"), emph))).
			WithSnippet(buildSnippet(h, snippet(newConfig).
				WithLocation(p("config.new")).
				AtRange(span(newConfig, `"fast"`)).Attach(p("now a string"), emph))))

	overlapSrc := `sum := total(items.length)`
	renderCase(h, &plainSnapshot, &ansiSnapshot, "overlapping annotations on same line", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Type errors in argument expression").
			WithSnippet(buildSnippet(h, snippet(overlapSrc).
				WithLocation(p("report.go")).
				AtRange(span(overlapSrc, "total(items.length)")).Attach(p("total expects []int, got int"), emph).
				AtRange(span(overlapSrc, "items.length")).Attach(p("slices have no .length field; use len(items)"), emph))))

	missingSemiSrc := `x := foo()`
	renderCase(h, &plainSnapshot, &ansiSnapshot, "point annotation at end of source", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Missing semicolon").
			WithSnippet(buildSnippet(h, snippet(missingSemiSrc).
				WithLocation(p("expr.go")).
				AtPos(len(missingSemiSrc)).Attach(p("insert ';' here"), emph))))

	multilineLFSrc := strings.Join([]string{
		`alpha := 1`,
		`beta := 2`,
	}, "\n")
	lfBreak := strings.Index(multilineLFSrc, "\n")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "point annotation on LF line break", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Missing semicolon before next statement").
			WithSnippet(buildSnippet(h, snippet(multilineLFSrc).
				WithLocation(p("lf.go")).
				WithStartLine(10).
				AtPos(lfBreak).Attach(p("insert ';' before newline"), emph).
				AtRange(span(multilineLFSrc, "beta")).Attach(p("next statement starts here"), emph))))

	multilineCRLFSrc := strings.Join([]string{
		`alpha := 1`,
		`beta := 2`,
	}, "\r\n")
	crlfBreak := strings.Index(multilineCRLFSrc, "\r\n")
	renderCase(h, &plainSnapshot, &ansiSnapshot, "point annotation on CRLF line break", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "Missing semicolon before next statement").
			WithSnippet(buildSnippet(h, snippet(multilineCRLFSrc).
				WithLocation(p("crlf.go")).
				WithStartLine(20).
				AtPos(crlfBreak).Attach(p("insert ';' before CRLF"), emph).
				AtRange(span(multilineCRLFSrc, "beta")).Attach(p("next statement starts here"), emph))))

	renderANSIStyleSmoke(h, &ansiSnapshot)

	fs := snapshots.FS(h, "testdata")
	golden.SnapshotAt(h, fs, pathx.NewRelPath("rendered_diagnostics.txt")).Matches(plainSnapshot.String())
	golden.SnapshotAt(h, fs, pathx.NewRelPath("rendered_diagnostics.ansi")).Matches(ansiSnapshot.String())
}

func TestRenderPrettyBuildErrors(t *testing.T) {
	h := check.New(t)
	p := errorx.NewPhrase
	snippet := diag.NewSnippet
	emph := diag.Emphasize()

	var snapshot strings.Builder

	renderSnippetErrorCase(h, &snapshot, "invalid UTF-8 snippet text",
		snippet(string([]byte{'a', 0xff, 0xab, 0xfe, 'b'})))

	utf8StartSrc := "xéy"
	renderSnippetErrorCase(h, &snapshot, "label span starts inside a UTF-8 codepoint",
		snippet(utf8StartSrc).
			AtRange(strings.Index(utf8StartSrc, "é")+1, 1).Attach(p("starts inside é"), emph))

	utf8EndSrc := "xéy"
	renderSnippetErrorCase(h, &snapshot, "label span ends inside a UTF-8 codepoint",
		snippet(utf8EndSrc).
			AtRange(strings.Index(utf8EndSrc, "é"), 1).Attach(p("ends inside é"), emph))

	graphemeStartSrc := "x👩‍💻y"
	renderSnippetErrorCase(h, &snapshot, "label span starts inside a grapheme cluster",
		snippet(graphemeStartSrc).
			AtRange(strings.Index(graphemeStartSrc, "💻"), len("💻")).Attach(p("starts inside the emoji cluster"), emph))

	graphemeEndSrc := "x👩‍💻y"
	renderSnippetErrorCase(h, &snapshot, "label span ends inside a grapheme cluster",
		snippet(graphemeEndSrc).
			AtRange(strings.Index(graphemeEndSrc, "👩"), len("👩")).Attach(p("ends inside the emoji cluster"), emph))

	elisionSrc := "abcdefghijéklmnopqrst"
	renderSnippetErrorCase(h, &snapshot, "byte visualization elides distant context",
		snippet(elisionSrc).
			AtRange(strings.Index(elisionSrc, "é")+1, 1).Attach(p("starts inside é"), emph))

	fs := snapshots.FS(h, "testdata")
	golden.SnapshotAt(h, fs, pathx.NewRelPath("rendered_diagnostics_build_errors.txt")).Matches(snapshot.String())
}

func TestRenderPrettyFIXME(t *testing.T) {
	h := check.New(t)

	var snapshot strings.Builder
	renderFIXMECase(h, &snapshot, "FIXME: embedded newlines in messages and hints break framing", diag.RenderPrettyOptionsDefault(),
		diag.NewReport[testCode](diag.Severity_Error, "first message line\nsecond message line").
			WithHint(diag.NewHint(diag.HintKind_Suggestion, "first hint line\nsecond hint line")))

	fs := snapshots.FS(h, "testdata")
	golden.SnapshotAt(h, fs, pathx.NewRelPath("rendered_diagnostics_fixme.txt")).Matches(snapshot.String())
}

func buildSnippet(h check.Harness, b *diag.SnippetBuilder) diag.Snippet {
	h.T().Helper()
	partial := b.Build()
	if buildErr := partial.Err(); buildErr != nil {
		h.NoErrorf(buildErr, "building snippet")
	}
	snippet, ok := partial.Value().Get()
	h.Assertf(ok, "expected snippet build to return a value")
	return snippet
}

func renderCase(h check.Harness, plain, _ *strings.Builder, name string, opts diag.RenderPrettyOptions, d *diag.Report[testCode]) {
	h.T().Helper()
	heading := name
	if n, ok := opts.MaxWidth.Get(); ok {
		heading = fmt.Sprintf("%s (MaxWidth: %d)", name, n)
	}
	fmt.Fprintf(plain, "## %s\n\n", heading)
	h.NoErrorf(diag.RenderPretty(plain, d, diag.PlainStyle, opts), "rendering diagnostic")
	plain.WriteByte('\n')
}

func renderSnippetErrorCase(h check.Harness, out *strings.Builder, name string, b *diag.SnippetBuilder) {
	h.T().Helper()
	fmt.Fprintf(out, "## %s\n\n", name)
	partial := b.Build()
	buildErr := partial.Err()
	if buildErr == nil {
		h.Assertf(false, "snippet build succeeded unexpectedly for %q", name)
	}
	h.NoErrorf(diag.RenderPretty[diag.Code](out, buildErr, diag.PlainStyle, diag.RenderPrettyOptionsDefault()), "rendering snippet build error")
	out.WriteByte('\n')
}

func renderFIXMECase(h check.Harness, out *strings.Builder, name string, opts diag.RenderPrettyOptions, d *diag.Report[testCode]) {
	h.T().Helper()
	heading := name
	if n, ok := opts.MaxWidth.Get(); ok {
		heading = fmt.Sprintf("%s (MaxWidth: %d)", name, n)
	}
	fmt.Fprintf(out, "## %s\n\n", heading)
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(out, "[panic: %v]\n\n", r)
		}
	}()
	if err := diag.RenderPretty(out, d, diag.PlainStyle, opts); err != nil {
		fmt.Fprintf(out, "[render error: %v]\n", err)
	}
	out.WriteByte('\n')
}

func renderANSIStyleSmoke(h check.Harness, ansi *strings.Builder) {
	h.T().Helper()
	p := errorx.NewPhrase
	fmt.Fprintf(ansi, "## ansi styling\n\n")
	h.NoErrorf(diag.RenderPretty(ansi,
		diag.NewReport[testCode](diag.Severity_Error, "ANSI styled error").
			WithCode("ANSI001").
			WithHint(diag.NewHint(diag.HintKind_Context, "context text")).
			WithHint(diag.NewHint(diag.HintKind_Suggestion, "hint text")),
		diag.ANSIStyle,
		diag.RenderPrettyOptionsDefault()),
		"rendering ANSI error diagnostic")
	ansi.WriteByte('\n')
	h.NoErrorf(diag.RenderPretty(ansi,
		diag.NewReport[testCode](diag.Severity_Warning, "ANSI styled warning").
			WithSnippet(buildSnippet(h, diag.NewSnippet("warn := true").
				WithLocation(p("ansi.go")).
				AtRange(0, len("warn")).Attach(p("warning span"), diag.Emphasize()))),
		diag.ANSIStyle,
		diag.RenderPrettyOptionsDefault()),
		"rendering ANSI warning diagnostic")
	ansi.WriteByte('\n')
}
