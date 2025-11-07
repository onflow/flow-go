package collections

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMissingCollectionQueue_EnqueueMissingCollections(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)
	collections := unittest.CollectionListFixture(3)
	collectionIDs := make([]flow.Identifier, len(collections))
	for i, col := range collections {
		collectionIDs[i] = col.ID()
	}

	callbackCalled := false
	callback := func() {
		callbackCalled = true
	}

	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	require.NoError(t, err)
	require.False(t, callbackCalled, "callback should not be called yet")
}

func TestMissingCollectionQueue_OnReceivedCollection_CompleteBlock(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)
	collections := unittest.CollectionListFixture(3)
	collectionIDs := make([]flow.Identifier, len(collections))
	for i, col := range collections {
		collectionIDs[i] = col.ID()
	}

	callbackCalled := false
	callback := func() {
		callbackCalled = true
	}

	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	require.NoError(t, err)

	// Receive first two collections - block should not be complete yet
	receivedCols, height, complete := mcq.OnReceivedCollection(collections[0])
	require.False(t, complete)
	require.Nil(t, receivedCols)
	require.Equal(t, uint64(0), height)

	receivedCols, height, complete = mcq.OnReceivedCollection(collections[1])
	require.False(t, complete)
	require.Nil(t, receivedCols)
	require.Equal(t, uint64(0), height)

	// Receive last collection - block should now be complete
	receivedCols, height, complete = mcq.OnReceivedCollection(collections[2])
	require.True(t, complete)
	require.NotNil(t, receivedCols)
	require.Equal(t, blockHeight, height)
	require.Len(t, receivedCols, 3)

	// Verify all collections are present
	receivedMap := make(map[flow.Identifier]*flow.Collection)
	for _, col := range receivedCols {
		receivedMap[col.ID()] = col
	}
	for _, expectedCol := range collections {
		require.Contains(t, receivedMap, expectedCol.ID())
		require.Equal(t, expectedCol, receivedMap[expectedCol.ID()])
	}

	require.False(t, callbackCalled, "callback should not be called until OnIndexedForBlock")
}

func TestMissingCollectionQueue_OnReceivedCollection_UntrackedCollection(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	collection := unittest.CollectionFixture(1)

	// Receive collection that was never enqueued
	receivedCols, height, complete := mcq.OnReceivedCollection(&collection)
	require.False(t, complete)
	require.Nil(t, receivedCols)
	require.Equal(t, uint64(0), height)
}

func TestMissingCollectionQueue_OnReceivedCollection_DuplicateCollection(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)
	collections := unittest.CollectionListFixture(2)
	collectionIDs := []flow.Identifier{collections[0].ID(), collections[1].ID()}

	callback := func() {}

	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	require.NoError(t, err)

	// Receive first collection
	receivedCols, height, complete := mcq.OnReceivedCollection(collections[0])
	require.False(t, complete)

	// Receive same collection again - should be ignored
	receivedCols, height, complete = mcq.OnReceivedCollection(collections[0])
	require.False(t, complete)
	require.Nil(t, receivedCols)
	require.Equal(t, uint64(0), height)
}

func TestMissingCollectionQueue_OnIndexedForBlock_InvokesCallback(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)
	collections := unittest.CollectionListFixture(2)
	collectionIDs := []flow.Identifier{collections[0].ID(), collections[1].ID()}

	callbackCalled := false
	var callbackMutex sync.Mutex
	callback := func() {
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		callbackCalled = true
	}

	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	require.NoError(t, err)

	// Receive all collections to complete the block
	_, _, complete := mcq.OnReceivedCollection(collections[0])
	require.False(t, complete)

	_, _, complete = mcq.OnReceivedCollection(collections[1])
	require.True(t, complete)

	// Index the block - should invoke callback
	mcq.OnIndexedForBlock(blockHeight)

	callbackMutex.Lock()
	require.True(t, callbackCalled, "callback should have been called")
	callbackMutex.Unlock()
}

func TestMissingCollectionQueue_OnIndexedForBlock_UntrackedBlock(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)

	// Try to index a block that was never enqueued - should be a no-op
	mcq.OnIndexedForBlock(blockHeight)
	// Should not panic
}

func TestMissingCollectionQueue_OnIndexedForBlock_AlreadyIndexed(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)
	collections := unittest.CollectionListFixture(1)
	collectionIDs := []flow.Identifier{collections[0].ID()}

	callbackCallCount := 0
	callback := func() {
		callbackCallCount++
	}

	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	require.NoError(t, err)

	// Receive collection to complete the block
	_, _, complete := mcq.OnReceivedCollection(collections[0])
	require.True(t, complete)

	// Index the block first time
	mcq.OnIndexedForBlock(blockHeight)
	require.Equal(t, 1, callbackCallCount)

	// Try to index again - should be a no-op
	mcq.OnIndexedForBlock(blockHeight)
	require.Equal(t, 1, callbackCallCount, "callback should not be called again")
}

