// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package fsx_opt covers different options used to configure
// filesystem operations.
package fsx_opt

import (
	"fmt"

	"code.kibou.tools/base/assert"
)

// OpenOptions represents the options for opening a file.
type OpenOptions struct {
	rw       OpenRW
	mode     OpenMode
	append   bool
	truncate bool
}

// RW returns whether the file is meant to be opened as
// read-only/write-only/read-write.
func (o OpenOptions) RW() OpenRW { return o.rw }

// Mode returns whether the file should be created and/or
// if an existing file can be opened as-is.
func (o OpenOptions) Mode() OpenMode { return o.mode }

// Append returns whether the file will be opened in
// append-mode.
func (o OpenOptions) Append() bool {
	return o.append
}

func (o OpenOptions) TruncateIfPresent() bool {
	return o.truncate
}

// OpenRW represents the read and/or write permissions
// for a file open operation.
type OpenRW uint8

const (
	// OpenRW_ReadOnly is used to open a file in read-only mode.
	OpenRW_ReadOnly OpenRW = iota + 1
	// OpenRW_WriteOnly is used to open a file in write-only mode.
	OpenRW_WriteOnly
	// OpenRW_ReadWrite is used to open a file in read-write mode.
	OpenRW_ReadWrite
)

func (rw OpenRW) String() string {
	switch rw {
	case OpenRW_ReadOnly:
		return "OpenRW_ReadOnly"
	case OpenRW_WriteOnly:
		return "OpenRW_WriteOnly"
	case OpenRW_ReadWrite:
		return "OpenRW_ReadWrite"
	default:
		return assert.PanicUnknownCase[string](rw)
	}
}

// Label returns a friendly kebab-case label for OpenRW suitable
// for user-facing messages in English.
func (rw OpenRW) Label() string {
	switch rw {
	case OpenRW_ReadOnly:
		return "read-only"
	case OpenRW_WriteOnly:
		return "write-only"
	case OpenRW_ReadWrite:
		return "read-write"
	default:
		return assert.PanicUnknownCase[string](rw)
	}
}

// OpenMode represents whether an existing file may be opened
// and/or whether a new file is created.
type OpenMode uint8

const (
	// OpenMode_Existing only opens the file if it exists already.
	// If the file doesn't exist, [fsx.ErrNotExist] is returned.
	OpenMode_Existing OpenMode = iota + 1
	// OpenMode_CreateOrKeep opens the file if it exists already,
	// or creates a new file and opens it if it doesn't exist.
	OpenMode_CreateOrKeep
	// OpenMode_CreateNew creates the file only if it doesn't exist already,
	// and opens the file.
	//
	// If the file already exists, [fsx.ErrExist] is returned.
	OpenMode_CreateNew
)

func (mode OpenMode) String() string {
	switch mode {
	case OpenMode_Existing:
		return "OpenMode_Existing"
	case OpenMode_CreateOrKeep:
		return "OpenMode_CreateOrKeep"
	case OpenMode_CreateNew:
		return "OpenMode_CreateNew"
	default:
		return assert.PanicUnknownCase[string](mode)
	}
}

// OpenOptionsBuilder is a builder struct used for creating an OpenOptions.
type OpenOptionsBuilder struct {
	rw       OpenRW
	mode     OpenMode
	append   bool
	truncate bool
}

// Open creates a new OpenOptionsBuilder, which can be constructed
// with [OpenOptionsBuilder.Build] or [OpenOptionsBuilder.MustBuild].
//
// For example, if you want to open a log file for appending,
// you'd typically do:
//
//	opts := fsx_opt.Open(fsx_opt.OpenRW_WriteOnly).
//		WithMode(fsx_opt.OpenMode_CreateOrKeep).
//		WithAppend(true).
//		MustBuild()
func Open(rw OpenRW) OpenOptionsBuilder {
	switch rw {
	case OpenRW_ReadOnly, OpenRW_WriteOnly, OpenRW_ReadWrite:
	default:
		assert.Preconditionf(false, "unknown value for fsx_opt.OpenRW %d", rw)
	}
	return OpenOptionsBuilder{rw: rw, mode: OpenMode_Existing, append: false, truncate: false}
}

