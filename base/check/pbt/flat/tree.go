// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package flat provides flat data structures.
package flat

import (
	"math"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/core/option"
)

const rootParentIndex int32 = -1

// TreeID identifies a node in a [Tree].
type TreeID struct {
	index int32
}

// Index returns the stable breadth-first index for this tree node.
func (id TreeID) Index() int {
	return int(id.index)
}

func (id TreeID) Compare(other TreeID) int {
	return int(id.index) - int(other.index)
}

// Tree is a rooted tree stored in breadth-first order.
type Tree[T any] struct {
	// Always non-nil. Contains at least one node.
	nodes []node[T]
}

type node[T any] struct {
	value T
	// rootParentIndex iff this is the root.
	parentIndex int32
	// Immediate children are stored in nodes[childStart:childEnd].
	childStart int32
	childEnd   int32
}

// UnfoldTree constructs a rooted tree from a root seed.
//
// For each seed, f returns the corresponding node value and may call yieldChild
// zero or more times to append child seeds. Nodes are stored in breadth-first
// order, and a node's immediate children are stored contiguously.
//
// The yieldChild callback must only be called during the invocation of f that
// received it. Calling it after f returns is a pre-condition violation.
//
// Pre-conditions:
//  1. f is non-nil.
//  2. f eventually stops yielding child seeds.
//  3. the total number of yielded nodes is <= math.MaxInt32.
func UnfoldTree[Seed, T any](root Seed, f func(seed Seed, yieldChild func(Seed)) T) Tree[T] {
	assert.Precondition(f != nil, "unfold function must be non-nil")

	var zero T
	nodes := []node[T]{{
		value:       zero,
		parentIndex: rootParentIndex,
		childStart:  0,
		childEnd:    0,
	}}
	seeds := []Seed{root}
	for index := 0; index < len(nodes); index++ {
		childStart := len(nodes)
		active := true
		value := f(seeds[index], func(child Seed) {
			assert.Precondition(active, "yieldChild called after unfold callback returned")
			assert.Precondition(len(nodes) < math.MaxInt32, "tree cannot contain more than math.MaxInt32 nodes")
			nodes = append(nodes, node[T]{
				value:       zero,
				parentIndex: int32(index),
				childStart:  0,
				childEnd:    0,
			})
			seeds = append(seeds, child)
		})
		active = false
		nodes[index].value = value
		nodes[index].childStart = int32(childStart)
		nodes[index].childEnd = int32(len(nodes))
	}
	return Tree[T]{nodes: nodes}
}

func (t Tree[T]) NodeCount() int {
	return len(t.nodes)
}

// ID returns the [TreeID] with index.
//
// Pre-condition: index is in [0, NodeCount()).
func (t Tree[T]) ID(index int) TreeID {
	t.checkIndex(index, "node index")
	return TreeID{index: int32(index)}
}

// Value returns the value stored at id.
//
// Pre-condition: id belongs to this tree.
func (t Tree[T]) Value(id TreeID) T {
	t.checkID(id, "node ID")
	return t.nodes[id.index].value
}

// ParentID returns the parent ID for id, or None for the root.
//
// Pre-condition: id belongs to this tree.
func (t Tree[T]) ParentID(id TreeID) option.Option[TreeID] {
	t.checkID(id, "node ID")
	parentIndex := t.nodes[id.index].parentIndex
	if parentIndex == rootParentIndex {
		return option.None[TreeID]()
	}
	return option.Some(TreeID{index: parentIndex})
}

// ChildIDs returns the immediate children of id.
//
// Pre-condition: id belongs to this tree.
func (t Tree[T]) ChildIDs(id TreeID) []TreeID {
	t.checkID(id, "node ID")
	node := t.nodes[id.index]
	children := make([]TreeID, 0, node.childEnd-node.childStart)
	for childIndex := node.childStart; childIndex < node.childEnd; childIndex++ {
		children = append(children, TreeID{index: childIndex})
	}
	return children
}

// MarkSubtrees returns a bool slice of length NodeCount marking every node in
// the subtree of any ID in sortedIDs.
//
// Pre-conditions:
//  1. every ID in sortedIDs belongs to this tree.
//  2. sortedIDs is sorted by [TreeID.Compare].
func (t Tree[T]) MarkSubtrees(sortedIDs []TreeID) []bool {
	marked := make([]bool, t.NodeCount())
	for _, id := range sortedIDs {
		t.markSubtree(id, marked)
	}
	return marked
}

func (t Tree[T]) markSubtree(root TreeID, marked []bool) {
	t.checkID(root, "node ID")
	if marked[root.Index()] {
		return
	}
	marked[root.Index()] = true
	for _, childID := range t.ChildIDs(root) {
		t.markSubtree(childID, marked)
	}
}

func (t Tree[T]) checkID(id TreeID, what string) {
	t.checkIndex(id.Index(), what)
}

func (t Tree[T]) checkIndex(index int, what string) {
	assert.Preconditionf(0 <= index && index < t.NodeCount(), "%s %d out of range [0, %d)", what, index, t.NodeCount())
}
