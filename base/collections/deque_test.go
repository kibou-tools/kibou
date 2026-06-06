// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package collections_test

import (
	"iter"
	"testing"

	"pgregory.net/rapid"

	"code.kibou.tools/base/check"
	"code.kibou.tools/base/collections"
)

// DequeModel is a linearly growing buffer model for Deque. It stores elements
// in logical order directly, without wrapping.
type DequeModel struct {
	values []int
}

func (m *DequeModel) Len() int {
	return len(m.values)
}

func (m *DequeModel) IsEmpty() bool {
	return m.Len() == 0
}

func (m *DequeModel) Iter() iter.Seq[int] {
	return func(yield func(int) bool) {
		for _, value := range m.values {
			if !yield(value) {
				return
			}
		}
	}
}

func (m *DequeModel) ReserveMore(int) {
	// No observable effect in the model.
}

func (m *DequeModel) PushFront(value int) {
	m.values = append([]int{value}, m.values...)
}

func (m *DequeModel) PushBack(value int) {
	m.values = append(m.values, value)
}

func (m *DequeModel) PopFront() int {
	value := m.values[0]
	m.values = m.values[1:]
	return value
}

func (m *DequeModel) PopBack() int {
	idx := len(m.values) - 1
	value := m.values[idx]
	m.values = m.values[:idx]
	return value
}

type dequeOp int

const (
	dequeOp_PushFront dequeOp = iota
	dequeOp_PushBack
	dequeOp_PopFront
	dequeOp_PopBack
)

type dequeOpSet struct {
	pushFront bool
	pushBack  bool
	popFront  bool
	popBack   bool
}

func (s dequeOpSet) enabledOps(nonEmpty bool) []dequeOp {
	ops := []dequeOp{}
	if s.pushFront {
		ops = append(ops, dequeOp_PushFront)
	}
	if s.pushBack {
		ops = append(ops, dequeOp_PushBack)
	}
	if nonEmpty && s.popFront {
		ops = append(ops, dequeOp_PopFront)
	}
	if nonEmpty && s.popBack {
		ops = append(ops, dequeOp_PopBack)
	}
	return ops
}

func (s dequeOpSet) drainOp(t *rapid.T) (dequeOp, bool) {
	switch {
	case s.popFront && s.popBack:
		if rapid.Bool().Draw(t, "drain dir") {
			return dequeOp_PopFront, true
		}
		return dequeOp_PopBack, true
	case s.popFront:
		return dequeOp_PopFront, true
	case s.popBack:
		return dequeOp_PopBack, true
	}
	return 0, false
}

func assertDequeState(
	h check.BasicHarness,
	deque *collections.Deque[int],
	model *DequeModel,
	what string,
) {
	check.AssertSame(h, model.Len(), deque.Len(), what+" len")
	check.AssertSame(h, model.IsEmpty(), deque.IsEmpty(), what+" empty")

	nextModel, stopModel := iter.Pull(model.Iter())
	defer stopModel()
	nextDeque, stopDeque := iter.Pull(deque.Iter())
	defer stopDeque()

	for range model.Len() {
		want, ok := nextModel()
		check.AssertSame(h, true, ok, what+" model iter has value")
		got, ok := nextDeque()
		check.AssertSame(h, true, ok, what+" deque iter has value")
		check.AssertSame(h, want, got, what+" iter value")
	}

	_, ok := nextModel()
	check.AssertSame(h, false, ok, what+" model iter exhausted")
	_, ok = nextDeque()
	check.AssertSame(h, false, ok, what+" deque iter exhausted")
}

func applyDequeOp(
	h check.BasicHarness,
	deque *collections.Deque[int],
	model *DequeModel,
	op dequeOp,
	value int,
	what string,
) {
	switch op {
	case dequeOp_PushFront:
		deque.PushFront(value)
		model.PushFront(value)
	case dequeOp_PushBack:
		deque.PushBack(value)
		model.PushBack(value)
	case dequeOp_PopFront:
		check.AssertSame(h, model.PopFront(), deque.PopFront(), what+" pop front")
	case dequeOp_PopBack:
		check.AssertSame(h, model.PopBack(), deque.PopBack(), what+" pop back")
	}
	assertDequeState(h, deque, model, what)
}

func TestDeque(t *testing.T) {
	h := check.New(t)
	h.Parallel()

	deque := collections.NewDeque[int]()
	model := DequeModel{values: nil}
	assertDequeState(h, &deque, &model, "new")

	deque.ReserveMore(4)
	model.ReserveMore(4)
	assertDequeState(h, &deque, &model, "after reserve")

	for _, step := range []struct {
		op    dequeOp
		value int
	}{
		{op: dequeOp_PushBack, value: 1},
		{op: dequeOp_PushFront, value: 2},
		{op: dequeOp_PushBack, value: 3},
		{op: dequeOp_PopBack, value: 0},
		{op: dequeOp_PushFront, value: 4},
		{op: dequeOp_PopFront, value: 0},
		{op: dequeOp_PopBack, value: 0},
		{op: dequeOp_PopFront, value: 0},
	} {
		applyDequeOp(h, &deque, &model, step.op, step.value, "scripted")
	}

	rapid.Check(t, func(t *rapid.T) {
		h := check.NewBasic(t)

		deque := collections.NewDeque[int]()
		model := DequeModel{values: nil}
		assertDequeState(h, &deque, &model, "rapid new")

		enabledPushes := rapid.SampledFrom([]dequeOpSet{
			{pushFront: true, pushBack: false, popFront: false, popBack: false},
			{pushFront: false, pushBack: true, popFront: false, popBack: false},
			{pushFront: true, pushBack: true, popFront: false, popBack: false},
		}).Draw(t, "enabled pushes")
		enabledPops := rapid.SampledFrom([]dequeOpSet{
			{pushFront: false, pushBack: false, popFront: false, popBack: false},
			{pushFront: false, pushBack: false, popFront: true, popBack: false},
			{pushFront: false, pushBack: false, popFront: false, popBack: true},
			{pushFront: false, pushBack: false, popFront: true, popBack: true},
		}).Draw(t, "enabled pops")
		enabled := dequeOpSet{
			pushFront: enabledPushes.pushFront,
			pushBack:  enabledPushes.pushBack,
			popFront:  enabledPops.popFront,
			popBack:   enabledPops.popBack,
		}

		ops := rapid.SliceOfN(rapid.Int(), 0, 100).Draw(t, "ops")
		reserveMore := rapid.IntRange(0, len(ops)).Draw(t, "reserve more")
		deque.ReserveMore(reserveMore)
		model.ReserveMore(reserveMore)
		assertDequeState(h, &deque, &model, "rapid after reserve")

		for _, value := range ops {
			choices := enabled.enabledOps(!model.IsEmpty())
			choice := choices[int(uint(value)%uint(len(choices)))]
			applyDequeOp(h, &deque, &model, choice, value, "rapid")
		}

		if enabled.popFront || enabled.popBack {
			for !model.IsEmpty() {
				op, _ := enabled.drainOp(t)
				applyDequeOp(h, &deque, &model, op, 0, "rapid drain")
			}
		}
	})
}
