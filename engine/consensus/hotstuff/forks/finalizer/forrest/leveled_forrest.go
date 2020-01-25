package forrest

import (
	"bytes"
	"fmt"
)

type Vertex interface {
	// VertexID returns the vertex's ID (in most cases its hash)
	VertexID() []byte
	// Level returns the vertex's level
	Level() uint64
	// Parent returns the returns the parents (level, ID)
	Parent() ([]byte, uint64)
}

// vertexContainer holds information about a tree vertex. Internally, we distinguish between
// * FULL container: has non-nil value for vertex.
//   Used for vertices, which have been added to the tree.
// * EMPTY container: has NIL value for vertex.
//   Used for vertices, which have NOT been added to the tree, but are
//   referenced by vertices in the tree. An empty container is converted to a
//   full container when the respective vertex is added to the tree
type vertexContainer struct {
	id       []byte
	level    uint64
	children VertexList

	// the following are only set if the block is actually known
	vertex Vertex
}

type VertexList []*vertexContainer
type VertexSet map[string]*vertexContainer

// LeveledForrest contains multiple trees (which is a potentially disconnected planar graph).
// Each vertexContainer in the graph has a level (view) and a hash. A vertexContainer can only have one parent
// with strictly smaller level (view). A vertexContainer can have multiple children, all with
// strictly larger level (view).
// A LeveledForrest provides the ability to prune all vertices up to a specific level.
// A tree whose root is below the pruning threshold might decompose into multiple
// disconnected subtrees as a result of pruning.
type LeveledForrest struct {
	vertices        VertexSet
	verticesAtLevel map[uint64]VertexList
	LowestLevel     uint64
}

// NewLeveledForrest initializes a LeveledForrest
func NewLeveledForrest() *LeveledForrest {
	return &LeveledForrest{
		vertices:        make(VertexSet),
		verticesAtLevel: make(map[uint64]VertexList),
	}
}

// pruneAtView prunes all blocks up to and INCLUDING `level`
func (f *LeveledForrest) PruneAtLevel(level uint64) error {
	if level+1 < f.LowestLevel {
		return fmt.Errorf("Cannot prune tree slice up to level %d because we only save up to level %d", level, f.LowestLevel)
	}
	for l := f.LowestLevel; l <= level; l++ {
		for _, v := range f.verticesAtLevel[l] { // nil map behaves like empty map when iterating over it
			delete(f.vertices, string(v.id))
		}
		delete(f.verticesAtLevel, l)
	}
	f.LowestLevel = level + 1
	return nil
}

// HasVertex returns true iff full vertex exists
func (f *LeveledForrest) HasVertex(id []byte) bool {
	container, exists := f.vertices[string(id)]
	return exists && !f.isEmptyContainer(container)
}

// isEmptyContainer returns true iff vertexContainer container is empty, i.e. full vertex itself has not been added
func (f *LeveledForrest) isEmptyContainer(vertexContainer *vertexContainer) bool {
	return vertexContainer.vertex == nil
}


// GetVertex returns (<full vertex>, true) if the vertex with `id` and `level` was found
// (nil, false) if full vertex is unknown
func (f *LeveledForrest) GetVertex(id []byte) (Vertex, bool) {
	container, exists := f.vertices[string(id)]
	if !exists || f.isEmptyContainer(container) {
		return nil, false
	}
	return container.vertex, true
}

//// GetVertex returns (<full vertex>, true) if the vertex with `id` and `level` was found
//// (nil, false) if full vertex is unknown
//func (f *LeveledForrest) VertexOrError(id []byte, level uint64) (Vertex, error) {
//	container, exists := f.vertices[string(id)]
//	if !exists || f.isEmptyContainer(container) {
//		return nil, fmt.Errorf("missing vertex with level %d and id %s, ", level, id)
//	}
//	if container.level !=
//	return container.vertex, true
//}

// GetChildren returns a VertexIterator to iterate over the children
// An empty VertexIterator is returned, if no vertices are known whose parent is `id` , `level`
func (f *LeveledForrest) GetChildren(id []byte) VertexIterator {
	container, _ := f.vertices[string(id)]
	// if vertex does not exists, container is the default zero value for vertexContainer, which contains a nil-slice for its children
	return newVertexIterator(container.children) // VertexIterator gracefully handles nil slices
}

