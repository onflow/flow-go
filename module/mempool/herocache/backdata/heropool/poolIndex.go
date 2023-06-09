package heropool

// poolIndex represents a slice-based linked list pointer. Instead of pointing
// to a memory address, this pointer points to a slice index.
type poolIndex struct {
	index EIndex
}

// getSliceIndex returns the slice-index equivalent of the poolIndex.
func (p poolIndex) getSliceIndex() EIndex {
	return EIndex(p.index)
}
