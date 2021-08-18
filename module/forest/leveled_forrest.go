package forest

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// LevelledForest contains multiple trees (which is a potentially disconnected planar graph).
// Each vertexContainer in the graph has a level (view) and a hash. A vertexContainer can only have one parent
// with strictly smaller level (view). A vertexContainer can have multiple children, all with
// strictly larger level (view).
// A LevelledForest provides the ability to prune all vertices up to a specific level.
// A tree whose root is below the pruning threshold might decompose into multiple
// disconnected subtrees as a result of pruning.
type LevelledForest struct {
	vertices        VertexSet
	verticesAtLevel map[uint64]VertexList
	size            uint64
	LowestLevel     uint64
}

type VertexList []*vertexContainer
type VertexSet map[flow.Identifier]*vertexContainer

// vertexContainer holds information about a tree vertex. Internally, we distinguish between
// * FULL container: has non-nil value for vertex.
//   Used for vertices, which have been added to the tree.
// * EMPTY container: has NIL value for vertex.
//   Used for vertices, which have NOT been added to the tree, but are
//   referenced by vertices in the tree. An empty container is converted to a
//   full container when the respective vertex is added to the tree
type vertexContainer struct {
	id       flow.Identifier
	level    uint64
	children VertexList

	// the following are only set if the block is actually known
	vertex Vertex
}

// NewLevelledForest initializes a LevelledForest
func NewLevelledForest(lowestLevel uint64) *LevelledForest {
	return &LevelledForest{
		vertices:        make(VertexSet),
		verticesAtLevel: make(map[uint64]VertexList),
		LowestLevel:     lowestLevel,
	}
}

// PruneUpToLevel prunes all blocks UP TO but NOT INCLUDING `level`
func (f *LevelledForest) PruneUpToLevel(level uint64) error {
	if level < f.LowestLevel {
		return fmt.Errorf("new lowest level %d cannot be smaller than previous last retained level %d", level, f.LowestLevel)
	}
	if len(f.vertices) == 0 {
		f.LowestLevel = level
		return nil
	}

	elementsPruned := 0

	// to optimize the pruning large level-ranges, we compare:
	//  * the number of levels for which we have stored vertex containers: len(f.verticesAtLevel)
	//  * the number of levels that need to be pruned: level-f.LowestLevel
	// We iterate over the dimension which is smaller.
	if uint64(len(f.verticesAtLevel)) < level-f.LowestLevel {
		for l, vertices := range f.verticesAtLevel {
			if l < level {
				for _, v := range vertices {
					if !f.isEmptyContainer(v) {
						elementsPruned++
					}
					delete(f.vertices, v.id)
				}
				delete(f.verticesAtLevel, l)
			}
		}
	} else {
		for l := f.LowestLevel; l < level; l++ {
			verticesAtLevel := f.verticesAtLevel[l]
			for _, v := range verticesAtLevel { // nil map behaves like empty map when iterating over it
				if !f.isEmptyContainer(v) {
					elementsPruned++
				}
				delete(f.vertices, v.id)
			}
			delete(f.verticesAtLevel, l)

		}
	}
	f.LowestLevel = level
	f.size -= uint64(elementsPruned)
	return nil
}

// HasVertex returns true iff full vertex exists
func (f *LevelledForest) HasVertex(id flow.Identifier) bool {
	container, exists := f.vertices[id]
	return exists && !f.isEmptyContainer(container)
}

// isEmptyContainer returns true iff vertexContainer container is empty, i.e. full vertex itself has not been added
func (f *LevelledForest) isEmptyContainer(vertexContainer *vertexContainer) bool {
	return vertexContainer.vertex == nil
}

// GetVertex returns (<full vertex>, true) if the vertex with `id` and `level` was found
// (nil, false) if full vertex is unknown
func (f *LevelledForest) GetVertex(id flow.Identifier) (Vertex, bool) {
	container, exists := f.vertices[id]
	if !exists || f.isEmptyContainer(container) {
		return nil, false
	}
	return container.vertex, true
}

// GetSize returns the total number of vertices above the pruned lowest level.
// Note this call is not concurrent-safe, caller is responsible to ensure concurrency safety.
func (f *LevelledForest) GetSize() uint64 {
	return f.size
}

// GetChildren returns a VertexIterator to iterate over the children
// An empty VertexIterator is returned, if no vertices are known whose parent is `id` , `level`
func (f *LevelledForest) GetChildren(id flow.Identifier) VertexIterator {
	container := f.vertices[id]
	// if vertex does not exists, container is the default zero value for vertexContainer, which contains a nil-slice for its children
	return newVertexIterator(container.children) // VertexIterator gracefully handles nil slices
}

