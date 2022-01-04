package heropool

// poolIndex represents a slice-based linked list pointer. Instead of pointing
// to a memory address, this pointer points to a slice index.
//
// Note: an "undefined" (i.e., nil) notion for this poolIndex corresponds to the
// value of uint32(0). Hence, legit "index" values start from uint32(1).
// poolIndex also furnished with methods to convert a "poolIndex" value to
// a slice index, and vice versa.
type poolIndex struct {
	index uint32
}

// isUndefined returns true if this poolIndex is set to zero. An undefined
// poolIndex is equivalent to a nil address-based one.
func (d poolIndex) isUndefined() bool {
	return d.index == uint32(0)
}

// setUndefined sets poolIndex to its undefined (i.e., nil equivalent) value.
func (d *poolIndex) setUndefined() {
	d.index = uint32(0)
}

// sliceIndex returns the slice-index equivalent of the poolIndex.
func (d poolIndex) sliceIndex() EIndex {
	return EIndex(d.index) - 1
}

// setPoolIndex converts the input slice-based index into a pool index and
// sets the underlying poolIndex.
func (d *poolIndex) setPoolIndex(sliceIndex EIndex) {
	d.index = uint32(sliceIndex + 1)
}
