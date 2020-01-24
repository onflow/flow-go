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

// TestLeveledForrest_AddVertex tests that Vertex can be added twice without problems
func TestLeveledForrest_AddVertex(t *testing.T) {
	F := NewLeveledForrest()
	err := F.AddVertex(NewVertexMock("A", 3, "Genesis", 0))
	assert.False(t, err != nil)

	// Adding Vertex twice should be fine
	err = F.AddVertex(NewVertexMock("A", 3, "Genesis", 0))
	assert.False(t, err != nil)
}

// TestLeveledForrest_AddInconsistentVertex checks that adding Vertex errors if we try adding a Vertex
// with an ID identical to a known reference BUT whose level is not consistent with the reference
func TestLeveledForrest_AddInconsistentVertex(t *testing.T) {
	F := populateNewForrest()

	// adding KNOWN vertex but with wrong level number
	err := F.AddVertex(NewVertexMock("D", 10, "C", 2))
	assert.True(t, err != nil)

	// adding KNOWN vertex whose PARENT references a known vertex but with mismatching level
	err = F.AddVertex(NewVertexMock("D", 10, "C", 10))
	assert.True(t, err != nil)

	// adding unknown vertex whose PARENT references a known vertex but with mismatching level
	err = F.AddVertex(NewVertexMock("F", 4, "Genesis", 10))
	assert.True(t, err != nil)
}

// TestLeveledForrest_HasVertex test that vertices as correctly reported as contained in forrest
// NOTE: We consider a vertex added only if it has been directly added through the AddVertex method.
//       Vertices that references bvy known vertices but have not themselves are considered to be not in the tree.
func TestLeveledForrest_HasVertex(t *testing.T) {
	F := populateNewForrest()
	assert.True(t, F.HasVertex([]byte("A")))
	assert.True(t, F.HasVertex([]byte("B")))
	assert.True(t, F.HasVertex([]byte("X")))

	assert.False(t, F.HasVertex([]byte("Genesis")))         // Genesis block never directly added (only referenced) => unknown
	assert.False(t, F.HasVertex([]byte("NotYetAdded"))) // Block never mentioned before
}

// TestLeveledForrest_GetChildren tests that children are returned properly
func TestLeveledForrest_GetChildren(t *testing.T) {
	F := populateNewForrest()

	// testing children for Block that is contained in Tree
	it := F.GetChildren([]byte("X"))
	expectedChildren := []*VertexMock{
		TestVertices["Y"],
		TestVertices["Z"],
	}
	assert.ElementsMatch(t, expectedChildren, children2List(&it))

	// testing children for referenced Block that is NOT contained in Tree
	it = F.GetChildren([]byte("Genesis"))
	expectedChildren = []*VertexMock{
		TestVertices["A"],
		TestVertices["B"],
	}
	assert.ElementsMatch(t, expectedChildren, children2List(&it))

	// testing children for Block that is contained in Tree but no children are known
	it = F.GetChildren([]byte("D"))
	assert.False(t, it.HasNext())
}

// TestLeveledForrest_GetNumberOfChildren tests that children are returned properly
func TestLeveledForrest_GetNumberOfChildren(t *testing.T) {
	F := populateNewForrest()

	// testing children for Block that is contained in Tree
	assert.Equal(t, 2, F.GetNumberOfChildren([]byte("X")))

	// testing children for referenced Block that is NOT contained in Tree
	assert.Equal(t, 2, F.GetNumberOfChildren([]byte("Genesis")))

	// testing children for Block that is contained in Tree but no children are known
	assert.Equal(t, 0, F.GetNumberOfChildren([]byte("D")))
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

// TestLeveledForrest_GetNumberOfVerticesAtLevel tests that the number of vertices at a specified level is reported correctly. 
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
	v, exists := F.GetVertex([]byte("D"))
	assert.Equal(t, TestVertices["D"], v)
	assert.True(t, exists)

	v, exists = F.GetVertex([]byte("X"))
	assert.Equal(t, TestVertices["X"], v)
	assert.True(t, exists)

	v, exists = F.GetVertex([]byte("Genesis"))
	assert.Equal(t, (Vertex)(nil), v)
	assert.False(t, exists)
}

// TestLeveledForrest_GetVertex tests that Vertex blob is returned properly
func TestLeveledForrest_PruneAtLevel(t *testing.T) {
	F := populateNewForrest()
	err := F.PruneAtLevel(0)
	assert.False(t, err != nil)
	assert.False(t, F.HasVertex([]byte("Genesis")))
	assert.True(t, F.HasVertex([]byte("A")))
	assert.True(t, F.HasVertex([]byte("B")))
	assert.True(t, F.HasVertex([]byte("C")))
	assert.True(t, F.HasVertex([]byte("D")))
	assert.True(t, F.HasVertex([]byte("X")))
	assert.True(t, F.HasVertex([]byte("Y")))
	assert.True(t, F.HasVertex([]byte("Z")))

	err = F.PruneAtLevel(2)
	assert.False(t, err != nil)
	assert.False(t, F.HasVertex([]byte("Genesis")))
	assert.True(t, F.HasVertex([]byte("A")))
	assert.False(t, F.HasVertex([]byte("B")))
	assert.False(t, F.HasVertex([]byte("C")))
	assert.True(t, F.HasVertex([]byte("D")))
	assert.True(t, F.HasVertex([]byte("X")))
	assert.True(t, F.HasVertex([]byte("Y")))
	assert.True(t, F.HasVertex([]byte("Z")))

	err = F.PruneAtLevel(5)
	assert.False(t, err != nil)
	assert.False(t, F.HasVertex([]byte("Genesis")))
	assert.False(t, F.HasVertex([]byte("A")))
	assert.False(t, F.HasVertex([]byte("B")))
	assert.False(t, F.HasVertex([]byte("C")))
	assert.False(t, F.HasVertex([]byte("D")))
	assert.False(t, F.HasVertex([]byte("X")))
	assert.True(t, F.HasVertex([]byte("Y")))
	assert.True(t, F.HasVertex([]byte("Z")))

	// pruning at same level repeatedly should be fine
	err = F.PruneAtLevel(5)
	assert.False(t, err != nil)
	assert.True(t, F.HasVertex([]byte("Y")))
	assert.True(t, F.HasVertex([]byte("Z")))

	// checking that pruning at lower level than what is already pruned results in error
	err = F.PruneAtLevel(4)
	assert.True(t, err != nil)
}


// ~~~~~~~~~~~~~~~~~~~~~~~~~~~ Helper Functions ~~~~~~~~~~~~~~~~~~~~~~~~~~~~ //

func populateNewForrest() *LeveledForrest {
	F := NewLeveledForrest()
	for _, v := range TestVertices {
		F.AddVertex(v)
	}
	return F
}

func children2List(it *VertexIterator) []*VertexMock {
	l := []*VertexMock{}
	for it.HasNext() {
		// Vertex interface is implemented by VertexMock POINTER!
		// Hence, the concrete type is *VertexMock
		v := it.NextVertex().(*VertexMock)
		l = append(l, v)
	}
	return l
}