// GetNumberOfChildren returns number of children of given vertex
func (f *LevelledForest) GetNumberOfChildren(id flow.Identifier) int {
	container := f.vertices[id] // if vertex does not exists, container is the default zero value for vertexContainer, which contains a nil-slice for its children
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
func (f *LevelledForest) GetVerticesAtLevel(level uint64) VertexIterator {
	return newVertexIterator(f.verticesAtLevel[level]) // go returns the zero value for a missing level. Here, a nil slice
}

// GetNumberOfVerticesAtLevel returns number of vertices at given level
func (f *LevelledForest) GetNumberOfVerticesAtLevel(level uint64) int {
	num := 0
	for _, container := range f.verticesAtLevel[level] {
		if container.vertex != nil {
			num++
		}
	}
	return num
}

// AddVertex adds vertex to forest if vertex is within non-pruned levels
// Handles repeated addition of same vertex (keeps first added vertex).
// If vertex is at or below pruning level: method is NoOp.
// UNVALIDATED:
// requires that vertex would pass validity check LevelledForest.VerifyVertex(vertex).
func (f *LevelledForest) AddVertex(vertex Vertex) {
	if vertex.Level() < f.LowestLevel {
		return
	}
	container := f.getOrCreateVertexContainer(vertex.VertexID(), vertex.Level())
	if !f.isEmptyContainer(container) { // the vertex was already stored
		return
	}
	// container is empty, i.e. full vertex is new and should be stored in container
	container.vertex = vertex // add vertex to container
	f.registerWithParent(container)
	f.size += 1
}

func (f *LevelledForest) registerWithParent(vertexContainer *vertexContainer) {
	// caution: do not modify this combination of check (a) and (a)
	// Deliberate handling of root vertex (genesis block) whose view is _exactly_ at LowestLevel
	// For this block, we don't care about its parent and the exception is allowed where
	// vertex.level = vertex.Parent().Level = LowestLevel = 0
	if vertexContainer.level <= f.LowestLevel { // check (a)
		return
	}

	_, parentView := vertexContainer.vertex.Parent()
	if parentView < f.LowestLevel {
		return
	}
	parentContainer := f.getOrCreateVertexContainer(vertexContainer.vertex.Parent())
	parentContainer.children = append(parentContainer.children, vertexContainer) // append works on nil slices: creates slice with capacity 2
}

// getOrCreateVertexContainer returns the vertexContainer if there exists one
// or creates a new vertexContainer and adds it to the internal data structures.
// It errors if a vertex with same id but different Level is already known
// (i.e. there exists an empty or full container with the same id but different level).
func (f *LevelledForest) getOrCreateVertexContainer(id flow.Identifier, level uint64) *vertexContainer {
	container, exists := f.vertices[id] // try to find vertex container with same ID
	if !exists {                        // if no vertex container found, create one and store it
		container = &vertexContainer{
			id:    id,
			level: level,
		}
		f.vertices[container.id] = container
		vtcs := f.verticesAtLevel[container.level]                   // returns nil slice if not yet present
		f.verticesAtLevel[container.level] = append(vtcs, container) // append works on nil slices: creates slice with capacity 2
	}
	return container
}

// VerifyVertex verifies that vertex satisfies the following conditions
// (1)
func (f *LevelledForest) VerifyVertex(vertex Vertex) error {
	if vertex.Level() < f.LowestLevel {
		return nil
	}
	isKnownVertex, err := f.isEquivalentToStoredVertex(vertex)
	if err != nil {
		return fmt.Errorf("invalid Vertex: %w", err)
	}
	if isKnownVertex {
		return nil
	}
	// vertex not found in storage => new vertex

	// verify new vertex
	if vertex.Level() == f.LowestLevel {
		return nil
	}
	return f.verifyParent(vertex)
}

// isEquivalentToStoredVertex
// evaluates whether a vertex is equivalent to already stored vertex.
// for vertices at pruning level, parents are ignored
//
// (1) return value (false, nil)
// Two vertices are _not equivalent_ if they have different IDs (Hashes).
//
// (2) return value (true, nil)
// Two vertices _are equivalent_ if their respective fields are identical:
// ID, Level, and Parent (both parent ID and parent Level)
//
// (3) return value (false, error)
// errors if the vertices' IDs are identical but they differ
// in any of the _relevant_ fields (as defined in (2)).
func (f *LevelledForest) isEquivalentToStoredVertex(vertex Vertex) (bool, error) {
	storedVertex, haveStoredVertex := f.GetVertex(vertex.VertexID())
	if !haveStoredVertex {
		return false, nil //have no vertex with same id stored
	}

	// found vertex in storage with identical ID
	// => we expect all other (relevant) fields to be identical
	if vertex.Level() != storedVertex.Level() { // view number
		return false, fmt.Errorf("conflicting vertices with ID %v", vertex.VertexID())
	}
	if vertex.Level() <= f.LowestLevel {
		return true, nil
	}
	newParentId, newParentView := vertex.Parent()
	storedParentId, storedParentView := storedVertex.Parent()
	if newParentId != storedParentId { // qc.blockID
		return false, fmt.Errorf("conflicting vertices with ID %v", vertex.VertexID())
	}
	if newParentView != storedParentView { // qc.view
		return false, fmt.Errorf("conflicting vertices with ID %v", vertex.VertexID())
	}
	// all _relevant_ fields identical
	return true, nil
}

// verifyParent verifies whether vertex.Parent() is consistent with current forest.
// An error is raised if
// * there is a parent with the same id but different view;
// * the parent's level is _not_ smaller than the vertex's level
func (f *LevelledForest) verifyParent(vertex Vertex) error {
	// verify parent
	parentID, parentLevel := vertex.Parent()
	if !(vertex.Level() > parentLevel) {
		return fmt.Errorf("parent vertex's level (%d) must be smaller than the vertex's level (%d)", parentLevel, vertex.Level())
	}
	parentVertex, haveParentStored := f.GetVertex(parentID)
	if !haveParentStored {
		return nil
	}
	if parentVertex.Level() != parentLevel {
		return fmt.Errorf("parent vertex of %v has different level (%d) than the stored vertex (%d)",
			vertex.VertexID(), parentLevel, parentVertex.Level(),
		)
	}
	return nil
}
