package forest

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_SlicePrimitives demonstrates that we can use slices, including `VertexList`
// as concurrency-safe snapshots.
func Test_SlicePrimitives(t *testing.T) {
	// Conceptually, we always proceed along the following pattern:
	//  • We assume there is a LevelledForest instance, protected for concurrent access by higher-level
	//    business logic (not represented in this test).
	//  • The higher-level business logic instantiates a `VertexIterator` (not represented in this test) by calling
	//    `GetChildren` or `GetVerticesAtLevel` for example. Under the hood, the `VertexIterator` receives a `VertexList`
	//    as it's sole input. The slice `VertexList` golang internally represents as the tripel
	//    [pointer to array, slice length, slice capacity] (see https://go.dev/blog/slices-intro for details). The slice
	//    is passed by value, i.e. `VertexIterator` maintains its own copy of these values.
	//  • Here, we emulate interleaving writes by the forest to the shared slice `VertexList`.

	v := NewVertexMock("v", 3, "C", 2)
	vContainer := &vertexContainer{id: unittest.IdentifierFixture(), level: 3, vertex: v}

	t.Run("nil slice", func(t *testing.T) {
		// Prepare vertex list that, representing the slice of children held by the
		var vertexList VertexList // nil zero value

		// vertex iterator maintains a snapshot of a nil slice
		iterator := newVertexIterator(vertexList)

		// Emulating concurrent access, where new data is added by the forest:
		// we expect that vertexList was expanded, but the iterator's notion should be unchanged
		vertexList = append(vertexList, vContainer)
		assert.Nil(t, iterator.data)
		assert.Equal(t, len(vertexList), len(iterator.data)+1)
	})

	t.Run("empty slice of zero capacity", func(t *testing.T) {
		// Prepare vertex list that, representing the slice of children held by the
		var vertexList VertexList = []*vertexContainer{}

		// vertex iterator maintains a snapshot of the non-nil slice, with zero capacity
		iterator := newVertexIterator(vertexList)

		// Emulating concurrent access, where new data is added by the forest:
		// we expect that vertexList was expanded, but the iterator's notion should be unchanged
		vertexList = append(vertexList, vContainer)
		assert.NotNil(t, iterator.data)
		assert.Zero(t, len(iterator.data))
		assert.Equal(t, len(vertexList), len(iterator.data)+1)
	})

	t.Run("empty slice of with capacity 2 (len = 0, cap = 2)", func(t *testing.T) {
		// Prepare vertex list that, representing the slice of children held by the
		var vertexList VertexList = make(VertexList, 0, 2)

		// vertex iterator maintains a snapshot of a slice with length zero but capacity 2
		iterator := newVertexIterator(vertexList)

		// Emulating concurrent access, where new data is added by the forest:
		// we expect that vertexList was expanded, but the iterator's notion should be unchanged
		vertexList = append(vertexList, vContainer)
		assert.NotNil(t, iterator.data)
		assert.Zero(t, len(iterator.data))
		assert.Equal(t, 2, cap(iterator.data))
		assert.Equal(t, len(vertexList), len(iterator.data)+1)
	})

	t.Run("non-empty slice with larger capacity (len = 1, cap = 2)", func(t *testing.T) {
		// Prepare vertex list that, representing the slice of children held by the
		var vertexList VertexList = make(VertexList, 1, 2)
		_v := NewVertexMock("v", 3, "C", 2)
		vertexList[0] = &vertexContainer{id: unittest.IdentifierFixture(), level: 3, vertex: _v}

		// vertex iterator maintains a snapshot of a slice with length 1 but capacity 2
		iterator := newVertexIterator(vertexList)

		// Emulating concurrent access, where new data is added by the forest:
		// we expect that vertexList was expanded, but the iterator's notion should be unchanged
		vertexList = append(vertexList, vContainer)
		assert.NotNil(t, iterator.data)
		assert.Equal(t, 1, len(iterator.data))
		assert.Equal(t, 2, cap(iterator.data))
		assert.Equal(t, len(vertexList), len(iterator.data)+1)
	})

	t.Run(fmt.Sprintf("fully filled non-empty slice (len = 10, cap = 10)"), func(t *testing.T) {
		// Prepare vertex list that, representing the slice of children held by the
		//nolint:S1019
		var vertexList VertexList = make(VertexList, 10, 10) // we want to explicitly state the capacity here for clarity
		for i := 0; i < cap(vertexList); i++ {
			_v := NewVertexMock(fmt.Sprintf("v%d", i), 3, "C", 2)
			vertexList[i] = &vertexContainer{id: unittest.IdentifierFixture(), level: 3, vertex: _v}
		}

		// vertex iterator maintains a snapshot of the slice, where it is filled with 10 elements
		iterator := newVertexIterator(vertexList)

		// Emulating concurrent access, where new data is added by the forest
		vertexList = append(vertexList, vContainer)

		// we expect that vertexList was expanded, but the iterator's notion should be unchanged
		assert.NotNil(t, iterator.data)
		assert.Equal(t, 10, len(iterator.data))
		assert.Equal(t, 10, cap(iterator.data))
		assert.Equal(t, len(vertexList), len(iterator.data)+1)
	})
}

