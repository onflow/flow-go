package heropool

// poolIndex represents a slice-based linked list pointer. Instead of pointing
// to a memory address, this pointer points to a slice index.
//
// Note: an "undefined" (i.e., nil) notion for this poolIndex corresponds to the
// value of uint32(0). Hence, legit "index" poolEntities start from uint32(1).
// poolIndex also furnished with methods to convert a "poolIndex" value to
// a slice index, and vice versa.
type poolIndex struct {
	index uint32
}

// isUndefined returns true if this poolIndex is set to zero. An undefined
// poolIndex is equivalent to a nil address-based one.
func (p poolIndex) isUndefined() bool {
	return p.index == uint32(0)
}

// setUndefined sets poolIndex to its undefined (i.e., nil equivalent) value.
func (p *poolIndex) setUndefined() {
	p.index = uint32(0)
}

// getSliceIndex returns the slice-index equivalent of the poolIndex.
func (p poolIndex) getSliceIndex() EIndex {
	return EIndex(p.index) - 1
}

// setPoolIndex converts the input slice-based index into a pool index and
// sets the underlying poolIndex.
func (p *poolIndex) setPoolIndex(sliceIndex EIndex) {
	p.index = uint32(sliceIndex + 1)
}
