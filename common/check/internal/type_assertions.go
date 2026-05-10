// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package internal

import (
	"code.kibou.tools/common/check"
	"code.kibou.tools/common/fsx"
)

var _ check.SnapshotFS = (fsx.FS)(nil)
