// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package pair

type KeyValue[K, V any] struct {
	Key   K
	Value V
}

func NewKeyValue[K, V any](k K, v V) KeyValue[K, V] {
	return KeyValue[K, V]{k, v}
}

type Pair[A, B any] struct {
	First  A
	Second B
}

func NewPair[A, B any](first A, second B) Pair[A, B] {
	return Pair[A, B]{first, second}
}

func (p Pair[A, B]) Unpack() (A, B) {
	return p.First, p.Second
}
