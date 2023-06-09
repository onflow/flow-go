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
// If satte has 0 size, its tail's and head's prev and next are treated as invalid.
// Moreover head's prev and tails next are always treated as invalid and may hold any values.
type state struct {
	//those might now coincide rather than point to 0
	head EIndex
	tail EIndex
	size uint32
}

// adds entity to the tail of the list or creates first element
func (s *state) appendEntity(p *Pool, entityIndex EIndex) {
	if s.size == 0 {
		s.head = entityIndex
		s.tail = entityIndex
		s.size = 1
		return
	}
	p.connect(s.tail, entityIndex)
	s.size++
	s.tail = entityIndex
}

// removes an entity from the list
func (s *state) removeEntity(p *Pool, entityIndex EIndex) {
	if s.size == 0 {
		panic("Removing entity from an empty list")
	}
	if s.size == 1 {
		s.size--
		return
	}
	node := p.poolEntities[entityIndex].node

	if entityIndex != s.head && entityIndex != s.tail {
		// links next and prev elements for non-head and non-tail element
		p.connect(node.prev, node.next)
	}

	if entityIndex == s.head {
		// moves head forward
		s.head = node.next
	}

	if entityIndex == s.tail {
		// moves tail backwards
		s.tail = node.prev
	}
	s.size--
}
