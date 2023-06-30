package heropool

// link represents a slice-based doubly linked-list node that
// consists of a next and previous poolIndex.
// if a link doesn't belong to any state it's next and prev should hold InvalidIndex.
type link struct {
	next EIndex
	prev EIndex
}

// state represents a doubly linked-list by its head and tail pool indices.
// If state has 0 size, its tail's and head's prev and next are treated as invalid and should hold InvalidIndex values.
type state struct {
	head EIndex
	tail EIndex
	size uint32
}
