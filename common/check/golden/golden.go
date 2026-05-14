// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package golden provides package-level golden-file snapshot management.
package golden

import (
	"flag"
	"fmt"
	"iter"
	"os"
	"strings"
	"sync"
	"testing"

	"code.kibou.tools/common/assert"
	"code.kibou.tools/common/check"
	"code.kibou.tools/common/collections"
	"code.kibou.tools/common/core/op"
	"code.kibou.tools/common/core/option"
	"code.kibou.tools/common/core/pathx"
	"code.kibou.tools/common/core/result"
	"code.kibou.tools/common/errorx"
	"code.kibou.tools/common/fsx"
	"code.kibou.tools/common/logx"
	"code.kibou.tools/common/syscaps" //nolint:depguard // golden is only used from TestMain, which is a program edge
)

// SnapshotDirSet coordinates package-level snapshot directories.
type SnapshotDirSet struct {
	// List of subdirectories which are tracked for snapshots.
	subdirs collections.MonotoneMap[pathx.RelPath, *snapshotDir]

	// fs is rooted at the source directory of the package to be tested.
	fs fsx.FS

	// Dedicated logger because testing.M usage doesn't grant access
	// to a test-specific logging API. :-/
	logger logx.Logger
}

// NewSnapshotDirSet creates a snapshot directory manager for use from TestMain.
//
// Pre-conditions:
//  1. Each dir in subdirs is a non-empty relative path.
//  2. roots must not contain duplicates after lexical normalization.
//
// NewSnapshotDirSet is meant to be used for initializing a global variable
// and the Run method should be called inside TestMain.
//
// The working directory must be set before this function is invoked.
//
// This function is only meant to be called in a testing context.
func NewSnapshotDirSet(subdirs ...string) *SnapshotDirSet {
	assert.Precondition(testing.Testing(), "NewSnapshotDirSet is only meant to be used in tests")

	cwd, err := syscaps.WorkingDirectory()
	assert.Preconditionf(err == nil, "failed to get current working directory: %v", err)

	// When global variables in a test are initialized, the working directory
	// is set to the source directory of the package, so rooting fs at cwd
	// is justified.
	fs, err := syscaps.FS(cwd)
	assert.Preconditionf(err == nil, "failed to create filesystem rooted at %s: %v", cwd, err)

	set := &SnapshotDirSet{
		subdirs: collections.NewMonotoneMap[pathx.RelPath, *snapshotDir](),
		fs:      fs,
		logger:  logx.NewLogger(os.Stderr, logx.ColorSupport_AutoDetect),
	}
	for _, subdir := range subdirs {
		rootRel := pathx.NewRelPath(subdir)
		dir := &snapshotDir{
			pkgRelPath: rootRel,
			mu:         sync.Mutex{},
			usedFiles:  collections.NewSet[pathx.RelPath](),
		}
		res := set.subdirs.InsertOrKeep(rootRel, dir)
		assert.Preconditionf(res == op.InsertedNew, "duplicate snapshot directory %q", rootRel)
	}
	return set
}

// Run runs tests and prunes stale snapshots after a successful -update run.
// It is intended for package TestMain functions:
//
//	func TestMain(m *testing.M) { os.Exit(snapshots.Run(m)) }
func (dirSet *SnapshotDirSet) Run(m *testing.M) int {
	code := m.Run()
	// Even a test is run multiple times using -count=N,
	// m.Run() will handle running the test N times, so the below
	// code can be assumed to be single-threaded.

	// Any flag checking should happen after m.Run() finishes,
	// because the flags get registered during m.Run().
	if code != 0 || !check.IsUpdateFlagSet() {
		return code
	}
	if shouldPrune, foundFlag := shouldPruneSnapshots(); !shouldPrune {
		dirSet.logger.Info("snapshot pruning skipped based on 'go test' flag", "flag", foundFlag)
		return code
	}
	if err := dirSet.pruneUnusedSnapshots(); err != nil {
		dirSet.logger.Info("snapshot pruning failed", "err", err)
		return 1
	}
	return code
}

// FS returns the filesystem rooted at the registered snapshot directory.
//
// Pre-conditions:
//  1. root is a non-empty relative path registered with [NewSnapshotDirSet].
//  2. set.Run has initialized the snapshot directory set.
func (dirSet *SnapshotDirSet) FS(h check.Harness, root string) SnapshotFS {
	h.T().Helper()
	rootRel := pathx.NewRelPath(root)
	dir, ok := dirSet.subdirs.Lookup(rootRel).Get()
	h.Assertf(ok, "unknown snapshot directory %q", root)

	if check.IsUpdateFlagSet() {
		h.NoErrorf(dirSet.fs.MkdirAll(dir.pkgRelPath, 0o755), "creating snapshot directory %s", dir.pkgRelPath)
	}
	return dirSet.snapshotFSFor(dir)
}

