package atomic

import (
	"go.uber.org/atomic"
)

// storedVal is the type stored in the atomic variable. It includes an extra
// field `notZero` which is always set to true, to allow storing zero values
// for the stored value `val`.
type storedVal[E any] struct {
	val     E
	notZero bool // always true
}

func newStoredValue[E any](val E) storedVal[E] {
	return storedVal[E]{val: val, notZero: true}
}

// Value is a wrapper around sync/atomic.Value providing type safety with generic parameterization
// and the ability to store the zero value for a type.
type Value[E any] struct {
	val *atomic.Value
}

func NewValue[E any]() Value[E] {
	return Value[E]{
		val: &atomic.Value{},
	}
}

// Set atomically stores the given value.
func (c Value[E]) Set(e E) {
	c.val.Store(newStoredValue(e))
}

// Get returns the stored value, if any, and whether any value was stored.
func (c Value[E]) Get() (E, bool) {
	stored := c.val.Load()
	// sync/atomic.Value returns nil if no value has ever been stored, or if the zero value
	// for a type has been stored. We only ever store non-zero instances of storedValue,
	// so this case only happens if no value has ever been stored.
	if stored == nil {
		var ret E
		return ret, false
	}
	return stored.(storedVal[E]).val, true
}
