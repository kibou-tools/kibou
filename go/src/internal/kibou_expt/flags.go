// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package kibou_expt

//go:generate go run mkconsts.go

// Flags describes the language settings enabled for
// a particular compilation.
type Flags struct {
	// Enums indicates whether support for sum types is enabled or not.
	Enums bool
}
