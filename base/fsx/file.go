// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx

import (
	"github.com/spf13/afero" //nolint:depguard // fsx is the designated wrapper

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/fsx/fsx_name"
)

type file struct {
	base afero.File
}

func wrapFile(base afero.File) File {
	assert.Invariant(base != nil, "cannot wrap nil file")
	return file{base: base}
}

func (f file) Close() error {
	return f.base.Close()
}

func (f file) Read(p []byte) (int, error) {
	return f.base.Read(p)
}

func (f file) ReadAt(p []byte, off int64) (int, error) {
	return f.base.ReadAt(p, off)
}

func (f file) Seek(offset int64, whence int) (int64, error) {
	return f.base.Seek(offset, whence)
}

func (f file) Write(p []byte) (int, error) {
	return f.base.Write(p)
}

func (f file) WriteAt(p []byte, off int64) (int, error) {
	return f.base.WriteAt(p, off)
}

func (f file) Name() Name {
	baseName := f.base.Name()
	name, err := fsx_name.ExtractBaseName(baseName)
	if err != nil {
		assert.Invariantf(false, "filesystem returned file name without basename: %v", err)
	}
	return name
}

func (f file) Truncate(size int64) error {
	return f.base.Truncate(size)
}
