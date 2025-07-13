package dockerimage

import (
	"container/heap"
)

type TopObjects []*ObjectMetadata

func NewTopObjects(n int) TopObjects {
	if n < 1 {
		n = 1
	}
	n++
	return make(TopObjects, 0, n)
}

func (to TopObjects) Len() int { return len(to) }

func (to TopObjects) Less(i, j int) bool {
	if to[i] == nil && to[j] != nil {
		return true
	}
	if to[i] != nil && to[j] == nil {
		return false
	}
	if to[i] == nil && to[j] == nil {
		return false
	}

	return to[i].Size < to[j].Size
}

func (to TopObjects) Swap(i, j int) {
	to[i], to[j] = to[j], to[i]
}

func (to *TopObjects) Push(x interface{}) {
	item := x.(*ObjectMetadata)
	if item == nil {
		return
	}
	*to = append(*to, item)
}

func (to *TopObjects) Pop() interface{} {
	old := *to
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*to = old[0 : n-1]
	return item
}

func (to TopObjects) List() []*ObjectMetadata {
	list := []*ObjectMetadata{}
	// Create a copy of the heap to avoid modifying the original heap
	h := make(TopObjects, len(to))
	copy(h, to)
	heap.Init(&h)

	for len(h) > 0 {
		item := heap.Pop(&h).(*ObjectMetadata)
		if item == nil {
			continue
		}
		list = append([]*ObjectMetadata{item}, list...)
	}

	return list
}
