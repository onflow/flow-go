package forest

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// LevelledForest contains multiple trees (which is a potentially disconnected planar graph).
// Each vertex in the graph has a level and a hash. A vertex can only have one parent, which
// must have strictly smaller level. A vertex can have multiple children, all with strictly
// larger level.
// A LevelledForest provides the ability to prune all vertices up to a specific level.
// A tree whose root is below the pruning threshold might decompose into multiple
// disconnected subtrees as a result of pruning.
// By design, the LevelledForest does _not_ touch the parent information for vertices
// that are on the lowest retained level. Thereby, it is possible to initialize the
// LevelledForest with a root vertex at the lowest retained level, without this root
// needing to have a parent. Furthermore, the root vertex can be at level 0 and in
// absence of a parent still satisfy the condition that any parent must be of lower level
// (mathematical principle of vacuous truth) without the implementation needing to worry
// about unsigned integer underflow.
//
// LevelledForest is NOT safe for concurrent use by multiple goroutines.
type LevelledForest struct {
	vertices        VertexSet
	verticesAtLevel map[uint64]VertexList
	size            uint64
	LowestLevel     uint64
}

type VertexList []*vertexContainer
type VertexSet map[flow.Identifier]*vertexContainer

// vertexContainer holds information about a tree vertex. Internally, we distinguish between
//   - FULL container: has non-nil value for vertex.
//     Used for vertices, which have been added to the tree.
//   - EMPTY container: has NIL value for vertex.
//     Used for vertices, which have NOT been added to the tree, but are
//     referenced by vertices in the tree. An empty container is converted to a
//     full container when the respective vertex is added to the tree
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

