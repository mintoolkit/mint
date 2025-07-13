package dockerimage

import (
	"container/heap"
	"sync"
)

type TopObjects struct {
	items []*ObjectMetadata
	mux   sync.Mutex
}

func NewTopObjects(n int) *TopObjects {
	if n < 1 {
		n = 1
	}
	n++
	return &TopObjects{
		items: make([]*ObjectMetadata, 0, n),
	}
}

func (to *TopObjects) Len() int { return len(to.items) }

func (to *TopObjects) Less(i, j int) bool {
	if to.items[i] == nil && to.items[j] != nil {
		return true
	}
	if to.items[i] != nil && to.items[j] == nil {
		return false
	}
	if to.items[i] == nil && to.items[j] == nil {
		return false
	}

	return to.items[i].Size < to.items[j].Size
}

func (to *TopObjects) Swap(i, j int) {
	to.items[i], to.items[j] = to.items[j], to.items[i]
}

func (to *TopObjects) Push(x interface{}) {
	to.mux.Lock()
	defer to.mux.Unlock()
	item := x.(*ObjectMetadata)
	if item == nil {
		return
	}
	to.items = append(to.items, item)
}

func (to *TopObjects) Pop() interface{} {
	to.mux.Lock()
	defer to.mux.Unlock()
	old := to.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	to.items = old[0 : n-1]
	return item
}

func (to *TopObjects) List() []*ObjectMetadata {
	to.mux.Lock()
	defer to.mux.Unlock()
	list := []*ObjectMetadata{}
	for to.Len() > 0 {
		item := heap.Pop(to).(*ObjectMetadata)
		if item == nil {
			continue
		}
		list = append([]*ObjectMetadata{item}, list...)
	}

	return list
}
