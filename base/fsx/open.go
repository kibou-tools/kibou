// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx

import (
	"os"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/fsx/fsx_opt"
)

func NewOpenOptions(rw OpenRW) fsx_opt.OpenOptionsBuilder {
	return fsx_opt.Open(rw)
}

func OpenReadOnly(fs FS, rel pathx.RelPath) (File, error) {
	return fs.OpenFile(rel, NewOpenOptions(OpenRW_ReadOnly).MustBuild())
}

// OpenFile opens the file at the given root-relative path.
func (fs rootedFS) OpenFile(rel pathx.RelPath, opts OpenOptions) (File, error) {
	f, err := fs.base.OpenFile(rel.String(), openFlags(opts), openPerm(opts))
	if err != nil {
		return nil, err
	}
	return wrapFile(f), nil
}

func openFlags(opts OpenOptions) int {
	var flags int
	switch opts.RW() {
	case OpenRW_ReadOnly:
		flags = os.O_RDONLY
	case OpenRW_WriteOnly:
		flags = os.O_WRONLY
	case OpenRW_ReadWrite:
		flags = os.O_RDWR
	default:
		return assert.PanicUnknownCase[int](opts.RW())
	}
	if opts.Append() {
		flags |= os.O_APPEND
	}
	if opts.TruncateIfPresent() {
		flags |= os.O_TRUNC
	}
	switch opts.Mode() {
	case OpenMode_Existing:
	case OpenMode_CreateOrKeep:
		flags |= os.O_CREATE
	case OpenMode_CreateNew:
		flags |= os.O_CREATE | os.O_EXCL
	default:
		return assert.PanicUnknownCase[int](opts.Mode())
	}
	return flags
}

func openPerm(opts OpenOptions) os.FileMode {
	switch opts.Mode() {
	case OpenMode_Existing:
		return 0
	case OpenMode_CreateOrKeep, OpenMode_CreateNew:
		// FIXME(issue: https://github.com/kibou-tools/kibou/issues/216):
		// Expose a better way of specifying the permissions here.
		// This matches the defaults in Rust's cap-std and Java's NIO2.
		return 0o666
	default:
		return assert.PanicUnknownCase[os.FileMode](opts.Mode())
	}
}
