// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package codegen

import (
	"context"
	"io"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/typesanitizer/happygo/common/check"
	"github.com/typesanitizer/happygo/common/cmdx"
	"github.com/typesanitizer/happygo/common/logx"
	"github.com/typesanitizer/happygo/common/syscaps"
)

func TestAssertInliningDiagnostics(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	outPath := filepath.Join(h.T().TempDir(), "codegen.test")
	cmd := cmdx.New(
		"go",
		"test",
		"-c",
		"-o", outPath,
		"-gcflags=github.com/typesanitizer/happygo/common/assert=-m=2",
		"-gcflags=github.com/typesanitizer/happygo/common/assert/internal/codegen=-m=2",
		".",
	)
	ctx := logx.NewLogCtx(context.Background(), logx.NewLogger(io.Discard, logx.ColorSupport_Disable))
	output, err := syscaps.CmdRunner{Env: syscaps.Env()}.Run(ctx, cmd, cmdx.RunOptionsDefault().WithCaptureStderr())
	h.NoErrorf(err, "go test -c failed\n%s", output.Stderr)

	inlineFunctions := []string{
		"Always",
		"Precondition",
		"Preconditionf",
		"Invariant",
		"Invariantf",
		"Postcondition",
		"Postconditionf",
	}
	for _, name := range inlineFunctions {
		pattern := regexp.MustCompile(`(?m)^.*codegen\.go:\d+:\d+: inlining call to assert\.` + name + `\b`)
		h.Assertf(pattern.MatchString(output.Stderr), "missing inline diagnostic for assert.%s\n%s", name, output.Stderr)
	}

	panicFunctions := []string{
		"PanicInvariantViolation",
		"PanicUnknownCase",
	}
	for _, name := range panicFunctions {
		inlinePattern := regexp.MustCompile(`inlining call to assert\.` + name + `\b`)
		h.Assertf(!inlinePattern.MatchString(output.Stderr), "assert.%s was unexpectedly inlined\n%s", name, output.Stderr)

		noInlinePattern := regexp.MustCompile(`cannot inline .*assert\.` + name + `.*marked go:noinline`)
		h.Assertf(noInlinePattern.MatchString(output.Stderr), "missing noinline diagnostic for assert.%s\n%s", name, output.Stderr)
	}
}
