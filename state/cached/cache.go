package cached

import (
	"unsafe"

	"go.uber.org/atomic"
)

// WriteOnce is a container for an entity, which can be set exactly once
// and read any number of times. It is useful for optional cached fields on
// structures which are used in concurrent environments.
// It is safe for concurrent use by multiple goroutines.
type WriteOnce[E any] struct {
	val *atomic.UnsafePointer
}

func NewWriteOnce[E any]() WriteOnce[E] {
	return WriteOnce[E]{
		val: atomic.NewUnsafePointer(nil),
	}
}

func (c WriteOnce[E]) Get() (*E, bool) {
	cached := (*E)(c.val.Load())
	if cached != nil {
		return cached, true
	}
	return nil, false
}

func (c WriteOnce[E]) Set(e *E) bool {
	if c.val.CompareAndSwap(nil, (unsafe.Pointer)(e)) {
		return true
	}
	return false
}

type Value[E any] struct {
	val *atomic.UnsafePointer
}

func NewValue[E any]() Value[E] {
	return Value[E]{
		val: atomic.NewUnsafePointer(nil),
	}
}

func (c Value[E]) Get() (*E, bool) {
	cached := (*E)(c.val.Load())
	if cached != nil {
		return cached, true
	}
	return nil, false
}

func (c Value[E]) Set(e *E) {
	c.val.Store((unsafe.Pointer)(e))
}
