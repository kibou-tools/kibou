// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package collections_test

import (
	"testing"

	"pgregory.net/rapid"

	"code.kibou.tools/base/check"
	"code.kibou.tools/base/collections"
	"code.kibou.tools/base/core/op"
	"code.kibou.tools/base/iterx"
)

func TestOrderedMap(t *testing.T) {
	h := check.New(t)

	h.Run("Unit", func(h check.Harness) {
		h.Parallel()

		m := collections.NewOrderedMap[string, int]()
		h.Assertf(!m.Lookup("missing").IsSome(), "missing key unexpectedly present")
		h.Assertf(m.InsertOrKeep("a", 1) == op.InsertedNew, "first insert should report InsertedNew")
		h.Assertf(m.InsertOrKeep("a", 2) == op.KeptOld, "duplicate insert should report KeptOld")

		old, ok := m.InsertOrReplace("a", 3).Get()
		h.Assertf(ok, "InsertOrReplace should return the old value")
		h.Assertf(old == 1, "InsertOrReplace returned old value %d, want 1", old)

		h.Assertf(m.InsertOrKeep("b", 4) == op.InsertedNew, "insert of b should report InsertedNew")
		check.AssertSame(h, []string{"a", "b"}, iterx.Collect(m.Keys()), "Keys()")

		deleted, ok := m.Delete("a").Get()
		h.Assertf(ok, "Delete should return the old value")
		h.Assertf(deleted == 3, "Delete returned %d, want 3", deleted)
		h.Assertf(!m.Lookup("a").IsSome(), "deleted key unexpectedly present")
		check.AssertSame(h, []string{"b"}, iterx.Collect(m.Keys()), "Keys() after delete")

		h.Assertf(m.InsertOrKeep("a", 5) == op.InsertedNew, "reinsert of deleted key should report InsertedNew")
		check.AssertSame(h, []string{"b", "a"}, iterx.Collect(m.Keys()), "Keys() after reinsert")

		compacted := collections.NewOrderedMap[int, int]()
		for i := range 200 {
			compacted.InsertOrReplace(i, i*10)
		}
		for i := range 150 {
			_, ok := compacted.Delete(i).Get()
			h.Assertf(ok, "Delete(%d) should find inserted key", i)
		}

		wantCompacted := make([]int, 0, 50)
		for i := 150; i < 200; i++ {
			wantCompacted = append(wantCompacted, i)
		}
		check.AssertSame(h, wantCompacted, iterx.Collect(compacted.Keys()), "Keys() after compaction")
		h.Assertf(compacted.Len() == len(wantCompacted), "Len() = %d, want %d", compacted.Len(), len(wantCompacted))

		compacted.InsertOrReplace(200, 2000)
		wantCompacted = append(wantCompacted, 200)
		check.AssertSame(h, wantCompacted, iterx.Collect(compacted.Keys()), "Keys() after post-compaction insert")
	})

	h.Run("Properties", func(h check.Harness) {
		h.Parallel()

		opsGen := rapid.SliceOfN(rapid.Custom(func(t *rapid.T) orderedMapOp {
			return orderedMapOp{
				kind:  orderedMapOpKind(rapid.IntRange(0, 2).Draw(t, "kind")),
				key:   rapid.IntRange(-20, 20).Draw(t, "key"),
				value: rapid.Int().Draw(t, "value"),
			}
		}), 0, 200)
		rapid.Check(h.T(), func(t *rapid.T) {
			h := check.NewBasic(t)
			ops := opsGen.Draw(t, "ops")
			checkKeyRolls := rapid.SliceOfN(rapid.IntRange(0, 15), len(ops), len(ops)).Draw(t, "check_key_rolls")

			impl := collections.NewOrderedMap[int, int]()
			model := collections.NewMonotoneMap[int, int]()
			for step, op := range ops {
				op.Apply(h, step, &impl, &model)

				if checkKeyRolls[step] == 0 {
					check.AssertSame(h, iterx.Collect(model.Keys()), iterx.Collect(impl.Keys()),
						"Keys() during model run")
				}
				h.Assertf(impl.Len() == model.Len(), "step %d: Len() = %d, want %d", step, impl.Len(), model.Len())
				modelValue, modelHasValue := model.Lookup(op.key).Get()
				implValue, implHasValue := impl.Lookup(op.key).Get()
				h.Assertf(implHasValue == modelHasValue,
					"step %d: Lookup(%d) presence = %v, want %v", step, op.key, implHasValue, modelHasValue)
				if modelHasValue {
					h.Assertf(implValue == modelValue,
						"step %d: Lookup(%d) = %d, want %d", step, op.key, implValue, modelValue)
				}
			}
			check.AssertSame(h, iterx.Collect(model.Keys()), iterx.Collect(impl.Keys()), "final Keys()")
		})
	})
}

type orderedMapOp struct {
	kind  orderedMapOpKind
	key   int
	value int
}

type orderedMapOpKind int

const (
	orderedMapOp_InsertOrKeep orderedMapOpKind = iota
	orderedMapOp_InsertOrReplace
	orderedMapOp_Delete
)

func (op orderedMapOp) Apply(
	h check.BasicHarness,
	step int,
	impl *collections.OrderedMap[int, int],
	model *collections.MonotoneMap[int, int],
) {
	switch op.kind {
	case orderedMapOp_InsertOrKeep:
		implResult := impl.InsertOrKeep(op.key, op.value)
		modelResult := model.InsertOrKeep(op.key, op.value)
		h.Assertf(implResult == modelResult,
			"step %d: InsertOrKeep(%d, %d) = %v, want %v", step, op.key, op.value, implResult, modelResult)
	case orderedMapOp_InsertOrReplace:
		implOld, implHadOld := impl.InsertOrReplace(op.key, op.value).Get()
		modelOld, modelHadOld := model.InsertOrReplace(op.key, op.value).Get()
		h.Assertf(implHadOld == modelHadOld,
			"step %d: InsertOrReplace(%d, %d) old presence = %v, want %v",
			step, op.key, op.value, implHadOld, modelHadOld)
		if modelHadOld {
			h.Assertf(implOld == modelOld,
				"step %d: InsertOrReplace(%d, %d) old = %d, want %d",
				step, op.key, op.value, implOld, modelOld)
		}
	case orderedMapOp_Delete:
		implOld, implHadOld := impl.Delete(op.key).Get()
		modelOld, modelHadOld := model.Lookup(op.key).Get()
		omit := collections.NewSet[int]()
		omit.Insert(op.key)
		*model = model.CloneWithout(omit)
		h.Assertf(implHadOld == modelHadOld,
			"step %d: Delete(%d) old presence = %v, want %v", step, op.key, implHadOld, modelHadOld)
		if modelHadOld {
			h.Assertf(implOld == modelOld,
				"step %d: Delete(%d) old = %d, want %d", step, op.key, implOld, modelOld)
		}
	default:
		h.Assertf(false, "step %d: unknown op kind %d", step, op.kind)
	}
}