// GetVerticesAtLevel returns a VertexIterator to iterate over the Vertices at the specified height
func (f *LeveledForrest) GetNumberOfChildren(id []byte) int {
	container, _ := f.vertices[string(id)] // if vertex does not exists, container is the default zero value for vertexContainer, which contains a nil-slice for its children
	num := 0
	for _, child := range container.children {
		if child.vertex != nil {
			num++
		}
	}
	return num
}

// GetVerticesAtLevel returns a VertexIterator to iterate over the Vertices at the specified height
// An empty VertexIterator is returned, if no vertices are known at the specified `level`
func (f *LeveledForrest) GetVerticesAtLevel(level uint64) VertexIterator {
	return newVertexIterator(f.verticesAtLevel[level]) // go returns the zero value for a missing level. Here, a nil slice
}

// GetVerticesAtLevel returns a VertexIterator to iterate over the Vertices at the specified height
func (f *LeveledForrest) GetNumberOfVerticesAtLevel(level uint64) int {
	num := 0
	for _, container := range f.verticesAtLevel[level] {
		if container.vertex != nil {
			num++
		}
	}
	return num
}

// AddVertex adds vertex to forrest if vertex is within non-pruned levels
// Safe:
// * Gracefully handles repeated addition of same vertex (keeps first added vertex)
// * if vertex is at or below pruning level: NoOp
// * checks for inconsistencies: vertex with same id but different level will cause error
//   (instead of leaving the data structure in an inconsistent state).
func (f *LeveledForrest) AddVertex(vertex Vertex) error {
	if vertex.Level() < f.LowestLevel {
		return nil
	}

	container, err := f.getOrCreateVertexContainer(vertex.VertexID(), vertex.Level())
	if err != nil {
		return fmt.Errorf("Cannot add Vertex: %w", err)
	}
	if f.isEmptyContainer(container) { // container is empty, i.e. full vertex itself has not been added
		container.vertex = vertex
		parentContainer, err := f.getOrCreateVertexContainer(vertex.Parent())
		if err != nil {
			return fmt.Errorf("Cannot add Vertex: %w", err)
		}
		parentContainer.children = append(parentContainer.children, container) // append works on nil slices: creates slice with capacity 2
	} else { // sanity check: check that both vertices reference same parent
		p1Id, p1Level := vertex.Parent()
		p2Id, p2Level := container.vertex.Parent()
		if !bytes.Equal(p1Id, p2Id) || (p1Level != p2Level) {
			return fmt.Errorf("Encountered same vertex but with mismatching parents")
		}
	}
	return nil
}

// getOrCreateVertexContainer returns the vertexContainer if there exists one
// or creates a new vertexContainer and adds it to the internal data structures.
// It errors if a vertex with same id but different Level is already known
// (i.e. there exists an empty or full container with the same id but different level).
func (f *LeveledForrest) getOrCreateVertexContainer(id []byte, level uint64) (*vertexContainer, error) {
	container, exists := f.vertices[string(id)]
	if !exists {
		container = f.uncheckedAddEmptyVertexContainer(id, level)
	} else {
		if container.level != level {
			return nil, fmt.Errorf(
				"new vertex (level=%d, id=%s) is incompatible with existing vertex (%d, %s)",
				container.level, container.id, level, id,
			)
		}
	}
	return container, nil
}

// uncheckedAddEmptyVertexContainer adds a vertexContainer to the internal data structures.
// UNSAFE: will override potentially existing values and/or introduce duplicates
func (f *LeveledForrest) uncheckedAddEmptyVertexContainer(id []byte, level uint64) *vertexContainer {
	v := &vertexContainer{
		id:    id,
		level: level,
	}
	f.vertices[string(v.id)] = v
	vtcs := f.verticesAtLevel[v.level]           // returns nil slice if not yet present
	f.verticesAtLevel[v.level] = append(vtcs, v) // append works on nil slices: creates slice with capacity 2
	return v
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
