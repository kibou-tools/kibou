// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package ranges

import "code.kibou.tools/common/assert"

// Bound identifies one of the two bounds of a half-open span.
type Bound bool

const (
	Bound_Start Bound = false
	Bound_End   Bound = true
)

func (b Bound) String() string {
	switch b {
	case Bound_Start:
		return "start"
	case Bound_End:
		return "end"
	default:
		return assert.PanicUnknownCase[string](b)
	}
}
