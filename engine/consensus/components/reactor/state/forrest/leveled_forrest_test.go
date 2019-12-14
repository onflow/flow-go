package forrest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ~~~~~~~~~~~~~~~~~~~~~ Mock implementation for Vertex ~~~~~~~~~~~~~~~~~~~~~ //
type VertexMock struct {
	id    []byte
	level uint64

	parentId    []byte
	parentLevel uint64
}

func (v *VertexMock) VertexID() []byte         { return v.id }
func (v *VertexMock) Level() uint64            { return v.level }
func (v *VertexMock) Parent() ([]byte, uint64) { return v.parentId, v.parentLevel }

func NewVertexMock(vertexId string, vertexLevel uint64, parentId string, parentLevel uint64) *VertexMock {
	return &VertexMock{
		id:          []byte(vertexId),
		level:       vertexLevel,
		parentId:    []byte(parentId),
		parentLevel: parentLevel,
	}
}

var TestVertices = map[string]*VertexMock{
	"A": NewVertexMock("A", 3, "Genesis", 0),
	"B": NewVertexMock("B", 1, "Genesis", 0),
	"C": NewVertexMock("C", 2, "B", 1),
	"D": NewVertexMock("D", 3, "C", 2),
	"W": NewVertexMock("W", 4, "Missing1", 3),
	"X": NewVertexMock("X", 5, "Missing2", 4),
	"Y": NewVertexMock("Y", 6, "X", 5),
	"Z": NewVertexMock("Z", 6, "X", 5),
}

// ~~~~~~~~~~~~~~~~~~~~~~~~ Tests for VertexIterator ~~~~~~~~~~~~~~~~~~~~~~~~ //

// TestVertexIterator tests VertexIterator on non-empty list
func TestVertexIterator(t *testing.T) {
	var vl VertexList
	for i := 1; i <= 10; i++ {
		b := VertexMock{level: uint64(i)}
		c := vertexContainer{vertex: &b}
		vl = append(vl, &c)
	}

	i := 0
	for I := newVertexIterator(vl); I.HasNext(); {
		assert.Equal(t, vl[i].vertex, I.NextVertex())
		i++
	}
}

// TestVertexIteratorOnEmpty tests VertexIterator on non-empty and nil lists
func TestVertexIteratorOnEmpty(t *testing.T) {
	I := newVertexIterator(nil)
	assert.False(t, I.HasNext())
	assert.True(t, I.NextVertex() == nil)

	I = newVertexIterator(VertexList{})
	assert.False(t, I.HasNext())
	assert.True(t, I.NextVertex() == nil)
}

// ~~~~~~~~~~~~~~~~~~~~~~~~ Tests for LeveledForrest ~~~~~~~~~~~~~~~~~~~~~~~~ //

// TestLeveledForrest_DoubleAddVertex tests that Vertex can be added twice without problems
func TestLeveledForrest_AddVertex(t *testing.T) {
	F := NewLeveledForrest()

	// Adding Vertex twice should be fine
	F.AddVertex(NewVertexMock("A", 3, "Genesis", 0))
	F.AddVertex(NewVertexMock("A", 3, "Genesis", 0))

	// checking that requests panics when it encounters a Vertex with an ID identical to a known reference
	// BUT whose level is not consistent with the reference
	assert.Panics(t, func() { // adding KNOWN vertex but with wrong level number
		F.AddVertex(NewVertexMock("A", 10, "Genesis", 0))
	})
	assert.Panics(t, func() { // adding vertex whose PARENT references a known vertex but with mismatching level
		F.AddVertex(NewVertexMock("F", 4, "Genesis", 10))
	})
}

