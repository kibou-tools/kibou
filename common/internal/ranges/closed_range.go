// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package ranges

type ClosedRange[T int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64] struct {
	Lo T
	Hi T
}

func (r ClosedRange[T]) Contains(t T) bool {
	return r.Lo <= t && t <= r.Hi
}
