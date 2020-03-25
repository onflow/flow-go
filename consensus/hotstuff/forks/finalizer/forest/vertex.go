package forest

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Vertex interface {
	// VertexID returns the vertex's ID (in most cases its hash)
	VertexID() flow.Identifier
	// Level returns the vertex's level
	Level() uint64
	// Parent returns the returns the parents (level, ID)
	Parent() (flow.Identifier, uint64)
}

// VertexIterator is a stateful iterator for VertexList.
// Internally operates directly on the Vertex Containers
// It has one-element look ahead for skipping empty vertex containers.
type VertexIterator struct {
	data VertexList
	idx  int
	next Vertex
}

func (it *VertexIterator) preLoad() {
	for it.idx < len(it.data) {
		v := it.data[it.idx].vertex
		it.idx++
		if v != nil {
			it.next = v
			return
		}
	}
	it.next = nil
}

// NextVertex returns the next Vertex or nil if there is none
func (it *VertexIterator) NextVertex() Vertex {
	res := it.next
	it.preLoad()
	return res
}

// HasNext returns true if and only if there is a next Vertex
func (it *VertexIterator) HasNext() bool {
	return it.next != nil
}

func newVertexIterator(vertexList VertexList) VertexIterator {
	it := VertexIterator{
		data: vertexList,
	}
	it.preLoad()
	return it
}
