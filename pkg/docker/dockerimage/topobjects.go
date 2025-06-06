package dockerimage

import (
	"container/heap"
)

// TopObjects is a slice of ObjectMetadata pointers that implements the heap.Interface
// for maintaining a priority queue of objects, typically sorted by size.
type TopObjects []*ObjectMetadata

// NewTopObjects creates and returns a new, empty TopObjects slice with a specified capacity.
func NewTopObjects(n int) TopObjects {
	if n < 1 {
		n = 1
	}
	n++
	return make(TopObjects, 0, n)
}

// Len returns the number of elements in the TopObjects slice.
func (to TopObjects) Len() int { return len(to) }

// Less compares two elements in the slice for sorting.
// Nil elements are considered smaller than non-nil elements.
// Two nil elements are considered equal.
func (to TopObjects) Less(i, j int) bool {
	// Handle cases where either element is nil
	if to[i] == nil && to[j] == nil {
		return false // Equal, so not less than
	}
	if to[i] == nil {
		return true // nil is considered smaller than non-nil
	}
	if to[j] == nil {
		return false // Non-nil is considered larger than nil
	}
	// Both elements are non-nil, compare their sizes
	return to[i].Size < to[j].Size
}

// Swap swaps the elements with indexes i and j.
// It performs bounds checking to prevent panics.
func (to TopObjects) Swap(i, j int) {
	if i < 0 || i >= len(to) || j < 0 || j >= len(to) {
		return // Out of bounds, no-op
	}
	to[i], to[j] = to[j], to[i]
}

// Push adds an element to the heap. It handles nil values safely and only adds
// valid *ObjectMetadata pointers to the slice.
func (to *TopObjects) Push(x interface{}) {
	if x == nil {
		return
	}
	item, ok := x.(*ObjectMetadata)
	if !ok || item == nil {
		return
	}
	*to = append(*to, item)
}

// Pop removes and returns the smallest element from the heap.
// The result is the element that would be returned by Pop() from the heap package.
func (to *TopObjects) Pop() interface{} {
	old := *to
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*to = old[0 : n-1]
	return item
}

// List returns a sorted slice of ObjectMetadata, with the largest elements first.
// It creates a copy of the underlying data to preserve the original heap.
// Returns nil if the receiver is nil.
func (to TopObjects) List() []*ObjectMetadata {
	if to == nil {
		return nil
	}

	tmp := make(TopObjects, len(to))
	copy(tmp, to)
	heap.Init(&tmp)

	list := make([]*ObjectMetadata, 0, len(to))
	for tmp.Len() > 0 {
		item := heap.Pop(&tmp).(*ObjectMetadata)
		if item != nil {
			list = append([]*ObjectMetadata{item}, list...) // prepend to maintain order
		}
	}

	return list
}
