// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package collections

import (
	"iter"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/op"
	"code.kibou.tools/base/core/option"
)

// OrderedMap is a mutable map that preserves insertion order of live keys,
// allowing deterministic iteration and in-place deletion.
//
// Implementation: Currently based on chunks of fixed-sized arrays
// of keys, values and some storage bits. Deletion leaves tombstones,
// and the map compacts itself once live entries are no more than
// 25% of the used slots.
//
// A single OrderedMap is limited to at most 2^48 live entries
// on 64-bit platforms.
type OrderedMap[K comparable, V any] struct {
	// May be nil when the map is empty.
	chunks []orderedMapChunk[K, V]
	// May be nil when the map is empty.
	positions map[K]position
	// Zero when chunks is nil; otherwise the next insertion offset in the last chunk.
	nextOffset int
}

type orderedMapChunk[K comparable, V any] struct {
	keys    [orderedMapChunkSize]K
	values  [orderedMapChunkSize]V
	present uint8
}

const (
	// Chosen somewhat arbitrarily: small enough to keep iteration
	// cache-friendly, but large enough to amortize the per-chunk
	// presence bitmap. It may make sense to tune this later.
	orderedMapChunkSize = 4
	// Conservative limit so we can repurpose bits in packed positions
	// later without making the position type bigger.
	orderedMapMaxEntries = uint64(1) << 48
	orderedMapMaxChunks  = orderedMapMaxEntries / orderedMapChunkSize
)

// NewOrderedMap returns an empty ordered map.
func NewOrderedMap[K comparable, V any]() OrderedMap[K, V] {
	return OrderedMap[K, V]{
		chunks:     nil,
		positions:  nil,
		nextOffset: 0,
	}
}

// Lookup returns the value for key, if present.
//
// Expected time: Θ(1).
func (m *OrderedMap[K, V]) Lookup(key K) option.Option[V] {
	pos, ok := m.positions[key]
	if !ok {
		return option.None[V]()
	}
	chunkIndex, offset := pos.split()
	return option.Some(m.chunks[chunkIndex].values[offset])
}

// Len returns the number of live entries.
func (m *OrderedMap[K, V]) Len() int {
	return len(m.positions)
}

// Keys returns the live keys in insertion order.
//
// Creating the iterator is Θ(1). Exhausting it is Θ(|m|).
func (m *OrderedMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for _, chunk := range m.chunks {
			for offset := range orderedMapChunkSize {
				if chunk.present&orderedMapBit(offset) == 0 {
					continue
				}
				if !yield(chunk.keys[offset]) {
					return
				}
			}
		}
	}
}

// InsertOrKeep inserts the value if the key is absent.
//
// Expected time: Θ(1).
func (m *OrderedMap[K, V]) InsertOrKeep(key K, value V) op.InsertResult {
	if _, ok := m.positions[key]; ok {
		return op.KeptOld
	}
	m.insertNew(key, value)
	return op.InsertedNew
}

// InsertOrReplace inserts or replaces the value, returning the old value if
// one existed.
//
// Expected time: Θ(1).
func (m *OrderedMap[K, V]) InsertOrReplace(key K, value V) option.Option[V] {
	pos, ok := m.positions[key]
	if !ok {
		m.insertNew(key, value)
		return option.None[V]()
	}
	chunkIndex, offset := pos.split()
	old := m.chunks[chunkIndex].values[offset]
	m.chunks[chunkIndex].values[offset] = value
	return option.Some(old)
}

// Delete removes key from the map and returns its old value, if present.
//
// Expected amortized time: Θ(1).
func (m *OrderedMap[K, V]) Delete(key K) option.Option[V] {
	pos, ok := m.positions[key]
	if !ok {
		return option.None[V]()
	}
	chunkIndex, offset := pos.split()
	chunk := &m.chunks[chunkIndex]
	old := chunk.values[offset]
	delete(m.positions, key)
	var zeroKey K
	var zeroValue V
	chunk.keys[offset] = zeroKey
	chunk.values[offset] = zeroValue
	chunk.present &^= orderedMapBit(offset)
	m.compactIfNeeded()
	return option.Some(old)
}

func (m *OrderedMap[K, V]) insertNew(key K, value V) {
	if m.positions == nil {
		m.positions = map[K]position{}
	}
	if len(m.chunks) == 0 || m.nextOffset == orderedMapChunkSize {
		m.chunks = append(m.chunks, orderedMapChunk[K, V]{
			keys:    [orderedMapChunkSize]K{},
			values:  [orderedMapChunkSize]V{},
			present: 0,
		})
		m.nextOffset = 0
	}
	chunkIndex := len(m.chunks) - 1
	offset := m.nextOffset
	m.chunks[chunkIndex].keys[offset] = key
	m.chunks[chunkIndex].values[offset] = value
	m.chunks[chunkIndex].present |= orderedMapBit(offset)
	m.positions[key] = newPosition(chunkIndex, offset)
	m.nextOffset++
}

func (m *OrderedMap[K, V]) compactIfNeeded() {
	usedSlots := m.usedSlots()
	if usedSlots == 0 || 4*m.Len() > usedSlots {
		return
	}
	compacted := NewOrderedMap[K, V]()
	for _, chunk := range m.chunks {
		for offset := range orderedMapChunkSize {
			if chunk.present&orderedMapBit(offset) == 0 {
				continue
			}
			compacted.insertNew(chunk.keys[offset], chunk.values[offset])
		}
	}
	*m = compacted
}

func (m *OrderedMap[K, V]) usedSlots() int {
	if len(m.chunks) == 0 {
		return 0
	}
	return (len(m.chunks)-1)*orderedMapChunkSize + m.nextOffset
}

type position int

func newPosition(chunk int, offset int) position {
	if chunk < 0 || orderedMapMaxChunks <= uint64(chunk) || offset < 0 || orderedMapChunkSize <= offset {
		assert.Invariantf(false, "ordered map position outside supported range: chunk=%d offset=%d max_entries=%d",
			chunk, offset, orderedMapMaxEntries)
	}
	return position(chunk<<2 | offset)
}

func (idx position) split() (chunk int, offset int) {
	pos := int(idx)
	return pos >> 2, pos & (orderedMapChunkSize - 1)
}

func orderedMapBit(offset int) uint8 {
	return 1 << uint(offset)
}
