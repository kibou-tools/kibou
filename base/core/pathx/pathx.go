// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package pathx provides typed path wrappers for host platform paths.
//
// These types improve code clarity and catch potential bugs (e.g. accidentally
// passing a relative path where an absolute one is expected). They are not
// a security mechanism; for sandboxed filesystem access, use [os.Root].
package pathx

import (
	"iter"
	"path/filepath"
	"runtime"
	"strings"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/option"
	"code.kibou.tools/base/fsx/fsx_name"
	internal_pathx "code.kibou.tools/base/internal/pathx"
)

// AbsPath carries an absolute path that has gone through [LexicallyNormalize].
//
// It is guaranteed to be non-empty.
type AbsPath struct {
	value string
}

func ParseAbsPath(path string) (AbsPath, *AbsPathParseError) {
	if path == "" {
		return AbsPath{}, NewAbsPathParseError(AbsPathParseErrorKind_Empty, path)
	}
	if !filepath.IsAbs(path) {
		return AbsPath{}, NewAbsPathParseError(AbsPathParseErrorKind_NotAbsolute, path)
	}
	return AbsPath{LexicallyNormalize(path)}, nil
}

// MustParseAbsPath creates an AbsPath from an already-absolute path string.
//
// Pre-condition: path is non-empty and absolute per [filepath.IsAbs].
func MustParseAbsPath(path string) AbsPath {
	absPath, err := ParseAbsPath(path)
	if err != nil {
		switch err.Kind() {
		case AbsPathParseErrorKind_Empty:
			assert.Preconditionf(false, "MustParseAbsPath called with empty path")
		case AbsPathParseErrorKind_NotAbsolute:
			assert.Preconditionf(false, "MustParseAbsPath called with non-absolute path: %q", path)
		default:
			assert.PanicUnknownCase[any](err.Kind())
		}
	}
	return absPath
}

func (p AbsPath) String() string {
	return p.value
}

func (p AbsPath) Compare(other AbsPath) int {
	return strings.Compare(p.value, other.value)
}

// Dir returns the parent directory of p, or None if p is a filesystem root.
func (p AbsPath) Dir() option.Option[AbsPath] {
	rootLen := p.rootLen()
	if len(p.value) == rootLen {
		return option.None[AbsPath]()
	}
	if IsPathSeparator(p.value[len(p.value)-1]) {
		return option.Some(AbsPath{p.value[:len(p.value)-1]})
	}
	lastSep := len(p.value) - 1
	for lastSep >= rootLen && !IsPathSeparator(p.value[lastSep]) {
		lastSep--
	}
	if lastSep < rootLen {
		return option.Some(AbsPath{p.value[:rootLen]})
	}
	return option.Some(AbsPath{p.value[:lastSep]})
}

func (p AbsPath) rootLen() int {
	rootLen := len(filepath.VolumeName(p.value))
	if rootLen < len(p.value) && IsPathSeparator(p.value[rootLen]) {
		rootLen++
	}
	return rootLen
}

func (p AbsPath) Split() (AbsPath, fsx_name.Name) {
	dir, file := filepath.Split(p.value)
	return MustParseAbsPath(dir), fsx_name.New(file)
}

// Ancestors returns an iterator over p's ancestor absolute paths,
// in shortest-first order. The receiver itself is not yielded.
//
// For example, "/a/b/c" yields "/a" then "/a/b". A filesystem root
// or a path directly below a filesystem root yields nothing.
func (p AbsPath) Ancestors() iter.Seq[AbsPath] {
	return func(yield func(AbsPath) bool) {
		rootLen := p.rootLen()
		for i := rootLen; i < len(p.value); i++ {
			if !IsPathSeparator(p.value[i]) {
				continue
			}
			// A trailing separator means the ancestor would equal the
			// receiver semantically; stop here.
			if i == len(p.value)-1 {
				return
			}
			if !yield(AbsPath{p.value[:i]}) {
				return
			}
		}
	}
}