// TestLeveledForrest_HasVertex test that vertices as correctly reported as contained in forrest
// NOTE: We consider a vertex added only if it has been directly added through the AddVertex method.
//       Vertices that references bvy known vertices but have not themselves are considered to be not in the tree.
func TestLeveledForrest_HasVertex(t *testing.T) {
	F := populateNewForrest()
	assert.True(t, F.HasVertex(toVertexInfo("A", 3)))
	assert.True(t, F.HasVertex(toVertexInfo("B", 1)))
	assert.True(t, F.HasVertex(toVertexInfo("X", 5)))

	assert.False(t, F.HasVertex(toVertexInfo("Genesis", 0)))         // Genesis block never directly added (only referenced) => unknown
	assert.False(t, F.HasVertex(toVertexInfo("NotYetAdded", 10000))) // Block never mentioned before

	// checking that requests panics when it encounters a Vertex with an ID identical to a known reference
	// BUT whose level is not consistent with the reference
	assert.Panics(t, func() { // inquiring about KNOWN vertex but with wrong level number
		F.HasVertex(toVertexInfo("C", 10))
	})
	assert.Panics(t, func() { // inquiring about vertex that is NOT in Tree (but known through references)
		F.HasVertex(toVertexInfo("Genesis", 10))
	})

}

// TestLeveledForrest_GetChildren tests that children are returned properly
func TestLeveledForrest_GetChildren(t *testing.T) {
	F := populateNewForrest()

	// testing children for Block that is contained in Tree
	it := F.GetChildren(toVertexInfo("X", 5))
	expectedChildren := []*VertexMock{
		TestVertices["Y"],
		TestVertices["Z"],
	}
	assert.ElementsMatch(t, expectedChildren, children2List(&it))

	// testing children for referenced Block that is NOT contained in Tree
	it = F.GetChildren(toVertexInfo("Genesis", 0))
	expectedChildren = []*VertexMock{
		TestVertices["A"],
		TestVertices["B"],
	}
	assert.ElementsMatch(t, expectedChildren, children2List(&it))

	// testing children for Block that is contained in Tree but no children are known
	it = F.GetChildren(toVertexInfo("D", 3))
	assert.False(t, it.HasNext())

	// checking that requests panics when it encounters a Vertex with an ID identical to a known reference
	// BUT whose level is not consistent with the reference
	assert.Panics(t, func() { // for vertex that IS in Tree but with wrong level number
		F.GetChildren(toVertexInfo("C", 10))
	})
	assert.Panics(t, func() { // for vertex that is NOT in Tree (but known through references)
		F.GetChildren(toVertexInfo("Genesis", 10))
	})
}

// TestLeveledForrest_GetNumberOfChildren tests that children are returned properly
func TestLeveledForrest_GetNumberOfChildren(t *testing.T) {
	F := populateNewForrest()

	// testing children for Block that is contained in Tree
	assert.Equal(t, 2, F.GetNumberOfChildren(toVertexInfo("X", 5)))

	// testing children for referenced Block that is NOT contained in Tree
	assert.Equal(t, 2, F.GetNumberOfChildren(toVertexInfo("Genesis", 0)))

	// testing children for Block that is contained in Tree but no children are known
	assert.Equal(t, 0, F.GetNumberOfChildren(toVertexInfo("D", 3)))

	// checking that requests panics when it encounters a Vertex with an ID identical to a known reference
	// BUT whose level is not consistent with the reference
	assert.Panics(t, func() { // for vertex that IS in Tree but with wrong level number
		F.GetNumberOfChildren(toVertexInfo("C", 10))
	})
	assert.Panics(t, func() { // for vertex that is NOT in Tree (but known through references)
		F.GetNumberOfChildren(toVertexInfo("Genesis", 10))
	})
}

// TestLeveledForrest_GetVerticesAtLevel tests that Vertex blob is returned properly
func TestLeveledForrest_GetVerticesAtLevel(t *testing.T) {
	F := populateNewForrest()

	// testing vertices for level that are contained in Tree
	it := F.GetVerticesAtLevel(6)
	expectedChildren := []*VertexMock{
		TestVertices["Y"],
		TestVertices["Z"],
	}
	assert.ElementsMatch(t, expectedChildren, children2List(&it))

	// testing vertices for level that are not in Tree but referenced by vertices in the Tree
	it = F.GetVerticesAtLevel(0)
	assert.ElementsMatch(t, []*VertexMock{}, children2List(&it))

	// testing vertices for level with a mixture of referenced but unknown vertices and known vertices
	it = F.GetVerticesAtLevel(4)
	expectedChildren = []*VertexMock{
		TestVertices["W"],
	}
	assert.ElementsMatch(t, expectedChildren, children2List(&it))

	// testing vertices for level that are neither in Tree nor referenced by vertices in the Tree
	it = F.GetVerticesAtLevel(100000)
	assert.ElementsMatch(t, []*VertexMock{}, children2List(&it))
}