func (dirSet *SnapshotDirSet) snapshotFSFor(dir *snapshotDir) snapshotFS {
	return snapshotFS{dir: dir, impl: dirSet.fs, pkgRelPath: dir.pkgRelPath}
}

func (dirSet *SnapshotDirSet) pruneUnusedSnapshots() error {
	for key := range dirSet.subdirs.Keys() {
		dir := dirSet.subdirs.Lookup(key).Expect("snapshot directory key should have a value")
		if err := dirSet.pruneSnapshotDir(dirSet.snapshotFSFor(dir), pathx.Dot()); err != nil {
			return errorx.Wrapf("+stacks", err, "snapshot directory %q", dir.pkgRelPath)
		}
	}
	return nil
}

type fileCount uint8

const (
	fileCount_0 fileCount = iota
	fileCount_1orMore
)

// pruneSnapshotDir prunes unused files under dir using a depth-first traversal.
// Empty descendant directories are removed; dir itself is left in place. Symlinks
// are rejected rather than followed.
func (dirSet *SnapshotDirSet) pruneSnapshotDir(fs snapshotFS, dir pathx.RelPath) error {
	type Phase uint8
	const (
		Enter Phase = iota
		Exit
	)
	type Frame struct {
		dir    pathx.RelPath
		parent option.Option[pathx.RelPath]
		phase  Phase
	}

	liveCounts := map[pathx.RelPath]fileCount{dir: fileCount_0}
	stack := collections.NewStack[Frame]()
	stack.Push(Frame{dir: dir, parent: option.None[pathx.RelPath](), phase: Enter})

	for !stack.IsEmpty() {
		frame := stack.Pop()
		if frame.phase == Exit {
			count := liveCounts[frame.dir]
			parent, ok := frame.parent.Get()
			if !ok {
				return nil
			}
			if count == fileCount_0 {
				if err := fs.RemoveAll(frame.dir); err != nil {
					return err
				}
				continue
			}
			liveCounts[parent] = fileCount_1orMore
			continue
		}

		liveCounts[frame.dir] = fileCount_0
		stack.Push(Frame{
			dir:    frame.dir,
			parent: frame.parent,
			phase:  Exit,
		})
		for entryRes := range fs.ReadDir(frame.dir) {
			entry, err := entryRes.Get()
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			path := frame.dir.JoinComponents(entry.BaseName().String())
			if info.Mode()&os.ModeSymlink != 0 {
				return errorx.Newf("nostack",
					"snapshot path %s is a symlink; symlink snapshots are not supported",
					fs.Root().Join(path))
			}
			if entry.IsDir() {
				stack.Push(Frame{dir: path, parent: option.Some(frame.dir), phase: Enter})
				continue
			}
			if fs.dir.usedFiles.Contains(path) {
				liveCounts[frame.dir] = fileCount_1orMore
				continue
			}
			if err := fs.RemoveAll(path); err != nil {
				return err
			}
			dirSet.logger.Info("deleted stale snapshot", "path", fs.Root().Join(path))
		}
	}
	return nil
}

// Snapshot holds a path for file-based snapshot comparison.
type Snapshot struct {
	fs   SnapshotFS
	path pathx.RelPath
}

// SnapshotAt returns a Snapshot for the given path in fs.
//
// path is meant to be relative to fs.Root().
//
// If fs implements SnapshotRecorder, fs.MarkUsed will be called
// by AssertMatch.
func SnapshotAt(fs SnapshotFS, path pathx.RelPath) Snapshot {
	return Snapshot{fs: fs, path: path}
}

// AssertMatch compares got to the snapshot file. If -update is set,
// the snapshot file is written (creating directories as needed).
//
// If the underlying FS implements [SnapshotRecorder], then the
// snapshot path is marked as used.
func (s Snapshot) AssertMatch(h check.Harness, got string) {
	h.T().Helper()
	if recorder, ok := s.fs.(SnapshotRecorder); ok {
		recorder.MarkUsed(s.path)
	}

	if check.IsUpdateFlagSet() {
		if dir, ok := s.path.Dir().Get(); ok {
			h.NoErrorf(s.fs.MkdirAll(dir, 0o755),
				"creating parent directory for snapshot %s", s.path)
		}
		h.NoErrorf(s.fs.WriteFile(s.path, []byte(got), 0o644),
			"writing snapshot %s", s.path)
		h.Logf("updated snapshot: %s", s.path)
		return
	}

	wantBytes, err := s.fs.ReadFile(s.path)
	if err != nil {
		if errorx.GetRootCauseAsValue(err, fsx.ErrNotExist) {
			h.Assertf(false, "snapshot %s not found; run with -update to create it", s.path)
		} else {
			h.NoErrorf(err, "reading snapshot %s", s.path)
		}
		return
	}

	check.AssertSame(h, string(wantBytes), got, fmt.Sprintf("snapshot %s", s.path))
}

