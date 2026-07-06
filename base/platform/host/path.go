// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package host provides path types for the operating system that the current
// program is compiled for.
//
// Each type wraps the corresponding platform-specific type, selected at build
// time: on Unix the impl types alias package unix, and on Windows package
// windows. Code that only deals with the host's own paths can use these types
// and stay platform-agnostic.
package host

import "code.kibou.tools/base/platform/platform_core"

// PathClassification describes how a path's meaning depends on process state.
//
// It is re-exported from platform_core so host consumers can classify paths
// without importing that package directly.
type PathClassification = platform_core.PathClassification

const (
	PathClassification_Absolute = platform_core.PathClassification_Absolute
	PathClassification_Relative = platform_core.PathClassification_Relative
	PathClassification_Neither  = platform_core.PathClassification_Neither
)

type Path struct {
	impl pathImpl
}

// Classify describes how this path's meaning depends on process state.
func (p Path) Classify() PathClassification {
	return p.impl.Classify()
}

type AbsPath struct {
	impl absPathImpl
}

type RelPath struct {
	impl relPathImpl
}

type NormalizedPath struct {
	impl normalizedPathImpl
}

type NormalizedAbsPath struct {
	impl normalizedAbsPathImpl
}

type NormalizedRelPath struct {
	impl normalizedRelPathImpl
}
