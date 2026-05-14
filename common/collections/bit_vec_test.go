// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package collections_test

import (
	"testing"

	"pgregory.net/rapid"

	"code.kibou.tools/common/check"
	"code.kibou.tools/common/collections"
	"code.kibou.tools/common/core/option"
)

func TestBitVec(t *testing.T) {
	h := check.New(t)
	h.Run("unit", func(h check.Harness) {
		b := collections.NewBitVec(130)
		model := NewBitVecModel(130)
		assertBitVecState(h, &b, &model, "new")

		for _, i := range []int{0, 1, 63, 64, 65, 129} {
			applyBitVecOp(&b, &model, bitVecOp_Set, i)
		}
		for _, i := range []int{0, 64, 129} {
			applyBitVecOp(&b, &model, bitVecOp_Clear, i)
		}
		assertBitVecState(h, &b, &model, "after ops")
	})
	h.Run("property", func(h check.Harness) {
		rapid.Check(h.T(), func(t *rapid.T) {
			h := check.NewBasic(t)
			n := rapid.IntRange(0, 300).Draw(t, "len")
			bitVec := collections.NewBitVec(n)
			model := NewBitVecModel(n)
			assertBitVecState(h, &bitVec, &model, "rapid new")

			if n == 0 {
				return
			}
			ops := rapid.SliceOfN(rapid.Int(), 0, 300).Draw(t, "ops")
			for _, value := range ops {
				i := int(uint(value) % uint(n))
				op := bitVecOp(uint(value>>1) % 2)
				applyBitVecOp(&bitVec, &model, op, i)
			}
			assertBitVecState(h, &bitVec, &model, "rapid after ops")

			queries := rapid.SliceOfN(rapid.IntRange(0, n-1), 0, 300).Draw(t, "find queries")
			for _, i := range queries {
				assertBitVecFinds(h, &bitVec, &model, i, "rapid query")
			}
		})
	})
}

type bitVecOp uint8

const (
	bitVecOp_Set bitVecOp = iota
	bitVecOp_Clear
)

type BitVecModel struct {
	bits []bool
}

func NewBitVecModel(n int) BitVecModel {
	return BitVecModel{bits: make([]bool, n)}
}

func (m *BitVecModel) Len() int { return len(m.bits) }

func (m *BitVecModel) Get(i int) bool { return m.bits[i] }

func (m *BitVecModel) Set(i int) { m.bits[i] = true }

func (m *BitVecModel) Clear(i int) { m.bits[i] = false }

func (m *BitVecModel) FindAtOrBefore(i int) option.Option[int] {
	for i := i; 0 <= i; i-- {
		if m.bits[i] {
			return option.Some(i)
		}
	}
	return option.None[int]()
}

func (m *BitVecModel) FindAtOrAfter(i int) option.Option[int] {
	for i := i; i < len(m.bits); i++ {
		if m.bits[i] {
			return option.Some(i)
		}
	}
	return option.None[int]()
}

func assertBitVecState(h check.BasicHarness, bitVec *collections.BitVec, model *BitVecModel, what string) {
	check.AssertSame(h, model.Len(), bitVec.Len(), what+" len")
	for i := 0; i < model.Len(); i++ {
		check.AssertSame(h, model.Get(i), bitVec.Get(i), what+" bit")
	}
	for i := 0; i < model.Len(); i++ {
		assertBitVecFinds(h, bitVec, model, i, what)
	}
}

func assertBitVecFinds(h check.BasicHarness, bitVec *collections.BitVec, model *BitVecModel, i int, what string) {
	want := model.FindAtOrBefore(i)
	got := bitVec.FindAtOrBefore(i)
	check.AssertSame(h, 0, option.Compare(want, got), what+" find at or before")

	want = model.FindAtOrAfter(i)
	got = bitVec.FindAtOrAfter(i)
	check.AssertSame(h, 0, option.Compare(want, got), what+" find at or after")
}

func applyBitVecOp(bitVec *collections.BitVec, model *BitVecModel, op bitVecOp, i int) {
	switch op {
	case bitVecOp_Set:
		bitVec.Set(i)
		model.Set(i)
	case bitVecOp_Clear:
		bitVec.Clear(i)
		model.Clear(i)
	}
}
