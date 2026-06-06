// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package errorx

import (
	"strconv"
	"unsafe"

	"code.kibou.tools/common/assert"
)

// StableFormatter implements Formatter using a deterministic, single-line,
// human-readable format.
//
// StableFormatter does not redact values. It is intended for tests and other
// callers that want a stable rendering contract instead of snapshotting a
// human-facing pretty formatter.
//
// StableFormatter always renders formatted fields in parentheses, even when
// there is no surrounding message text.
//
// StableFormatter values must be constructed with [NewStableFormatter] and must
// not be copied.
type StableFormatter struct {
	self       *StableFormatter
	buf        []byte
	state      fmtState
	fieldCount int
}

type fmtState uint8

const (
	// fmtState_Empty means the formatter has not emitted anything yet.
	fmtState_Empty fmtState = iota + 1
	// fmtState_Text means the formatter has emitted text that should precede any
	// following field list.
	fmtState_Text
	// fmtState_ParenFields means the formatter is emitting a parenthesized field
	// list.
	fmtState_ParenFields
)

var _ Formatter = (*StableFormatter)(nil)

// NewStableFormatter returns an empty StableFormatter.
func NewStableFormatter() *StableFormatter {
	f := &StableFormatter{self: nil, buf: nil, state: fmtState_Empty, fieldCount: 0}
	f.self = f
	return f
}

// Finish returns the formatted output and consumes the formatter's current
// buffer. Formatting after Finish starts a fresh buffer.
func (f *StableFormatter) Finish() string {
	f.copyCheck()
	switch f.state {
	case fmtState_Empty, fmtState_Text:
		// Nothing to close.
	case fmtState_ParenFields:
		f.buf = append(f.buf, ')')
	default:
		return assert.PanicUnknownCase[string](f.state)
	}
	f.state = fmtState_Empty
	f.fieldCount = 0
	var out string
	if len(f.buf) > 0 {
		out = unsafe.String(&f.buf[0], len(f.buf))
	}
	f.buf = nil
	return out
}

func (f *StableFormatter) FormatDynamic(vk ValueKind, key string, value string) {
	f.copyCheck()
	if vk < ValueKind_StdMin || ValueKind_StdMax < vk {
		assert.Preconditionf(false, "value kind %d is out of range [%d, %d]",
			vk, ValueKind_StdMin, ValueKind_StdMax)
	}
	f.appendField(func(buf []byte) []byte {
		buf = appendKeyPrefix(buf, key)
		return strconv.AppendQuote(buf, value)
	})
}

func (f *StableFormatter) FormatBool(key string, value bool) {
	f.copyCheck()
	f.appendField(func(buf []byte) []byte {
		buf = appendKeyPrefix(buf, key)
		return strconv.AppendBool(buf, value)
	})
}

func (f *StableFormatter) FormatUint64(key string, value uint64) {
	f.copyCheck()
	f.appendField(func(buf []byte) []byte {
		buf = appendKeyPrefix(buf, key)
		return strconv.AppendUint(buf, value, 10)
	})
}

func (f *StableFormatter) FormatUintptr(key string, value uintptr) {
	f.copyCheck()
	f.appendField(func(buf []byte) []byte {
		buf = appendKeyPrefix(buf, key)
		return strconv.AppendUint(buf, uint64(value), 10)
	})
}

func (f *StableFormatter) FormatInt64(key string, value int64) {
	f.copyCheck()
	f.appendField(func(buf []byte) []byte {
		buf = appendKeyPrefix(buf, key)
		return strconv.AppendInt(buf, value, 10)
	})
}

func (f *StableFormatter) FormatFloat64(key string, value float64) {
	f.copyCheck()
	f.appendField(func(buf []byte) []byte {
		buf = appendKeyPrefix(buf, key)
		// 'g': switch between compact and general format
		// depending on the value.
		// -1: minimize number of digits used.
		return strconv.AppendFloat(buf, value, 'g', -1, 64)
	})
}

func (f *StableFormatter) FormatConstMsg(value string) {
	f.copyCheck()
	f.closeFieldList()
	if value != "" {
		f.buf = append(f.buf, value...)
		f.state = fmtState_Text
	}
}

func (f *StableFormatter) FormatConstString(key string, value string) {
	f.copyCheck()
	f.appendField(func(buf []byte) []byte {
		buf = appendKeyPrefix(buf, key)
		return strconv.AppendQuote(buf, value)
	})
}

func (f *StableFormatter) FormatGroup(key string, forEach func(Formatter)) {
	f.copyCheck()
	f.appendField(func(buf []byte) []byte {
		buf = appendKeyPrefix(buf, key)
		return append(buf, '{')
	})

	parentState := f.state
	parentFieldCount := f.fieldCount
	f.state = fmtState_Empty
	f.fieldCount = 0
	forEach(f)
	f.closeFieldList()
	f.buf = append(f.buf, '}')
	f.state = parentState
	f.fieldCount = parentFieldCount
}

func (f *StableFormatter) Err() *FormatterError {
	f.copyCheck()
	return nil
}

func (f *StableFormatter) copyCheck() {
	if f.self != f {
		if f.self == nil {
			assert.Invariantf(false, "StableFormatter was not constructed with NewStableFormatter")
		}
		assert.Invariantf(false, "StableFormatter was copied")
	}
}

func (f *StableFormatter) appendField(write func([]byte) []byte) {
	switch f.state {
	case fmtState_Empty, fmtState_Text:
		f.openFieldList()
	case fmtState_ParenFields:
		// Continue the current field list.
	default:
		assert.PanicUnknownCase[any](f.state)
	}
	if f.fieldCount > 0 {
		f.buf = append(f.buf, ", "...)
	}
	f.buf = write(f.buf)
	f.fieldCount++
}

func (f *StableFormatter) openFieldList() {
	f.fieldCount = 0
	switch f.state {
	case fmtState_Empty:
		f.buf = append(f.buf, '(')
	case fmtState_Text:
		f.buf = append(f.buf, " ("...)
	case fmtState_ParenFields:
		assert.PanicInvariantViolation[any]("field list is already open")
	default:
		assert.PanicUnknownCase[any](f.state)
	}
	f.state = fmtState_ParenFields
}

func (f *StableFormatter) closeFieldList() {
	switch f.state {
	case fmtState_Empty, fmtState_Text:
		// Nothing to close.
	case fmtState_ParenFields:
		f.buf = append(f.buf, ')')
		f.state = fmtState_Text
	default:
		assert.PanicUnknownCase[any](f.state)
	}
	f.fieldCount = 0
}

func appendKeyPrefix(buf []byte, key string) []byte {
	if key == "" {
		return buf
	}
	buf = append(buf, key...)
	return append(buf, '=')
}
