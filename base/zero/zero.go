// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package zero provides the [Zero] generic zero-value helper.
//
// Intended to be dot-imported so callers can write [Zero][T] directly.
package zero

// Zero returns the zero value of T.
//
// The primary use case for this to be used in return statements
// of the form return Zero[T](), <error> where <error> is some
// non-heap-allocating type (e.g. a boolean, some integer type)
// that signals an error.
//
// If the type of the second return value is `error`, then exhaustruct
// correctly allows T{}, <error>. However, exhaustruct does not extend
// this generality to other error-like return types.
//
// Avoid using this except in return statements.
func Zero[T any]() (t T) { return }
