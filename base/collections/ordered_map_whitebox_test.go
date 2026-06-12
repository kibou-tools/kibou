// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package collections

import (
	"testing"

	"pgregory.net/rapid"

	"code.kibou.tools/base/check"
)

func TestOrderedMapWhitebox(t *testing.T) {
	h := check.New(t)

	h.Run("Properties", func(h check.Harness) {
		h.Parallel()

		deletionCountGen := rapid.IntRange(0, 300)
		rapid.Check(h.T(), func(t *rapid.T) {
			h := check.NewBasic(t)
			count := deletionCountGen.Draw(t, "count")
			checkRolls := rapid.SliceOfN(rapid.IntRange(0, 15), count, count).Draw(t, "check_rolls")

			m := NewOrderedMap[int, int]()
			assertOrderedMapWhitebox(h, &m, "empty map")
			for i := range count {
				m.InsertOrReplace(i, i*10)
			}
			assertOrderedMapWhitebox(h, &m, "after inserts")

			for i := range count {
				beforeUsedSlots := m.usedSlots()
				beforeChunkCount := len(m.chunks)
				beforeChunkCap := cap(m.chunks)
				old, ok := m.Delete(i).Get()
				h.Assertf(ok, "Delete(%d) should find inserted key", i)
				h.Assertf(old == i*10, "Delete(%d) = %d, want %d", i, old, i*10)
				h.Assertf(m.usedSlots() <= beforeUsedSlots,
					"usedSlots increased after deletion: %d -> %d", beforeUsedSlots, m.usedSlots())
				h.Assertf(len(m.chunks) <= beforeChunkCount,
					"chunk count increased after deletion: %d -> %d", beforeChunkCount, len(m.chunks))
				h.Assertf(cap(m.chunks) <= beforeChunkCap,
					"chunk capacity increased after deletion: %d -> %d", beforeChunkCap, cap(m.chunks))
				if checkRolls[i] == 0 {
					assertOrderedMapWhitebox(h, &m, "after delete")
				}
			}
			assertOrderedMapWhitebox(h, &m, "after all deletes")
			assertOrderedMapHasEmptyShape(h, &m)
		})
	})
}

func assertOrderedMapHasEmptyShape(h check.BasicHarness, m *OrderedMap[int, int]) {
	empty := NewOrderedMap[int, int]()
	h.Assertf(len(m.chunks) == len(empty.chunks), "final chunk count = %d, want %d", len(m.chunks), len(empty.chunks))
	h.Assertf(cap(m.chunks) == cap(empty.chunks), "final chunk capacity = %d, want %d", cap(m.chunks), cap(empty.chunks))
	h.Assertf(m.positions == nil, "final positions should be nil")
	h.Assertf(m.nextOffset == empty.nextOffset, "final nextOffset = %d, want %d", m.nextOffset, empty.nextOffset)
}

func assertOrderedMapWhitebox(h check.BasicHarness, m *OrderedMap[int, int], what string) {
	usedSlots := m.usedSlots()
	h.Assertf(usedSlots == 0 || 4*m.Len() > usedSlots,
		"%s: live count = %d, usedSlots = %d", what, m.Len(), usedSlots)
	h.Assertf(0 <= m.nextOffset && m.nextOffset <= orderedMapChunkSize,
		"%s: nextOffset = %d, want in [0, %d]", what, m.nextOffset, orderedMapChunkSize)
	h.Assertf(m.Len() <= m.usedSlots(), "%s: live count %d exceeds usedSlots %d", what, m.Len(), m.usedSlots())
	h.Assertf(m.usedSlots() <= len(m.chunks)*orderedMapChunkSize,
		"%s: usedSlots %d exceeds chunk capacity %d", what, m.usedSlots(), len(m.chunks)*orderedMapChunkSize)
	for key, pos := range m.positions {
		chunkIndex, offset := pos.split()
		h.Assertf(0 <= chunkIndex && chunkIndex < len(m.chunks),
			"%s: position for key %d has chunk %d outside [0, %d)", what, key, chunkIndex, len(m.chunks))
		h.Assertf(0 <= offset && offset < orderedMapChunkSize,
			"%s: position for key %d has offset %d outside [0, %d)", what, key, offset, orderedMapChunkSize)
		chunk := m.chunks[chunkIndex]
		h.Assertf(chunk.present&orderedMapBit(offset) != 0,
			"%s: position for key %d points at absent slot", what, key)
		h.Assertf(chunk.keys[offset] == key,
			"%s: position for key %d points at key %d", what, key, chunk.keys[offset])
	}
}