// LexicallyContains reports whether child is lexically contained within p.
func (p AbsPath) LexicallyContains(child RelPath) bool {
	if runtime.GOOS == "windows" {
		return p.lexicallyContainsSlow(child)
	}
	return child.lexicallyContainsUnix()
}

func (p AbsPath) lexicallyContainsSlow(child RelPath) bool {
	rel, err := filepath.Rel(p.value, filepath.Join(p.value, child.value))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (p AbsPath) Join(rel RelPath) AbsPath {
	return MustParseAbsPath(filepath.Join(p.value, rel.value))
}

// AppendExtension returns p with ext appended.
//
// Pre-conditions:
// 1. p must be non-empty.
// 2. p does not end with a path separator (i.e. p must be a valid file path).
func (p AbsPath) AppendExtension(ext string) AbsPath {
	if len(p.value) == 0 {
		assert.Precondition(false, "empty path")
	} else {
		lastCharStr := p.value[len(p.value)-1:]
		assert.Preconditionf(!HasPathSeparators(lastCharStr),
			"path %q ends with a path separator; so it's not a valid file path", p.value)
	}
	return MustParseAbsPath(p.value + ext)
}

// JoinComponents joins individual path components onto p.
//
// Pre-condition: no element contains a path separator.
func (p AbsPath) JoinComponents(pathElems ...string) AbsPath {
	parts := make([]string, 0, len(pathElems)+1)
	parts = append(parts, p.value)
	for _, elem := range pathElems {
		assert.Preconditionf(!HasPathSeparators(elem), "path element contains separator: %q", elem)
		parts = append(parts, elem)
	}
	return MustParseAbsPath(filepath.Join(parts...))
}

// MakeRelativeTo is the equivalent of filepath.Rel with typed paths.
//
// If the receiver and the root are the same, then
// Some(RootRelPath{root, Rel: "."}) will be returned.
func (p AbsPath) MakeRelativeTo(root AbsPath) option.Option[RootRelPath] {
	rel, err := filepath.Rel(root.value, p.value)
	if err != nil {
		return option.None[RootRelPath]()
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return option.None[RootRelPath]()
	}
	return option.Some(NewRootRelPath(root, MustParseRelPath(rel)))
}

// RelPath carries a relative path that has gone through [LexicallyNormalize].
//
// It is guaranteed to be non-empty.
type RelPath struct {
	value string
}

// Dot returns a relative path '.'.
func Dot() RelPath {
	return RelPath{"."}
}

// ParseRelPath creates a RelPath from a relative path string.
func ParseRelPath(path string) (RelPath, *RelPathParseError) {
	if path == "" {
		return RelPath{}, NewRelPathParseError(RelPathParseErrorKind_Empty, path)
	}
	if filepath.IsAbs(path) {
		return RelPath{}, NewRelPathParseError(RelPathParseErrorKind_NotRelative, path)
	}
	return RelPath{LexicallyNormalize(path)}, nil
}

// MustParseRelPath creates a RelPath from a relative path string.
//
// Pre-condition: path is non-empty and not absolute per [filepath.IsAbs].
func MustParseRelPath(path string) RelPath {
	relPath, err := ParseRelPath(path)
	if err != nil {
		switch err.Kind() {
		case RelPathParseErrorKind_Empty:
			assert.Preconditionf(false, "MustParseRelPath called with empty path")
		case RelPathParseErrorKind_NotRelative:
			assert.Preconditionf(false, "MustParseRelPath called with absolute path: %q", path)
		default:
			assert.PanicUnknownCase[any](err.Kind())
		}
	}
	return relPath
}

// NewRelPathFromName returns name as a single-component relative path.
func NewRelPathFromName(name fsx_name.Name) RelPath {
	return RelPath{name.String()}
}

// String is guaranteed to be "." if a relative path for the current directory.
func (p RelPath) String() string {
	return p.value
}

func (p RelPath) Compare(other RelPath) int {
	return strings.Compare(p.value, other.value)
}

// Dir returns the parent directory of p, or None if p is ".".
func (p RelPath) Dir() option.Option[RelPath] {
	parent := filepath.Dir(p.value)
	if parent == p.value || parent == "." {
		return option.None[RelPath]()
	}
	return option.Some(MustParseRelPath(parent))
}

func (p RelPath) Join(rel RelPath) RelPath {
	return MustParseRelPath(filepath.Join(p.value, rel.value))
}

// RelativeTo returns p expressed as a relative path from base.
//
// Pre-condition: base is an ancestor of p, or equal to p.
// In particular, base == "." returns p unchanged, and base == p returns ".".
func (p RelPath) RelativeTo(base RelPath) RelPath {
	if base.value == "." {
		return p
	}
	suffix, ok := strings.CutPrefix(p.value, base.value)
	assert.Preconditionf(ok, "base %q is not an ancestor of %q", base.value, p.value)
	if suffix == "" {
		return Dot()
	}
	assert.Preconditionf(IsPathSeparator(suffix[0]), "base %q is not an ancestor of %q", base.value, p.value)
	return RelPath{suffix[1:]}
}

func (p RelPath) JoinOne(name fsx_name.Name) RelPath {
	// TODO: Use an unchecked code path here, because we know the invariant can't be violated.
	return MustParseRelPath(filepath.Join(p.value, name.String()))
}

// BaseName returns the final path component of p.
func (p RelPath) BaseName() fsx_name.Name {
	return fsx_name.New(filepath.Base(p.value))
}

// JoinComponents joins individual path components onto p.
//
// Pre-condition: all elements are non-empty and do not contain a path separator.
func (p RelPath) JoinComponents(pathElems ...string) RelPath {
	parts := make([]string, 0, len(pathElems)+1)
	parts = append(parts, p.value)
	for _, elem := range pathElems {
		assert.Preconditionf(!HasPathSeparators(elem), "path element contains separator: %q", elem)
		parts = append(parts, elem)
	}
	return MustParseRelPath(filepath.Join(parts...))
}

// Ancestors returns an iterator over p's ancestor relative paths,
// in shortest-first order. The receiver itself is not yielded.
//
// For example, "a/b/c" yields "a" then "a/b". A path of "."
// or a single-component path yields nothing.
func (p RelPath) Ancestors() iter.Seq[RelPath] {
	return func(yield func(RelPath) bool) {
		if p.value == "." {
			return
		}
		n := len(p.value)
		for i := range n {
			if !IsPathSeparator(p.value[i]) {
				continue
			}
			// A trailing separator means the ancestor would equal the
			// receiver semantically; stop here.
			if i == n-1 {
				return
			}
			if !yield(RelPath{p.value[:i]}) {
				return
			}
		}
	}
}

func (p RelPath) Components() iter.Seq[string] {
	return internal_pathx.Components(p.value).Components()
}

func (p RelPath) lexicallyContainsUnix() bool {
	depth := 0
	for component := range p.Components() {
		switch component {
		case ".":
			continue
		case "..":
			if depth == 0 {
				return false
			}
			depth--
		default:
			depth++
		}
	}
	return true
}

// HasPathSeparators reports whether s contains any path separators.
func HasPathSeparators(s string) bool {
	for i := range len(s) {
		if IsPathSeparator(s[i]) {
			return true
		}
	}
	return false
}

type RootRelPath struct {
	root  AbsPath
	value RelPath
}

// NewRootRelPath creates a RootRelPath anchored at root.
//
// Pre-condition: subpath does not escape root (per [AbsPath.LexicallyContains]).
func NewRootRelPath(root AbsPath, subpath RelPath) RootRelPath {
	assert.Preconditionf(root.LexicallyContains(subpath), "subpath %q escapes root %q", subpath.value, root.value)
	return RootRelPath{root: root, value: subpath}
}

func (p RootRelPath) String() string {
	return p.value.value
}

func (p RootRelPath) Compare(other RootRelPath) int {
	if c := p.root.Compare(other.root); c != 0 {
		return c
	}
	return p.value.Compare(other.value)
}

func (p RootRelPath) AsAbsPath() AbsPath {
	return p.root.Join(p.value)
}

// Rel returns the anchored relative portion of p, discarding the root.
func (p RootRelPath) Rel() RelPath {
	return p.value
}
