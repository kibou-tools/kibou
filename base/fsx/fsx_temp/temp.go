// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx_temp

import (
	"fmt"
	"iter"
	"strings"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/errorx"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/fsx/fsx_opt"
)

// CreateFile creates a temporary file in the directory 'dir' in 'fs'.
//
// The candidate file names are returned based on the names iterator.
//
// Pre-condition: opts must build successfully after setting
// [fsx.OpenMode_CreateNew]. In particular, its [fsx.OpenRW] must allow writes.
func CreateFile(
	fs fsx.FS, dir pathx.RelPath,
	names iter.Seq[fsx.Name],
	opts fsx_opt.OpenOptionsBuilder,
) (fsx.File, *CreateFileError) {
	opts_, err := opts.
		WithMode(fsx.OpenMode_CreateNew).
		Build()
	if err != nil {
		assert.Preconditionf(false, "CreateFile.opts failed to build with OpenMode_CreateNew: %v (input: %+v)", err, opts)
	}

	for name := range names {
		rel := dir.JoinOne(name)
		f, err := fs.OpenFile(rel, opts_)
		if err == nil {
			return f, nil
		}
		if errorx.GetRootCauseAsValue(err, fsx.ErrExist) {
			continue
		}
		assert.Invariant(err != nil, "nil I/O error")
		return nil, &CreateFileError{kind: CreateFileErrorKind_IOError, path: rel, err: err}
	}
	return nil, &CreateFileError{kind: CreateFileErrorKind_OutOfNames, path: pathx.RelPath{}, err: nil}
}

// CreateFileErrorKind classifies a [CreateFileError].
type CreateFileErrorKind uint8

const (
	// CreateFileErrorKind_IOError indicates a filesystem I/O failure while
	// trying to create a candidate file.
	//
	// Methods returning data:
	//   - [CreateFileError.Path]: root-relative path of the failing operation.
	//   - [CreateFileError.Unwrap]: the underlying I/O error.
	CreateFileErrorKind_IOError CreateFileErrorKind = iota + 1
	// CreateFileErrorKind_OutOfNames indicates that every candidate name was
	// already taken.
	CreateFileErrorKind_OutOfNames
)

// CreateFileError is the structured error type returned by [CreateFile].
type CreateFileError struct {
	kind CreateFileErrorKind
	path pathx.RelPath
	err  error
}

// Kind returns the error kind.
func (e *CreateFileError) Kind() CreateFileErrorKind {
	return e.kind
}

// Path returns the root-relative path associated with the error.
//
// Pre-condition: [CreateFileError.Kind] == [CreateFileErrorKind_IOError].
func (e *CreateFileError) Path() pathx.RelPath {
	switch e.kind {
	case CreateFileErrorKind_IOError:
		return e.path
	case CreateFileErrorKind_OutOfNames:
		assert.Preconditionf(false, "Path() called on CreateFileError kind %v", e.kind)
		return pathx.RelPath{}
	default:
		return assert.PanicUnknownCase[pathx.RelPath](e.kind)
	}
}

func (e *CreateFileError) Error() string {
	switch e.kind {
	case CreateFileErrorKind_IOError:
		return fmt.Sprintf("create temporary file %s: %v", e.path, e.err)
	case CreateFileErrorKind_OutOfNames:
		return "no temporary file candidate name could be created"
	default:
		return assert.PanicUnknownCase[string](e.kind)
	}
}

func (e *CreateFileError) Unwrap() error {
	return e.err
}

func Names(prefix []byte, fragments iter.Seq[[]byte], suffix []byte) iter.Seq[fsx.Name] {
	return func(yield func(fsx.Name) bool) {
		var builder strings.Builder
		for fragment := range fragments {
			builder.Grow(len(prefix) + len(fragment) + len(suffix))
			builder.Write(prefix)
			builder.Write(fragment)
			builder.Write(suffix)
			if !yield(fsx.NewName(builder.String())) {
				return
			}
			builder.Reset()
		}
	}
}
