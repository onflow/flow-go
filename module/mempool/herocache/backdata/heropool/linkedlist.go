package heropool

// link represents a slice-based doubly linked-list node that
// consists of a next and previous poolIndex.
// if a link doesn't belong to any state it's next and prev may have any values,
// but those are treated as invalid and should not be used.
type link struct {
	next EIndex
	prev EIndex
}

// state represents a doubly linked-list by its head and tail pool indices.
// If state has 0 size, its tail's and head's prev and next are treated as invalid.
// Moreover head's prev and tails next are always treated as invalid and may hold any values.
type state struct {
	head EIndex
	tail EIndex
	size uint32
}
