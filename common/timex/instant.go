// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package timex

import stdlib_time "time"

type Instant struct {
	// There doesn't seem to be a supported way of getting
	// just the monotonic value from stdlib_time.Time,
	// or for converting from a monotonic value to a stdlib_time.Time.
	//
	// The only possible hack seems to be we could use //go:linkname
	// to access the monotonic field directly.
	// Right now, //go:linkname is already forbidden for
	// some files, see go/src/cmd/link/internal/loader/loader.go.
	//
	// TODO(issue: https://github.com/kibou-tools/kibou/issues/160):
	// In the future, we should replace this, even if that means
	// directly doing syscalls ourselves as a fully supported
	// interface.
	value stdlib_time.Time
}

func NewInstant(value stdlib_time.Time) Instant {
	return Instant{value: value}
}

func (i Instant) IsZero() bool { return i.value.IsZero() }

func (i Instant) Add(d Duration) Instant { return Instant{value: i.value.Add(d)} }

func (i Instant) Sub(other Instant) Duration { return i.value.Sub(other.value) }

func (i Instant) IsBefore(other Instant) bool { return i.value.Before(other.value) }

func (i Instant) IsAfter(other Instant) bool { return i.value.After(other.value) }

func (i Instant) Equals(other Instant) bool {
	return i.value.Equal(other.value)
}

func (i Instant) Compare(other Instant) int {
	return i.value.Compare(other.value)
}

func (i Instant) AsStdlibTime() stdlib_time.Time { return i.value }