// TestLeveledForrest_GetNumberOfVerticesAtLevel tests that Vertex blob is returned properly
func TestLeveledForrest_GetNumberOfVerticesAtLevel(t *testing.T) {
	F := populateNewForrest()

	// testing vertices for level that are contained in Tree
	assert.Equal(t, 2, F.GetNumberOfVerticesAtLevel(6))

	// testing vertices for level that are not in Tree but referenced by vertices in the Tree
	assert.Equal(t, 0, F.GetNumberOfVerticesAtLevel(0))

	// testing vertices for level with a mixture of referenced but unknown vertices and known vertices
	assert.Equal(t, 1, F.GetNumberOfVerticesAtLevel(4))

	// testing vertices for level that are neither in Tree nor referenced by vertices in the Tree
	assert.Equal(t, 0, F.GetNumberOfVerticesAtLevel(100000))
}

// TestLeveledForrest_GetVertex tests that Vertex blob is returned properly
func TestLeveledForrest_GetVertex(t *testing.T) {
	F := populateNewForrest()
	v, exists := F.GetVertex(toVertexInfo("D", 3))
	assert.Equal(t, TestVertices["D"], v)
	assert.True(t, exists)

	v, exists = F.GetVertex(toVertexInfo("X", 5))
	assert.Equal(t, TestVertices["X"], v)
	assert.True(t, exists)

	v, exists = F.GetVertex(toVertexInfo("Genesis", 0))
	assert.Equal(t, (Vertex)(nil), v)
	assert.False(t, exists)

	// checking that requests panics when it encounters a Vertex with an ID identical to a known reference
	// BUT whose level is not consistent with the reference
	assert.Panics(t, func() { // getting vertex with ID "C" BUT different level
		F.GetVertex(toVertexInfo("C", 10))
	})
	assert.Panics(t, func() { // for vertex that is NOT in Tree (but known through references)
		F.GetVertex(toVertexInfo("Genesis", 10))
	})
}

// TestLeveledForrest_GetVertex tests that Vertex blob is returned properly
func TestLeveledForrest_PruneAtLevel(t *testing.T) {
	F := populateNewForrest()
	F.PruneAtLevel(0)
	assert.False(t, F.HasVertex(toVertexInfo("Genesis", 0)))
	assert.True(t, F.HasVertex(toVertexInfo("A", 3)))
	assert.True(t, F.HasVertex(toVertexInfo("B", 1)))
	assert.True(t, F.HasVertex(toVertexInfo("C", 2)))
	assert.True(t, F.HasVertex(toVertexInfo("D", 3)))
	assert.True(t, F.HasVertex(toVertexInfo("X", 5)))
	assert.True(t, F.HasVertex(toVertexInfo("Y", 6)))
	assert.True(t, F.HasVertex(toVertexInfo("Z", 6)))

	F.PruneAtLevel(2)
	assert.False(t, F.HasVertex(toVertexInfo("Genesis", 0)))
	assert.True(t, F.HasVertex(toVertexInfo("A", 3)))
	assert.False(t, F.HasVertex(toVertexInfo("B", 1)))
	assert.False(t, F.HasVertex(toVertexInfo("C", 2)))
	assert.True(t, F.HasVertex(toVertexInfo("D", 3)))
	assert.True(t, F.HasVertex(toVertexInfo("X", 5)))
	assert.True(t, F.HasVertex(toVertexInfo("Y", 6)))
	assert.True(t, F.HasVertex(toVertexInfo("Z", 6)))

	F.PruneAtLevel(5)
	assert.False(t, F.HasVertex(toVertexInfo("Genesis", 0)))
	assert.False(t, F.HasVertex(toVertexInfo("A", 3)))
	assert.False(t, F.HasVertex(toVertexInfo("B", 1)))
	assert.False(t, F.HasVertex(toVertexInfo("C", 2)))
	assert.False(t, F.HasVertex(toVertexInfo("D", 3)))
	assert.False(t, F.HasVertex(toVertexInfo("X", 5)))
	assert.True(t, F.HasVertex(toVertexInfo("Y", 6)))
	assert.True(t, F.HasVertex(toVertexInfo("Z", 6)))

	// pruning at same level repeatedly should be fine
	F.PruneAtLevel(5)
	assert.True(t, F.HasVertex(toVertexInfo("Y", 6)))
	assert.True(t, F.HasVertex(toVertexInfo("Z", 6)))

	// checking that pruning at lower level than what is already pruned results in panic
	assert.Panics(t, func() { // getting vertex with ID "C" BUT different level
		F.PruneAtLevel(4)
	})
}

