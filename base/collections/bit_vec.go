// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package collections

import (
	"math/bits"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/option"
)

// BitVec is a compact fixed-length vector of bits.
//
// The implementation is currently focused on simplicity,
// with just a simple backing uint64 array.
type BitVec struct {
	// Bit i is stored in blocks[i >> 6] at bit position i & 63.
	//
	//   Indexes:  63 ...  2  1  0 | 127 ... 66 65 64 | ...
	//   blocks:  [      blocks[0] ] [      blocks[1] ] ...
	//
	// Within each uint64, lower BitVec indexes use less-significant bits.
	blocks []uint64
	len    int
}

// NewBitVec returns a BitVec with n bits, all initially unset.
//
// Pre-condition: n must be non-negative.
func NewBitVec(n int) BitVec {
	if n < 0 {
		assert.Preconditionf(false, "BitVec length must be >= 0, but got: %d", n)
	}
	return BitVec{blocks: make([]uint64, blocksForBits(n)), len: n}
}

// Len returns the number of bits in b.
func (b *BitVec) Len() int { return b.len }

// Get returns whether bit i is set.
//
// Pre-condition: i ∈ [0, b.Len()).
func (b *BitVec) Get(i int) bool {
	b.checkIndex(i)
	return b.blocks[blockIndex(i)]&bitMask(i) != 0
}

// Set sets bit i.
//
// Pre-condition: i ∈ [0, b.Len()).
func (b *BitVec) Set(i int) {
	b.checkIndex(i)
	b.blocks[blockIndex(i)] |= bitMask(i)
}

// Clear clears bit i.
//
// Pre-condition: i ∈ [0, b.Len()).
func (b *BitVec) Clear(i int) {
	b.checkIndex(i)
	b.blocks[blockIndex(i)] &^= bitMask(i)
}

// FindAtOrBefore returns the greatest set bit j such that j <= i, if any.
//
// Pre-condition: i ∈ [0, b.Len()).
//
// Post-condition: If the return value is Some(j), then j ∈ [0, b.Len()).
func (b *BitVec) FindAtOrBefore(i int) option.Option[int] {
	b.checkIndex(i)
	startBlock := blockIndex(i)
	for oi := startBlock; 0 <= oi; oi-- {
		blockBits := b.blocks[oi]
		if oi == startBlock { // TODO: Does unrolling once help?
			blockBits &= lowBitsMask(uint8(i&63) + 1)
		}
		if blockBits != 0 {
			idx := oi*64 + 63 - bits.LeadingZeros64(blockBits)
			return option.Some(idx)
		}
	}
	return option.None[int]()
}

// FindAtOrAfter returns the least set bit j such that i <= j, if any.
//
// Pre-condition: i ∈ [0, b.Len()).
//
// Post-condition: If the return value is Some(j), then j ∈ [0, b.Len()).
func (b *BitVec) FindAtOrAfter(i int) option.Option[int] {
	b.checkIndex(i)
	startBlock := blockIndex(i)
	for oi := startBlock; oi < len(b.blocks); oi++ {
		blockBits := b.blocks[oi]
		if oi == startBlock { // TODO: Does unrolling once help?
			blockBits &^= lowBitsMask(uint8(i & 63))
		}
		if blockBits != 0 {
			idx := oi*64 + bits.TrailingZeros64(blockBits)
			if b.len <= idx {
				assert.Invariantf(false, "unused bit %d is set in BitVec of length %d", idx, b.len)
			}
			return option.Some(idx)
		}
	}
	return option.None[int]()
}

// checkIndex asserts that i ∈ [0, b.Len()).
func (b *BitVec) checkIndex(i int) {
	if i < 0 || b.len <= i {
		assert.Preconditionf(false, "bit index %d out of range [0, %d)", i, b.len)
	}
}

// blocksForBits returns the number of blocks needed for
// storing n bits.
func blocksForBits(n int) int {
	blocks := n >> 6
	if n&63 != 0 {
		blocks++
	}
	return blocks
}

// blockIndex returns the index in the block slice that
// needs to be accessed for getting the data for bit i.
//
// Pre-condition: i >= 0
func blockIndex(i int) int {
	return i >> 6
}

// bitMask returns a mask for getting the bit corresponding
// to the bit i from the block in a BitVec.
//
// Pre-condition: i >= 0
func bitMask(i int) uint64 {
	return uint64(1) << uint(i&63)
}

// lowBitsMask returns a word whose lowest n bits are set and whose remaining
// bits are unset. For example, lowBitsMask(3) == 0b111.
//
// Pre-condition: n ∈ [0, 64].
func lowBitsMask(n uint8) uint64 {
	// If n == 64, the shift will lead to 0, so -1 will still
	// give the correct behavior.
	return (uint64(1) << n) - 1
}