// Test_VertexIteratorConcurrencySafe verifies concurrent iteration
// We start with a forest (populated by `populateNewForest`) containing the following vertices:
//
//	       ↙-- [A]
//	··-[C] ←-- [D]
//
// Then vertices v0, v1, v2, etc are added concurrently here in the test
//
//		       ↙-- [A]
//		··-[C] ←-- [D]
//		       ↖-- [v0]
//		       ↖-- [v1]
//	             ⋮
//
// Before each addition, we create a vertex operator. Wile more and more vertices are added
// the constructed VertexIterators are checked to confirm they are unaffected, like they
// are operating on a snapshot taken at the time of their construction.
func Test_VertexIteratorConcurrencySafe(t *testing.T) {
	forest := newConcurrencySafeForestWrapper(populateNewForest(t))

	start := make(chan struct{})
	done1, done2 := make(chan struct{}), make(chan struct{})

	go func() { // Go Routine 1
		<-start
		for i := 0; i < 1000; i++ {
			// add additional child vertex of [C]
			var v Vertex = NewVertexMock(fmt.Sprintf("v%03d", i), 3, "C", 2)
			err := forest.VerifyAndAddVertex(&v)
			assert.NoError(t, err)
			time.Sleep(500 * time.Microsecond) // sleep 0.5ms -> in total 0.5s
		}
		close(done1)
	}()

	go func() { // Go Routine 2
		<-start
		var vertexIteratorCheckers []*vertexIteratorChecker

		for {
			select {
			case <-done1:
				close(done2)
				return
			default: // fallthrough
			}

			// the other thread is concurrently adding [C]. At all times, there should be at least
			iteratorChecker := forest.GetChildren(TestVertices["C"].VertexID())
			vertexIteratorCheckers = append(vertexIteratorCheckers, iteratorChecker)
			for _, checker := range vertexIteratorCheckers {
				checker.Check(t)
			}
			// sleep randomly up to 2ms, average 1ms, so we create only about half as much
			// iterators as new vertices are added.
			time.Sleep(time.Duration(rand.Intn(2000)) * time.Microsecond)
		}
	}()

	// start, and then wait for all go routines to finish. Routine 1 finishes after it added 1000
	// new vertices [v000], [v001], [v999] to the forest. Routine 2 will run until routine 1 has
	// finished. While routine 2 is running, it verifies that vertex additions to the forests
	// leve the iterators unchanged.
	close(start)

	// Wait up to 2 seconds, checking every 100 milliseconds
	bothDone := func() bool {
		select {
		case <-done1:
			select {
			case <-done2:
				return true
			default:
				return false
			}
		default:
			return false
		}
	}
	assert.Eventually(t, bothDone, 2*time.Second, 100*time.Millisecond, "Condition never became true")

}

// For testing only!
type concurrencySafeForestWrapper struct {
	forest *LevelledForest
	mu     sync.RWMutex
}

func newConcurrencySafeForestWrapper(f *LevelledForest) *concurrencySafeForestWrapper {
	return &concurrencySafeForestWrapper{forest: f}
}

func (w *concurrencySafeForestWrapper) VerifyAndAddVertex(vertex *Vertex) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.forest.VerifyVertex(*vertex)
	if err != nil {
		return err
	}
	w.forest.AddVertex(*vertex)
	return nil
}

// GetChildren returns an iterator the children of the specified vertex.
func (w *concurrencySafeForestWrapper) GetChildren(id flow.Identifier) *vertexIteratorChecker {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// creating non-concurrency safe iterator and memorizing its snapshot information for later testing
	unsafeIter := w.forest.GetChildren(id)
	numberChildren := w.forest.GetNumberOfChildren(id)
	sliceCapacity := cap(unsafeIter.data)

	// create wapper `VertexIteratorConcurrencySafe` and a check for verifying it
	safeIter := NewVertexIteratorConcurrencySafe(unsafeIter)
	return newVertexIteratorChecker(safeIter, numberChildren, sliceCapacity)
}

// For testing only!
type vertexIteratorChecker struct {
	safeIterator     *VertexIteratorConcurrencySafe
	expectedLength   int
	expectedCapacity int
}

func newVertexIteratorChecker(iter *VertexIteratorConcurrencySafe, expectedLength int, expectedCapacity int) *vertexIteratorChecker {
	return &vertexIteratorChecker{
		safeIterator:     iter,
		expectedLength:   expectedLength,
		expectedCapacity: expectedCapacity,
	}
}

func (c *vertexIteratorChecker) Check(t *testing.T) {
	// We are directly accessing the slice here backing the unsafe iterator without any concurrency
	// protection. This is expected to be fine, because the `data` slice is append only.
	unsafeIter := c.safeIterator.unsafeIter
	assert.NotNil(t, unsafeIter.data)
	assert.Equal(t, c.expectedLength, len(unsafeIter.data))
	assert.Equal(t, c.expectedCapacity, cap(unsafeIter.data))
}
