package fetcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMissingCollectionQueue_CompleteBlockLifecycle tests the complete lifecycle of a block:
// 1. Initially IsHeightQueued returns false
// 2. After enqueuing, IsHeightQueued returns true
// 3. Enqueuing twice should error (not idempotent - prevents overwriting)
// 4. Receiving one collection doesn't complete the block, IsHeightQueued still returns true
// 5. Receiving all collections completes the block, IsHeightQueued still returns true (not yet indexed)
// 6. OnIndexedForBlock returns the callback
// 7. After indexing, IsHeightQueued returns false
// 8. OnReceivedCollection for that block returns false because the block has been removed
func TestMissingCollectionQueue_CompleteBlockLifecycle(t *testing.T) {
	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(100)

	// Create test collections
	collections := unittest.CollectionListFixture(3)
	collectionIDs := make([]flow.Identifier, len(collections))
	for i, col := range collections {
		collectionIDs[i] = col.ID()
	}

	callbackInvoked := false
	callback := func() {
		callbackInvoked = true
	}

	// Step 1: Initially IsHeightQueued returns false
	assert.False(t, mcq.IsHeightQueued(blockHeight), "height should not be queued initially")

	// Step 2: After enqueuing, IsHeightQueued returns true
	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	require.NoError(t, err)
	assert.True(t, mcq.IsHeightQueued(blockHeight), "height should be queued after enqueuing")

	// Step 3: Enqueuing twice should error (prevents overwriting)
	anotherCallback := func() {}
	err = mcq.EnqueueMissingCollections(blockHeight, collectionIDs, anotherCallback)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already enqueued")
	assert.Contains(t, err.Error(), "cannot overwrite existing job")

	// Step 4: Receiving one collection doesn't complete the block, IsHeightQueued still returns true
	collectionsReturned, heightReturned, complete := mcq.OnReceivedCollection(collections[0])
	assert.Nil(t, collectionsReturned, "should not return collections when block is not complete")
	assert.Equal(t, uint64(0), heightReturned, "should not return height when block is not complete")
	assert.False(t, complete, "block should not be complete with only one collection")
	assert.True(t, mcq.IsHeightQueued(blockHeight), "height should still be queued after receiving one collection")

	// Step 5: Receiving all collections completes the block, IsHeightQueued still returns true (not yet indexed)
	collectionsReturned, _, complete = mcq.OnReceivedCollection(collections[1])
	assert.Nil(t, collectionsReturned, "should not return collections when block is not complete")
	assert.False(t, complete, "block should not be complete with only two collections")
	assert.True(t, mcq.IsHeightQueued(blockHeight), "height should still be queued")

	// Receive the last collection - block should now be complete
	collectionsReturned, heightReturned, complete = mcq.OnReceivedCollection(collections[2])
	assert.NotNil(t, collectionsReturned, "should return collections when block is complete")
	assert.Equal(t, 3, len(collectionsReturned), "should return all 3 collections")
	assert.Equal(t, blockHeight, heightReturned, "should return correct block height")
	assert.True(t, complete, "block should be complete after receiving all collections")
	assert.True(t, mcq.IsHeightQueued(blockHeight), "height should still be queued (not yet indexed)")

	// Step 6: OnIndexedForBlock returns the callback
	returnedCallback, exists := mcq.OnIndexedForBlock(blockHeight)
	assert.NotNil(t, returnedCallback, "should return callback")
	assert.True(t, exists, "block should exist")
	assert.False(t, callbackInvoked, "callback should not be invoked yet")

	// Step 7: After indexing, IsHeightQueued returns false
	assert.False(t, mcq.IsHeightQueued(blockHeight), "height should not be queued after indexing")

	// Invoke the callback
	returnedCallback()
	assert.True(t, callbackInvoked, "callback should be invoked")

	// Step 8: OnReceivedCollection for that block returns false because the block has been removed
	collectionsReturned, heightReturned, complete = mcq.OnReceivedCollection(collections[0])
	assert.Nil(t, collectionsReturned, "should not return collections for removed block")
	assert.Equal(t, uint64(0), heightReturned, "should not return height for removed block")
	assert.False(t, complete, "should not indicate completion for removed block")
}

// TestMissingCollectionQueue_IndexBeforeBlockCompletion tests that OnIndexedForBlock
// can return the callback even before the block is complete (i.e., before all collections have been received).
func TestMissingCollectionQueue_IndexBeforeBlockCompletion(t *testing.T) {
	mcq := NewMissingCollectionQueue()
	blockHeight := uint64(200)

	// Create test collections
	collections := unittest.CollectionListFixture(3)
	collectionIDs := make([]flow.Identifier, len(collections))
	for i, col := range collections {
		collectionIDs[i] = col.ID()
	}

	callbackInvoked := false
	callback := func() {
		callbackInvoked = true
	}

	// Enqueue block with 3 collections
	err := mcq.EnqueueMissingCollections(blockHeight, collectionIDs, callback)
	require.NoError(t, err)
	assert.True(t, mcq.IsHeightQueued(blockHeight), "height should be queued")

	// Receive only one collection (block is not complete)
	collectionsReturned, heightReturned, complete := mcq.OnReceivedCollection(collections[0])
	assert.Nil(t, collectionsReturned, "should not return collections when block is not complete")
	assert.Equal(t, uint64(0), heightReturned, "should not return height when block is not complete")
	assert.False(t, complete, "block should not be complete with only one collection")
	assert.True(t, mcq.IsHeightQueued(blockHeight), "height should still be queued")

	// OnIndexedForBlock can return the callback even before the block is complete
	returnedCallback, exists := mcq.OnIndexedForBlock(blockHeight)
	assert.NotNil(t, returnedCallback, "should return callback even when block is not complete")
	assert.True(t, exists, "block should exist")
	assert.False(t, callbackInvoked, "callback should not be invoked yet")

	// After indexing, the block is removed from tracking
	assert.False(t, mcq.IsHeightQueued(blockHeight), "height should not be queued after indexing")

	// Verify that remaining collections cannot be received (block has been removed)
	collectionsReturned, heightReturned, complete = mcq.OnReceivedCollection(collections[1])
	assert.Nil(t, collectionsReturned, "should not return collections for removed block")
	assert.Equal(t, uint64(0), heightReturned, "should not return height for removed block")
	assert.False(t, complete, "should not indicate completion for removed block")

	collectionsReturned, _, complete = mcq.OnReceivedCollection(collections[2])
	assert.Nil(t, collectionsReturned, "should not return collections for removed block")
	assert.False(t, complete, "should not indicate completion for removed block")

	// Invoke the callback
	returnedCallback()
	assert.True(t, callbackInvoked, "callback should be invoked")
}
