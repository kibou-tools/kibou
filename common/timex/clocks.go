// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package timex

type TimestampClock interface {
	GetTimestamp() Timestamp
}
