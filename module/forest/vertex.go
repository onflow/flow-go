package forest

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type Vertex interface {
	// VertexID returns the vertex's ID (in most cases its hash)
	VertexID() flow.Identifier
	// Level returns the vertex's level
	Level() uint64
	// Parent returns the parent's (level, ID)
	Parent() (flow.Identifier, uint64)
}

// VertexToString returns a string representation of the vertex.
func VertexToString(v Vertex) string {
	parentID, parentLevel := v.Parent()
	return fmt.Sprintf("<id=%x level=%d parent_id=%d parent_level=%d>", v.VertexID(), v.Level(), parentID, parentLevel)
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

// InvalidVertexError indicates that a proposed vertex is invalid for insertion to the forest.
type InvalidVertexError struct {
	// Vertex is the invalid vertex
	Vertex Vertex
	// msg provides additional context
	msg string
}

func (err InvalidVertexError) Error() string {
	return fmt.Sprintf("invalid vertex %s: %s", VertexToString(err.Vertex), err.msg)
}

func IsInvalidVertexError(err error) bool {
	var target InvalidVertexError
	return errors.As(err, &target)
}

func NewInvalidVertexErrorf(vertex Vertex, msg string, args ...interface{}) InvalidVertexError {
	return InvalidVertexError{
		Vertex: vertex,
		msg:    fmt.Sprintf(msg, args...),
	}
}
