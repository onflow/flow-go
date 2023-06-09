package heropool

// link represents a slice-based doubly linked-list node that
// consists of a next and previous poolIndex.
// if a link doesn't belong to any state it's next and prev may have any values,
// but those are treated as invalid and should not be used.
type link struct {
	// As we dont have from now on an invalid index we need either to make next/prev point to itself
	// in order to show that its invalid
	// Other option is to say that  if a link is a tail or head then respectively its prev or next should not be used
	// if tail and head concide then neither of them is to be used
	// Both solutions defacto would reintroduce isDefined, we would need to check if next != current Index
	// of if this is head or tail ... of one of the lists. For this reason I am not sure that this idea of removing 0 as undefined wont backfire

	// lets start with head and tail check
	next poolIndex
	prev poolIndex
}

// state represents a doubly linked-list by its head and tail pool indices.
// If satte has 0 size, its tail's and head's prev and next are treated as invalid.
// Moreover head's prev and tails next are always treated as invalid and may hold any values.
type state struct {
	//those might now coincide rather than point to 0
	head poolIndex
	tail poolIndex
	size uint32
}

// Adds element to the tail of the list or creates first element
func (s *state) addElement(p *Pool, entity EIndex) {
	if s.size == 0 {
		s.head.index = entity
		s.tail.index = entity
		s.size = 1
		return
	}
	p.connect(s.tail, entity)
	s.size++
	s.tail.index = entity
}
