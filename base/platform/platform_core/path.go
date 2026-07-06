// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package platform_core

type PathClassification uint8

const (
	// Indicates that the meaning of the path does not depend on the global
	// state of the process using the path.
	PathClassification_Absolute PathClassification = iota + 1
	// Indicates that the meaning of the path depends on the working directory
	// for the process using the path.
	PathClassification_Relative
	// Indicates that the meaning of the path depends on some process-global
	// state other than the working directory.
	PathClassification_Neither
)