// TestLeveledForrest_CopyDescendants tests that CopyDescendants copies the Sub-tree from the specified root
// (but not the root itself) to the target forrest
func TestLeveledForrest_CopyDescendants(t *testing.T) {
	source := populateNewForrest()
	target := NewLeveledForrest()

	id, level := toVertexInfo("A", 3)
	source.CopyDescendants(id, level, target) // Copies descendants of A but not A itself
	id, level = toVertexInfo("B", 1)
	source.CopyDescendants(id, level, target)
	assert.False(t, target.HasVertex(toVertexInfo("Genesis", 0)))
	assert.False(t, target.HasVertex(toVertexInfo("A", 3)))
	assert.False(t, target.HasVertex(toVertexInfo("B", 1)))
	assert.True(t, target.HasVertex(toVertexInfo("C", 2)))
	assert.True(t, target.HasVertex(toVertexInfo("D", 3)))
	assert.False(t, target.HasVertex(toVertexInfo("X", 5)))
	assert.False(t, target.HasVertex(toVertexInfo("Y", 6)))
	assert.False(t, target.HasVertex(toVertexInfo("Z", 6)))

	id, level = toVertexInfo("Missing2", 4)
	source.CopyDescendants(id, level, target)
	assert.False(t, target.HasVertex(toVertexInfo("Genesis", 0)))
	assert.False(t, target.HasVertex(toVertexInfo("A", 3)))
	assert.False(t, target.HasVertex(toVertexInfo("B", 1)))
	assert.True(t, target.HasVertex(toVertexInfo("C", 2)))
	assert.True(t, target.HasVertex(toVertexInfo("D", 3)))
	assert.True(t, target.HasVertex(toVertexInfo("X", 5)))
	assert.True(t, target.HasVertex(toVertexInfo("Y", 6)))
	assert.True(t, target.HasVertex(toVertexInfo("Z", 6)))

	// checking that requests panics when it encounters a Vertex with an ID identical to a known reference
	// BUT whose level is not consistent with the reference
	assert.Panics(t, func() { // getting vertex with ID "C" BUT different level
		id, level = toVertexInfo("C", 10)
		source.CopyDescendants(id, level, target)
	})
	assert.Panics(t, func() { // for vertex that is NOT in Tree (but known through references)
		id, level = toVertexInfo("Genesis", 10)
		source.CopyDescendants(id, level, target)
	})
}

// ~~~~~~~~~~~~~~~~~~~~~~~~~~~ Helper Functions ~~~~~~~~~~~~~~~~~~~~~~~~~~~~ //

func populateNewForrest() *LeveledForrest {
	F := NewLeveledForrest()
	for _, v := range TestVertices {
		F.AddVertex(v)
	}
	return F
}

func children2List(it *vertexIterator) []*VertexMock {
	l := []*VertexMock{}
	for it.HasNext() {
		// Vertex interface is implemented by VertexMock POINTER!
		// Hence, the concrete type is *VertexMock
		v := it.NextVertex().(*VertexMock)
		l = append(l, v)
	}
	return l
}

func toVertexInfo(vertexId string, vertexLevel uint64) ([]byte, uint64) {
	return []byte(vertexId), vertexLevel
}
