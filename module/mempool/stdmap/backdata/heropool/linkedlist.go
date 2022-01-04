package heropool

// link represents a slice-based doubly linked-list node that
// consists of a next and previous poolIndex.
type link struct {
	next poolIndex
	prev poolIndex
}

// state represents a doubly linked-list by its head and tail pool indices.
type state struct {
	head poolIndex
	tail poolIndex
}

func newState() *state {
	return &state{
		head: poolIndex{index: 0},
		tail: poolIndex{index: 0},
	}
}