// SnapshotFS is the filesystem capability needed by snapshots.
type SnapshotFS interface {
	Root() pathx.AbsPath
	ReadFile(pathx.RelPath) ([]byte, error)
	WriteFile(pathx.RelPath, []byte, os.FileMode) error
	MkdirAll(pathx.RelPath, os.FileMode) error
	ReadDir(pathx.RelPath) iter.Seq[result.Result[fsx.DirEntry]]
	RemoveAll(pathx.RelPath) error
}

var _ SnapshotFS = (fsx.FS)(nil)

// SnapshotRecorder is an optional capability that records reads/writes
// against a snapshot path for later pruning.
type SnapshotRecorder interface {
	// MarkUsed marks a path as being 'used' by a test.
	//
	// This is used to garbage-collect old snapshots when all of a
	// package's tests are run together.
	//
	// This method must be concurrency-safe.
	MarkUsed(pathx.RelPath)
}

// snapshotFS is the SnapshotDirSet-backed implementation of SnapshotFS.
type snapshotFS struct {
	dir        *snapshotDir
	impl       fsx.FS
	pkgRelPath pathx.RelPath
}

var (
	_ SnapshotFS       = (*snapshotFS)(nil)
	_ SnapshotRecorder = (*snapshotFS)(nil)
)

func (fs snapshotFS) Root() pathx.AbsPath {
	return fs.impl.Root().Join(fs.pkgRelPath)
}

func (fs snapshotFS) ReadFile(path pathx.RelPath) ([]byte, error) {
	return fs.impl.ReadFile(fs.pkgRelPath.Join(path))
}

func (fs snapshotFS) WriteFile(path pathx.RelPath, data []byte, perm os.FileMode) error {
	return fs.impl.WriteFile(fs.pkgRelPath.Join(path), data, perm)
}

func (fs snapshotFS) MkdirAll(path pathx.RelPath, perm os.FileMode) error {
	return fs.impl.MkdirAll(fs.pkgRelPath.Join(path), perm)
}

func (fs snapshotFS) ReadDir(path pathx.RelPath) iter.Seq[result.Result[fsx.DirEntry]] {
	return fs.impl.ReadDir(fs.pkgRelPath.Join(path))
}

func (fs snapshotFS) RemoveAll(path pathx.RelPath) error {
	return fs.impl.RemoveAll(fs.pkgRelPath.Join(path))
}

func (fs snapshotFS) MarkUsed(path pathx.RelPath) {
	fs.dir.markUsed(path)
}

// snapshotDir holds per-directory bookkeeping used during pruning.
type snapshotDir struct {
	// pkgRelPath is the path to this directory relative to SnapshotDirSet.fs's root.
	pkgRelPath pathx.RelPath

	// mu protects usedFiles.
	mu sync.Mutex
	// usedFiles contains paths to files which have been read from/written to
	// at least once during the full run of all of the package's tests.
	usedFiles collections.Set[pathx.RelPath]
}

func (dir *snapshotDir) markUsed(path pathx.RelPath) {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	dir.usedFiles.Insert(path)
}

// shouldPruneSnapshots checks for the presence of 'go test' flags
// which could lead to incomplete coverage of code paths which read from/
// write to snapshot files.
//
// Returns either (true, "") or (false, "<flag>")
func shouldPruneSnapshots() (shouldPrune bool, foundFlag string) {
	// For example, when 'go test -run <test-regex>' is invoked, that gets
	// translated to a '<test-binary> -test.run <test-regex>' invocation by cmd/go.
	for _, name := range []string{"test.run", "test.skip", "test.list"} {
		if value := flagValue[string](name); value != "" {
			userFacingFlag := strings.ReplaceAll(name, "test.", "")
			return false, fmt.Sprintf("-%s=%s", userFacingFlag, value)
		}
	}
	if flagValue[bool]("test.short") {
		return false, "-short"
	}
	return true, ""
}

// flagValue returns the argument to the flag '-name=<arg>',
// interpreted as a T.
//
// Pre-condition: The flag 'name' must've been registered earlier
// with type T.
func flagValue[T any](name string) T {
	var zero T
	f := flag.Lookup(name)
	assert.Invariantf(f != nil, "testing flag %q is not registered", name)
	getter, ok := f.Value.(flag.Getter)
	assert.Invariantf(ok, "testing flag %q does not implement flag.Getter", name)
	value, ok := getter.Get().(T)
	assert.Invariantf(ok, "testing flag %q has type %T, want %T", name, getter.Get(), zero)
	return value
}
