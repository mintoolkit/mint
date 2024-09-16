package acounter

import (
	"sync/atomic"
)

type Type struct {
	val uint64
}

func (ref *Type) Value() uint64 {
	return atomic.LoadUint64(&ref.val)
}

func (ref *Type) Inc() uint64 {
	return ref.Add(1)
}

func (ref *Type) Add(val uint64) uint64 {
	return atomic.AddUint64(&ref.val, val)
}

func (ref *Type) Clear() {
	atomic.StoreUint64(&ref.val, 0)
}
