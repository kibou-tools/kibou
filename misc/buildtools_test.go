// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package misc_test

import (
	"iter"
	"regexp"
	"strings"
	"testing"

	"code.kibou.tools/base/check"
	. "code.kibou.tools/base/check/prelude"
	. "code.kibou.tools/base/core"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/fsx/fsx_walk"
	"code.kibou.tools/base/syscaps"
)

// buildToolsAtLatest matches references to the go-delve/build-tools generators
// pinned with @latest (for example, ".../cmd/gen-starlark-bindings@latest").
var buildToolsAtLatest = regexp.MustCompile(`go-delve/build-tools/cmd/[\w-]+@latest`)

// TestNoBuildToolsAtLatestInDelve guards against reintroducing
// `go run .../go-delve/build-tools/cmd/X@latest` in delve (in tests or
// //go:generate directives).
//
// The @latest form runs in isolated module mode, which ignores the workspace
// and resolves build-tools' own pinned golang.org/x/tools — too old to parse
// the HEAD toolchain's export data. The version-less form resolves through the
// workspace to third_party/build-tools (and golang.org/x/tools to ./tools).
// See issue #221.
func TestNoBuildToolsAtLatestInDelve(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	workingDir := DoMsg(syscaps.WorkingDirectory())(h, "resolving working directory")
	repoRoot, ok := workingDir.Dir().Get()
	h.Assertf(ok, "working directory %s must have a parent", workingDir)
	repoFS := DoMsg(syscaps.FS(repoRoot))(h, "opening repo FS")

	delveRoot := MustParseRelPath("delve")
	entries := DoMsg(fsx_walk.WalkNonDet(repoFS, delveRoot, fsx_walk.WalkOptions{RespectGitIgnore: true}))(h,
		"walking %s", delveRoot)
	assertNoBuildToolsAtLatest(h, repoFS, delveRoot, entries)
}

func assertNoBuildToolsAtLatest(
	h check.Harness, repoFS fsx.FS, parent RelPath, entries iter.Seq[Result[fsx_walk.FSWalkEntry]],
) {
	for entryRes := range entries {
		entry := DoMsg(entryRes.Get())(h, "walking %s", parent)
		rel := parent.JoinComponents(entry.Name().String())
		if entry.IsDir() {
			assertNoBuildToolsAtLatest(h, repoFS, rel, entry.ChildrenNonDet())
			continue
		}
		if !strings.HasSuffix(rel.String(), ".go") {
			continue
		}
		content := DoMsg(repoFS.ReadFile(rel))(h, "reading %s", rel)
		h.Assertf(!buildToolsAtLatest.Match(content),
			"%s references go-delve/build-tools/cmd/...@latest; drop @latest so it resolves "+
				"through the workspace to third_party/build-tools (see issue #221)", rel)
	}
}
