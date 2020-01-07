package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// LeveledForrest contains multiple trees (which is a potentially disconnected planar graph).
// Each vertexContainer in the graph has a level (view) and a hash. A vertexContainer can only have one parent
// with strictly smaller level (view). A vertexContainer can only have multiple children, all with
// strictly larger level (view).
// A LeveledForrest provides the ability to prune all vertices up to a specific level.
// A tree whose root is below the pruning threshold might decompose into multiple
// disconnected subtrees as a result of pruning.
type LevelledForrest struct {
	// rw lock
	vertices    map[types.MRH]Vertex
	levels      map[uint64][]Vertex
	lowestLevel uint64
}

const LOWEST_LEVEL = 1

type Vertex interface {
	// ID returns the vertex's ID (in most cases its hash)
	ID() types.MRH
	// Level returns the vertex's level, level starts from LOWEST_LEVEL
	Level() uint64
	// Parent returns the returns the parents (ID, level)
	Parent() (types.MRH, uint64)
}

// PruneAtLevel prunes all vertices below the given level
// 3, 3 <- 4 <- 5 <- 6 <- 7
// PruneAtLevel(4) will return
// 4, 4 <- 5 <- 6 <- 7
func (f *LevelledForrest) PruneAtLevel(level uint64) {
	if level <= f.lowestLevel {
		return
	}
	for l := f.lowestLevel; l <= level-1; l++ {
		f.deleteAtLevel(l)
	}
	f.lowestLevel = level
}

// AddVertex add a vertex to the LevelledForrest structure, and returns if it was added.
// It returns false if the vertex already exists
//		or can not be incorperated
func (f *LevelledForrest) AddIncorporatableVertex(v Vertex) bool {
	level := v.Level()
	id := v.ID()
	if level < LOWEST_LEVEL {
		// should not accept a block with level 0
		return false
	}

	_, exists := f.vertices[id]
	if exists {
		// exists already
		return false
	}

	if !f.CanIncorporate(v) {
		return false
	}

	f.vertices[id] = v

	sameLevelVertices := f.levels[level]
	f.levels[level] = append(sameLevelVertices, v)

	return true
}

func (f *LevelledForrest) FindVerticesByLevel(level uint64) []Vertex {
	return f.levels[level]
}

func (f *LevelledForrest) FindVerticeByID(id types.MRH) (Vertex, bool) {
	v, exists := f.vertices[id]
	return v, exists
}

func (f *LevelledForrest) CanIncorporate(v Vertex) bool {
	if v.Level() <= f.lowestLevel {
		return false
	}

	parentID, parentLevel := v.Parent()

	if parentLevel >= f.lowestLevel {
		_, parentExists := f.vertices[parentID]
		if !parentExists {
			// unincorperated vertex are not allowed to be added
			return false
		}
	}
	return true
}

func (f *LevelledForrest) FindThreeParentsOf(id types.MRH) (parent, grantParent, greatGrantParent Vertex) {
	v, exists := f.vertices[id]
	if !exists {
		return nil, nil, nil
	}

	parent, exists = f.parentOf(v)
	if !exists {
		return nil, nil, nil
	}

	grantParent, exists = f.parentOf(parent)
	if !exists {
		return parent, nil, nil
	}

	greatGrantParent, exists = f.parentOf(grantParent)
	if !exists {
		return parent, grantParent, nil
	}

	return parent, grantParent, greatGrantParent
}

// parentOf find the parent of the given vertex, assuming the vertex exists
func (f *LevelledForrest) parentOf(v Vertex) (Vertex, bool) {
	// Genesis doesn't have parent
	if v.Level() < LOWEST_LEVEL {
		return nil, false
	}

	parentID, _ := v.Parent()
	parent, exists := f.vertices[parentID]
	if !exists {
		return nil, false
	}
	return parent, true
}

func (f *LevelledForrest) deleteAtLevel(level uint64) {
	for _, v := range f.levels[level] {
		delete(f.vertices, v.ID())
	}
	delete(f.levels, level)
}
