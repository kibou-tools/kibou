// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package timex

import "time"

// Duration is an elapsed time duration.
type Duration = time.Duration

const (
	Nanosecond  Duration = time.Nanosecond
	Microsecond Duration = time.Microsecond
	Millisecond Duration = time.Millisecond
	Second      Duration = time.Second
	Minute      Duration = time.Minute
	Hour        Duration = time.Hour
)
