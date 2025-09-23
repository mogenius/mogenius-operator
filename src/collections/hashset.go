package collections

import (
	"cmp"
	"iter"
	"maps"
	"slices"
)

type HashSet[T cmp.Ordered] struct {
	data map[T]struct{}
}

func NewHashSet[T cmp.Ordered]() HashSet[T] {
	d := HashSet[T]{}
	d.data = make(map[T]struct{})
	return d
}

func (d *HashSet[T]) Contains(s T) bool {
	_, ok := d.data[s]
	return ok
}

// returns whether the value had to be added
func (d *HashSet[T]) Insert(s T) bool {
	if d.Contains(s) {
		return false
	}

	d.data[s] = struct{}{}
	return true
}

// returns whether the values was present
func (d *HashSet[T]) Remove(s T) bool {
	if d.Contains(s) {
		delete(d.data, s)
		return true
	}

	return false
}

func (d *HashSet[T]) Len() int {
	return len(d.data)
}

func (d *HashSet[T]) IsEmpty() bool {
	return d.Len() == 0
}

func (d *HashSet[T]) Clear() {
	d.data = make(map[T]struct{})
}

func (d *HashSet[T]) Iter() iter.Seq[T] {
	return maps.Keys(d.data)
}

func (d *HashSet[T]) Slice() []T {
	return slices.Collect(d.Iter())
}