func (b OpenOptionsBuilder) WithMode(mode OpenMode) OpenOptionsBuilder {
	switch mode {
	case OpenMode_Existing, OpenMode_CreateOrKeep, OpenMode_CreateNew:
	default:
		assert.Preconditionf(false, "unknown value for fsx_opt.OpenMode %d", mode)
	}
	b.mode = mode
	return b
}

// WithAppend changes whether the file should be opened in append
// mode or not.
//
// NOTE: This is incompatible with OpenRW_ReadOnly, and will lead to
// an error with Build and a panic with MustBuild.
func (b OpenOptionsBuilder) WithAppend(append bool) OpenOptionsBuilder {
	b.append = append
	return b
}

// WithTruncateIfPresent changes whether the file should be truncated
// if it's already present on disk, or not.
//
// NOTE: This is incompatible with OpenRW_ReadOnly, and will lead to
// an error with Build and a panic with MustBuild.
func (b OpenOptionsBuilder) WithTruncateIfPresent(truncate bool) OpenOptionsBuilder {
	b.truncate = truncate
	return b
}

// Build attempts to Build an OpenOptions, returning an error
// if some invalid configuration of options was configured.
//
// Currently, an error is returned if the following are combined
// with OpenRW_ReadOnly:
//
// - OpenMode_CreateOrKeep
// - OpenMode_CreateNew
// - TruncateIfPresent(true)
// - Append(true)
//
// The set of errors may increase over time.
func (b OpenOptionsBuilder) Build() (OpenOptions, *OpenOptionsBuildError) {
	switch b.rw {
	case OpenRW_ReadOnly:
		if b.mode != OpenMode_Existing {
			return OpenOptions{}, missingWritePerm(b.mode.String())
		}
		if b.truncate {
			return OpenOptions{}, missingWritePerm("OpenOptions.TruncateIfPresent=true")
		}
		if b.append {
			return OpenOptions{}, missingWritePerm("OpenOptions.Append=true")
		}
	case OpenRW_WriteOnly, OpenRW_ReadWrite:
	default:
		assert.PanicUnknownCase[struct{}](b.rw)
	}
	if b.mode == OpenMode_CreateNew && b.truncate { //nolint:staticcheck
		// TODO: In this situation, b.truncate will not have
		// the intended effect; it will just be ignored.
		// Perhaps we should emit a warning using a separate API.
	}
	return OpenOptions(b), nil
}

// MustBuild asserts that the builder is well-formed and returns
// the built OpenOptions.
//
// See [OpenOptionsBuilder.Build] for the possible error cases.
func (b OpenOptionsBuilder) MustBuild() OpenOptions {
	opts, err := b.Build()
	if err != nil {
		assert.Preconditionf(false, "invalid OpenOptions: %v", err)
	}
	return opts
}

type OpenOptionsBuildError struct {
	kind     OpenOptionsBuildErrorKind
	neededBy string
}

type OpenOptionsBuildErrorKind uint8

const (
	OpenOptionsBuildErrorKind_MissingWritePermission OpenOptionsBuildErrorKind = iota + 1
)

func NewOpenOptionsBuildError(kind OpenOptionsBuildErrorKind, neededBy string) *OpenOptionsBuildError {
	return &OpenOptionsBuildError{kind: kind, neededBy: neededBy}
}

func (e *OpenOptionsBuildError) Kind() OpenOptionsBuildErrorKind {
	return e.kind
}

// NeededBy returns which setting for OpenOptionsBuilder needed the
// necessary permission.
func (e *OpenOptionsBuildError) NeededBy() string {
	return e.neededBy
}

func (e *OpenOptionsBuildError) Error() string {
	switch e.kind {
	case OpenOptionsBuildErrorKind_MissingWritePermission:
		return fmt.Sprintf("%s requires OpenRW_WriteOnly or OpenRW_ReadWrite", e.neededBy)
	default:
		return assert.PanicUnknownCase[string](e.kind)
	}
}

func missingWritePerm(neededBy string) *OpenOptionsBuildError {
	return NewOpenOptionsBuildError(
		OpenOptionsBuildErrorKind_MissingWritePermission,
		neededBy)
}
