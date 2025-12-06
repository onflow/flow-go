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
// Internally, it operates directly on the Vertex Containers.
// It has one-element look ahead for skipping empty vertex containers.
//
// ATTENTION: Not Concurrency Safe!
//
// This vertex iterator does NOT COPY the provided list of vertices for
// efficiency reasons. For APPEND_ONLY `VertexList`s, the `VertexIterator`
// can be wrapped into a VertexIteratorConcurrencySafe to make it concurrency
// safe. By design, the ResultForest guarantees this. Hence, construction
// of these vertex iterators is private to the `forest` package.
type VertexIterator struct {
	// CAUTION: to support concurrency-safe iterators, the `VertexIterator` *must* maintain its own slice descriptor.
	// This is the default in Golang, as slices are typically passed by value, since only the slice descriptor (see
	// https://go.dev/blog/slices-intro for details) is copied, but not the backing array. While very uncommon in go,
	// we emphasize that a hypothetical change to `data *VertexList` (using pointer to slice) would break our wrapper
	// `VertexIteratorConcurrencySafe`.
	// Context:
	// • `VertexIterator`s are instantiated by calling LevelledForest.GetChildren or .GetVerticesAtLevel` for
	//    example. In both cases, the provided `VertexList` is append-only in the levelled forest. So we assume a
	//    LevelledForest instance which is synchronized for concurrent access by higher-level business logic. Then,
	//    a `VertexIterator` many be iterated on, while concurrently elements are added to the forest.
	// • Note that `data` is intrinsically safe for concurrent access, as long as no elements are modified inplace.
	//   In other words, append-only usage patterns are intrinsically safe for concurrent access. Eventually, the forest
	//   may exceed the current slice's capacity, at which point a new array is allocated by forest, while we maintain
	//   a reference to the older array here. Essentially, we maintain a snapshot of the slice at the point we received
	//   it, since our `data` field below also contains a local copy of the slice's length at the point we received it.
	data VertexList // tldr; assumed safe for concurrent access, as forest operates append-only

	// not protected for concurrent access:

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

// newVertexIterator instantiates an iterator. Essentially it operates on a snapshot of the slice.
// Even if the Levelled Forest makes additions to the input slice, we maintain our own notion of
// length and backing slice.
// CAUTION:
//   - we NOT COPY the list's containers for efficiency.
//   - Package-private, as usage must be limited to APPEND-ONLY `VertexList`
//     Without append-only guarantees, we would break the `VertexIteratorConcurrencySafe`
//     and generally a lot of conceptual challenges arise for iteration in concurrent
//     environments. We easily avoid the complexity by restricting the usage to the
//     levelled forest, which by design operates append-only (and eventually garbage collected
//     on pruning.
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

// IsInvalidVertexError returns ture if and only if the input error is a (wrapped) InvalidVertexError.
func IsInvalidVertexError(err error) bool {
	var target InvalidVertexError
	return errors.As(err, &target)
}

// NewInvalidVertexErrorf instantiates an [InvalidVertexError]. The
// inputs `msg` and `args` follow the pattern of [fmt.Errorf].
func NewInvalidVertexErrorf(vertex Vertex, msg string, args ...interface{}) InvalidVertexError {
	return InvalidVertexError{
		Vertex: vertex,
		msg:    fmt.Sprintf(msg, args...),
	}
}
