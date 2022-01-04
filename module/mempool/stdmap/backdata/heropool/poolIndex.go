package heropool

// poolIndex represents a slice-based linked list pointer. Instead of pointing
// to a memory address, this pointer points to a slice index.
//
// Note: an "undefined" (i.e., nil) notion for this poolIndex corresponds to the
// value of uint32(0). Hence, legit "pointer" values start from uint32(1).
// poolIndex also furnished with methods to convert a "pointer" value to
// a slice index, and vice versa.
type poolIndex struct {
	pointerValue uint32
}

// isUndefined returns true if this pointer is set to zero. An undefined
// slice-based pointer is equivalent to a nil address-based one.
func (d poolIndex) isUndefined() bool {
	return d.pointerValue == uint32(0)
}

// setUndefined sets sliced-based pointer to its undefined (i.e., nil equivalent) value.
func (d *poolIndex) setUndefined() {
	d.pointerValue = uint32(0)
}

// sliceIndex returns the slice-index equivalent of the pointer.
func (d poolIndex) sliceIndex() EIndex {
	return EIndex(d.pointerValue) - 1
}

// setPointer converts the input slice-based index into a slice-based pointer and
// sets the underlying pointer.
func (d *poolIndex) setPointer(sliceIndex EIndex) {
	d.pointerValue = uint32(sliceIndex + 1)
}