func TestMissingCollectionQueue_EnqueueSameBlockHeight_ReplacesCallback(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)
	collections := unittest.CollectionListFixture(1)
	collectionIDs := []flow.Identifier{collections[0].ID()}

	firstCallbackCalled := false
	firstCallback := func() {
		firstCallbackCalled = true
	}

	secondCallbackCalled := false
	secondCallback := func() {
		secondCallbackCalled = true
	}

	// Enqueue first time
	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, firstCallback)
	require.NoError(t, err)

	// Enqueue same block height again - should replace callback
	err = mcq.EnqueueMissingCollections(blockHeight, collectionIDs, secondCallback)
	require.NoError(t, err)

	// Complete and index
	_, _, complete := mcq.OnReceivedCollection(collections[0])
	require.True(t, complete)

	mcq.OnIndexedForBlock(blockHeight)

	require.False(t, firstCallbackCalled, "first callback should not be called")
	require.True(t, secondCallbackCalled, "second callback should be called")
}

func TestMissingCollectionQueue_CollectionToBlock_OneToOneRelationship(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	collection := unittest.CollectionFixture(1)
	collectionID := collection.ID()

	blockHeight1 := uint64(100)
	blockHeight2 := uint64(200)

	callback1 := func() {}
	callback2 := func() {}

	// Assign collection to first block
	err := mcq.EnqueueMissingCollections(blockHeight1, []flow.Identifier{collectionID}, callback1)
	require.NoError(t, err)

	// Try to assign same collection to different block - should error
	err = mcq.EnqueueMissingCollections(blockHeight2, []flow.Identifier{collectionID}, callback2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already assigned")
}

func TestMissingCollectionQueue_MultipleBlocks_Independent(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()

	blockHeight1 := uint64(100)
	blockHeight2 := uint64(200)

	collections1 := unittest.CollectionListFixture(2)
	collections2 := unittest.CollectionListFixture(2)

	collectionIDs1 := []flow.Identifier{collections1[0].ID(), collections1[1].ID()}
	collectionIDs2 := []flow.Identifier{collections2[0].ID(), collections2[1].ID()}

	callback1Called := false
	callback1 := func() {
		callback1Called = true
	}

	callback2Called := false
	callback2 := func() {
		callback2Called = true
	}

	// Enqueue both blocks
	err := mcq.EnqueueMissingCollections(blockHeight1, collectionIDs1, callback1)
	require.NoError(t, err)

	err = mcq.EnqueueMissingCollections(blockHeight2, collectionIDs2, callback2)
	require.NoError(t, err)

	// Complete block 1
	_, _, complete := mcq.OnReceivedCollection(collections1[0])
	require.False(t, complete)

	receivedCols, height, complete := mcq.OnReceivedCollection(collections1[1])
	require.True(t, complete)
	require.Equal(t, blockHeight1, height)
	require.NotNil(t, receivedCols)

	mcq.OnIndexedForBlock(blockHeight1)
	require.True(t, callback1Called)
	require.False(t, callback2Called)

	// Complete block 2
	_, _, complete = mcq.OnReceivedCollection(collections2[0])
	require.False(t, complete)

	receivedCols, height, complete = mcq.OnReceivedCollection(collections2[1])
	require.True(t, complete)
	require.Equal(t, blockHeight2, height)
	require.NotNil(t, receivedCols)

	mcq.OnIndexedForBlock(blockHeight2)
	require.True(t, callback2Called)
}

func TestMissingCollectionQueue_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)
	collections := unittest.CollectionListFixture(10)
	collectionIDs := make([]flow.Identifier, len(collections))
	for i, col := range collections {
		collectionIDs[i] = col.ID()
	}

	callbackCalled := false
	var callbackMutex sync.Mutex
	callback := func() {
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		callbackCalled = true
	}

	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	require.NoError(t, err)

	// Receive collections concurrently
	var wg sync.WaitGroup
	for _, col := range collections {
		wg.Add(1)
		go func(c *flow.Collection) {
			defer wg.Done()
			mcq.OnReceivedCollection(c)
		}(col)
	}
	wg.Wait()

	// One of the receives should have completed the block
	// We can't predict which one, but we can check that eventually the block is complete
	// by trying to index it
	mcq.OnIndexedForBlock(blockHeight)

	callbackMutex.Lock()
	require.True(t, callbackCalled)
	callbackMutex.Unlock()
}

func TestMissingCollectionQueue_EmptyCollectionList(t *testing.T) {
	t.Parallel()

	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)

	callback := func() {
		// Callback for empty collection list
	}

	// Enqueue with no missing collections
	err := mcq.EnqueueMissingCollections(blockHeight, []flow.Identifier{}, callback)
	require.NoError(t, err)

	// Block should be immediately complete (no collections to wait for)
	// But we can't test this directly since OnReceivedCollection won't be called
	// The caller should handle this case by checking if all collections are already in storage
}

