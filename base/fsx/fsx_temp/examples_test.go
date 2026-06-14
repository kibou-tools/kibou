// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package fsx_temp_test

import (
	"io"

	"code.kibou.tools/base/core/pathx"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/fsx/fsx_temp"
	"code.kibou.tools/base/syscaps"
)

func ExampleCreateFile_withSyscapsTempFileNames() {
	repoFS, _ := fsx.MemMap(pathx.MustParseAbsPath("/repo"))

	file, _ := fsx_temp.CreateFile(
		repoFS,
		pathx.Dot(),
		syscaps.TempFileNames(`capture-*.jsonl`),
		fsx.NewOpenOptions(fsx.OpenRW_WriteOnly),
	)

	_, _ = io.WriteString(file, "{}\n")
	_ = file.Close()
}
