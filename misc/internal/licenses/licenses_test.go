// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package licenses

import (
	"bytes"
	"io"
	"iter"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/sync/semaphore"

	"code.kibou.tools/base/cancel"
	"code.kibou.tools/base/check"
	. "code.kibou.tools/base/check/prelude"
	. "code.kibou.tools/base/core"
	"code.kibou.tools/base/errorx"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/fsx/fsx_walk"
	"code.kibou.tools/base/syscaps"
	"code.kibou.tools/misc/internal/config"
)

const firstPartyGoLicenseHeader = `// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

`

func TestFirstPartyGoLicenseHeaders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("license header check is sensitive to Git checkout line endings on Windows")
	}

	h := check.New(t)

	workingDir := DoMsg(syscaps.WorkingDirectory())(h, "resolving working directory")
	repoRoot := repoRootFromMiscDir(h, workingDir)
	repoFS := DoMsg(syscaps.FS(repoRoot))(h, "opening repo FS rooted at %s", repoRoot)
	wsConfig := loadWorkspaceConfig(h, repoFS)

	results := visitFirstPartyGoFiles(h, repoFS, wsConfig, func(rel RelPath) Option[string] {
		if ensureLicenseHeader(h, repoFS, rel) {
			return None[string]()
		}
		return Some(rel.String())
	})

	var missing []string
	for _, res := range results {
		if path, ok := res.Get(); ok {
			missing = append(missing, path)
		}
	}
	slices.Sort(missing)

	h.Assertf(len(missing) == 0,
		"the following first-party Go files are missing the license header:\n%s\n"+
			"hint: Run `go test ./misc/internal/licenses -update`",
		strings.Join(missing, "\n"))
}

func repoRootFromMiscDir(h check.Harness, workingDir AbsPath) AbsPath {
	h.T().Helper()

	misc := fsx.NewName("misc")
	for ancestor := range workingDir.Ancestors() {
		if parent, baseName := ancestor.Split(); baseName == misc {
			return parent
		}
	}
	h.Assertf(false, "working directory %s is not within misc/", workingDir)
	panic("unreachable")
}

func loadWorkspaceConfig(h check.Harness, repoFS fsx.FS) config.WorkspaceConfig {
	h.T().Helper()

	path := MustParseRelPath("misc/repo-configuration.json")
	f := DoMsg(fsx.OpenReadOnly(repoFS, path))(h, "opening %s", path)
	defer h.Close(f)

	return DoMsg(config.Load(f))(h, "loading %s", path)
}

func visitFirstPartyGoFiles[T any](
	h check.Harness,
	repoFS fsx.FS,
	wsConfig config.WorkspaceConfig,
	visit func(RelPath) T,
) []T {
	h.T().Helper()

	var wg sync.WaitGroup
	resultCh := make(chan T)
	sem := semaphore.NewWeighted(int64(2 * runtime.GOMAXPROCS(0)))
	enqueueVisit := func(rel RelPath) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.NoErrorf(sem.Acquire(cancel.Never().AsStdlibContext(), 1), "acquire license check semaphore")
			defer sem.Release(1)
			resultCh <- visit(rel)
		}()
	}

	for entryRes := range repoFS.ReadDir(MustParseRelPath(".")) {
		entry := DoMsg(entryRes.Get())(h, "reading repository root %s", repoFS.Root())
		if !entry.IsDir() {
			continue
		}

		name := entry.BaseName()
		if _, isForked := wsConfig.ForkedFolders[MustParseRelPath(name.String())]; isForked {
			continue
		}

		moduleRoot := MustParseRelPath(".").JoinComponents(name.String())
		goModInfo, err := repoFS.Stat(moduleRoot.JoinComponents("go.mod"), fsx.StatOptions{
			FollowFinalSymlink:     false,
			OnErrorTraverseParents: false,
		})
		if err != nil && errorx.GetRootCauseAsValue(err, fsx.ErrNotExist) {
			continue
		}
		h.NoErrorf(err, "checking for go.mod under %s", moduleRoot)
		h.Assertf(goModInfo.Mode().IsRegular(), "%s/go.mod is not a regular file", moduleRoot)

		h.T().Helper()

		entries := DoMsg(fsx_walk.WalkNonDet(repoFS, moduleRoot, fsx_walk.WalkOptions{RespectGitIgnore: true}))(h, "walking %s", moduleRoot)
		visitWalkEntries(h, moduleRoot, entries, enqueueVisit)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []T
	for res := range resultCh {
		results = append(results, res)
	}
	return results
}

func visitWalkEntries(
	h check.Harness,
	parent RelPath,
	entries iter.Seq[Result[fsx_walk.FSWalkEntry]],
	visit func(RelPath),
) {
	h.T().Helper()

	for entryRes := range entries {
		entry := DoMsg(entryRes.Get())(h, "walking %s", parent)
		entryName := entry.Name().String()
		rel := parent.JoinComponents(entryName)
		if entry.IsDir() {
			visitWalkEntries(h, rel, entry.ChildrenNonDet(), visit)
			continue
		}
		if strings.HasSuffix(rel.String(), ".go") {
			visit(rel)
		}
	}
}

// ensureLicenseHeader reports whether rel has the required header, updating it when -update is set.
func ensureLicenseHeader(h check.Harness, repoFS fsx.FS, rel RelPath) bool {
	mode := fsx.OpenRW_ReadOnly
	if check.IsUpdateFlagSet() {
		mode = fsx.OpenRW_ReadWrite
	}
	f := DoMsg(repoFS.OpenFile(rel, fsx.NewOpenOptions(mode).MustBuild()))(h, "open %s", rel)
	defer h.Close(f)

	var want bytes.Buffer
	want.WriteString(firstPartyGoLicenseHeader)

	var got bytes.Buffer
	_, err := io.CopyN(&got, f, int64(want.Len()))
	if err != nil && err != io.EOF {
		h.NoErrorf(err, "read license header prefix from %s", rel)
	}
	if bytes.Equal(got.Bytes(), want.Bytes()) {
		return true
	}
	if !check.IsUpdateFlagSet() {
		return false
	}

	DoMsg(io.Copy(&got, f))(h, "read rest of %s", rel)
	if end, ok := findLicenseHeaderEnd(got.Bytes()).Get(); ok {
		got.Next(end)
	}
	want.Write(got.Bytes())

	DoMsg(f.Seek(0, io.SeekStart))(h, "seek to start of %s", rel)
	h.NoErrorf(f.Truncate(0), "truncate %s", rel)
	_ = DoMsg(want.WriteTo(f))(h, "write updated %s", rel) // WriteTo succeeds => buffer was drained
	return true
}

// findLicenseHeaderEnd returns the end offset of an existing header with an SPDX line.
func findLicenseHeaderEnd(data []byte) Option[int] {
	spdxPrefix := []byte("\n// SPDX-License-Identifier: ")
	start := bytes.Index(data, spdxPrefix)
	if start < 0 {
		return None[int]()
	}
	endRel := bytes.IndexByte(data[start+len(spdxPrefix):], '\n')
	if endRel < 0 {
		return None[int]()
	}
	end := start + len(spdxPrefix) + endRel + 1
	// Include blank line after the header, since firstPartyGoLicenseHeader includes it too
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return Some(end)
}
