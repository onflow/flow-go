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

// Adds entity to the tail of the list or creates first element
func (s *state) appendEntity(p *Pool, entityIndex EIndex) {
	if s.size == 0 {
		s.head.index = entityIndex
		s.tail.index = entityIndex
		s.size = 1
		return
	}
	p.connect(s.tail, entityIndex)
	s.size++
	s.tail.index = entityIndex
}

// Removes an entity from the list
func (s *state) removeEntity(p *Pool, entityIndex EIndex) {
	if s.size == 0 {
		panic("Removing entity from an empty list")
	}
	if s.size == 1 {
		s.size--
		return
	}
	node := p.poolEntities[entityIndex].node

	if entityIndex != s.head.getSliceIndex() && entityIndex != s.tail.getSliceIndex() {
		// links next and prev elements for non-head and non-tail element
		p.connect(node.prev, node.next.getSliceIndex())
	}

	if entityIndex == s.head.getSliceIndex() {
		// moves head forward
		s.head = node.next
	}

	if entityIndex == s.tail.getSliceIndex() {
		// moves tail backwards
		s.tail = node.prev
	}
	s.size--
}