// PruneUpToLevel prunes all blocks UP TO but NOT INCLUDING `level`.
// Error returns:
// * mempool.BelowPrunedThresholdError if input level is below the lowest retained level
func (f *LevelledForest) PruneUpToLevel(level uint64) error {
	if level < f.LowestLevel {
		return mempool.NewBelowPrunedThresholdErrorf("new lowest level %d cannot be smaller than previous last retained level %d", level, f.LowestLevel)
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

// HasVertex returns true iff full vertex exists.
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

// GetSize returns the total number of vertices above the lowest pruned level.
// Note this call is not concurrent-safe, caller is responsible to ensure concurrency safety.
func (f *LevelledForest) GetSize() uint64 {
	return f.size
}

// GetChildren returns a VertexIterator to iterate over the children
// An empty VertexIterator is returned, if no vertices are known whose parent is `id`.
func (f *LevelledForest) GetChildren(id flow.Identifier) VertexIterator {
	// if vertex does not exist, container will be nil
	if container, ok := f.vertices[id]; ok {
		return newVertexIterator(container.children)
	}
	return newVertexIterator(nil) // VertexIterator gracefully handles nil slices
}

// GetNumberOfChildren returns number of children of given vertex
func (f *LevelledForest) GetNumberOfChildren(id flow.Identifier) int {
	container := f.vertices[id] // if vertex does not exist, container is the default zero value for vertexContainer, which contains a nil-slice for its children
	num := 0
	for _, child := range container.children {
		if child.vertex != nil {
			num++
		}
	}
	return num
}

// GetVerticesAtLevel returns a VertexIterator to iterate over the Vertices at the specified level.
// An empty VertexIterator is returned, if no vertices are known at the specified level.
// If `level` is already pruned, an empty VertexIterator is returned.
func (f *LevelledForest) GetVerticesAtLevel(level uint64) VertexIterator {
	return newVertexIterator(f.verticesAtLevel[level]) // go returns the zero value for a missing level. Here, a nil slice
}

// GetNumberOfVerticesAtLevel returns the number of full vertices at given level.
// A full vertex is a vertex that was explicitly added to the forest. In contrast,
// an empty vertex container represents a vertex that is _referenced_ as parent by
// one or more full vertices, but has not been added itself to the forest.
// We only count vertices that have been explicitly added to the forest and not yet
// pruned. (In comparision, we do _not_ count vertices that are _referenced_ as
// parent by vertices, but have not been added themselves).
func (f *LevelledForest) GetNumberOfVerticesAtLevel(level uint64) int {
	num := 0
	for _, container := range f.verticesAtLevel[level] {
		if !f.isEmptyContainer(container) {
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

// registerWithParent retrieves the parent and registers the given vertex as a child.
// For a block, whose level equal to the pruning threshold, we do not inspect the parent at all.
// Thereby, this implementation can gracefully handle the corner case where the tree has a defined
// end vertex (distinct root). This is commonly the case in blockchain (genesis, or spork root block).
// Mathematically, this means that this library can also represent bounded trees.
func (f *LevelledForest) registerWithParent(vertexContainer *vertexContainer) {
	// caution, necessary for handling bounded trees:
	// For root vertex (genesis block) the view is _exactly_ at LowestLevel. For these blocks,
	// a parent does not exist. In the implementation, we deliberately do not call the `Parent()` method,
	// as its output is conceptually undefined. Thereby, we can gracefully handle the corner case of
	//   vertex.level = vertex.Parent().Level = LowestLevel = 0
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
// (i.e. there exists an empty or full container with the same id but different level).
func (f *LevelledForest) getOrCreateVertexContainer(id flow.Identifier, level uint64) *vertexContainer {
	container, exists := f.vertices[id] // try to find vertex container with same ID
	if !exists {                        // if no vertex container found, create one and store it
		container = &vertexContainer{
			id:    id,
			level: level,
		}
		f.vertices[container.id] = container
		vertices := f.verticesAtLevel[container.level]                   // returns nil slice if not yet present
		f.verticesAtLevel[container.level] = append(vertices, container) // append works on nil slices: creates slice with capacity 2
	}
	return container
}

// VerifyVertex verifies that adding vertex `v` would yield a valid Levelled Forest.
// Specifically, we verify that _all_ of the following conditions are satisfied:
//
//  1. `v.Level()` must be strictly larger than the level that `v` reports
//     for its parent (maintains an acyclic graph).
//
//  2. If a vertex with the same ID as `v.VertexID()` exists in the graph or is
//     referenced by another vertex within the graph, the level must be identical.
//     (In other words, we don't have vertices with the same ID but different level)
//
//  3. Let `ParentLevel`, `ParentID` denote the level, ID that `v` reports for its parent.
//     If a vertex with `ParentID` exists (or is referenced by other vertices as their parent),
//     we require that the respective level is identical to `ParentLevel`.
//
// Notes:
//   - If `v.Level()` has already been pruned, adding it to the forest is a NoOp.
//     Hence, any vertex with level below the pruning threshold automatically passes.
//   - By design, the LevelledForest does _not_ touch the parent information for vertices
//     that are on the lowest retained level. Thereby, it is possible to initialize the
//     LevelledForest with a root vertex at the lowest retained level, without this root
//     needing to have a parent. Furthermore, the root vertex can be at level 0 and in
//     absence of a parent still satisfy the condition that any parent must be of lower level
//     (mathematical principle of vacuous truth) without the implementation needing to worry
//     about unsigned integer underflow.
//
// Error returns:
// * InvalidVertexError if the input vertex is invalid for insertion to the forest.
func (f *LevelledForest) VerifyVertex(v Vertex) error {
	if v.Level() < f.LowestLevel {
		return nil
	}

	storedContainer, haveVertexContainer := f.vertices[v.VertexID()]
	if !haveVertexContainer { // have no vertex with same id stored
		// the only thing remaining to check is the parent information
		return f.ensureConsistentParent(v)
	}

	// Found a vertex container, i.e. `v` already exists, or it is referenced by some other vertex.
	// In all cases, `v.Level()` should match the vertexContainer's information
	if v.Level() != storedContainer.level {
		return NewInvalidVertexErrorf(v, "level conflicts with stored vertex with same id (%d!=%d)", v.Level(), storedContainer.level)
	}

	// vertex container is empty, i.e. `v` is referenced by some other vertex as its parent:
	if f.isEmptyContainer(storedContainer) {
		// the only thing remaining to check is the parent information
		return f.ensureConsistentParent(v)
	}

	// vertex container holds a vertex with the same ID as `v`:
	// The parent information from vertexContainer has already been checked for consistency. So
	// we simply compare with the existing vertex for inconsistencies

	// the vertex is at or below the lowest retained level, so we can't check the parent (it's pruned)
	if v.Level() == f.LowestLevel {
		return nil
	}

	newParentId, newParentLevel := v.Parent()
	storedParentId, storedParentLevel := storedContainer.vertex.Parent()
	if newParentId != storedParentId {
		return NewInvalidVertexErrorf(v, "parent ID conflicts with stored parent (%x!=%x)", newParentId, storedParentId)
	}
	if newParentLevel != storedParentLevel {
		return NewInvalidVertexErrorf(v, "parent level conflicts with stored parent (%d!=%d)", newParentLevel, storedParentLevel)
	}
	// all _relevant_ fields identical
	return nil
}

// ensureConsistentParent verifies that vertex.Parent() is consistent with current forest.
// Returns InvalidVertexError if:
// * there is a parent with the same ID but different level;
// * the parent's level is _not_ smaller than the vertex's level
func (f *LevelledForest) ensureConsistentParent(vertex Vertex) error {
	if vertex.Level() <= f.LowestLevel {
		// the vertex is at or below the lowest retained level, so we can't check the parent (it's pruned)
		return nil
	}

	// verify parent
	parentID, parentLevel := vertex.Parent()
	if !(vertex.Level() > parentLevel) {
		return NewInvalidVertexErrorf(vertex, "vertex parent level (%d) must be smaller than proposed vertex level (%d)", parentLevel, vertex.Level())
	}
	storedParent, haveParentStored := f.GetVertex(parentID)
	if !haveParentStored {
		return nil
	}
	if storedParent.Level() != parentLevel {
		return NewInvalidVertexErrorf(vertex, "parent level conflicts with stored parent (%d!=%d)", parentLevel, storedParent.Level())
	}
	return nil
}
