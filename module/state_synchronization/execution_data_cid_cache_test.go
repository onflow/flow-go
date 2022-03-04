package state_synchronization_test

import (
	"math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/utils/unittest"
)

func generateRandomLocation() (flow.Identifier, uint64, int) {
	return unittest.IdentifierFixture(), rand.Uint64(), rand.Int()
}

func TestCacheHit(t *testing.T) {
	cache := state_synchronization.NewExecutionDataCIDCache(10)

	blockID, blockHeight, chunkIndex := generateRandomLocation()
	var blobTree state_synchronization.BlobTree
	blobTree = append(blobTree, []cid.Cid{unittest.CidFixture(), unittest.CidFixture()}, []cid.Cid{unittest.CidFixture()})
	cache.Insert(blockID, blockHeight, chunkIndex, blobTree)

	for height, cids := range blobTree {
		for index, cid := range cids {
			record, err := cache.Get(cid)
			assert.NoError(t, err)
			assert.Equal(t, record.BlobTreeRecord.BlockHeight, blockHeight)
			assert.Equal(t, record.BlobTreeRecord.BlockID, blockID)
			assert.Equal(t, record.BlobTreeRecord.ChunkIndex, chunkIndex)
			assert.Equal(t, record.BlobTreeLocation.Height, uint(height))
			assert.Equal(t, record.BlobTreeLocation.Index, uint(index))
		}
	}
}

func TestCacheMiss(t *testing.T) {
	cache := state_synchronization.NewExecutionDataCIDCache(10)

	_, err := cache.Get(unittest.CidFixture())
	assert.ErrorIs(t, err, state_synchronization.ErrCacheMiss)
}

func TestCacheEviction(t *testing.T) {
	size := uint(5)
	cache := state_synchronization.NewExecutionDataCIDCache(size)

	var blockIDs []flow.Identifier
	var blockHeights []uint64
	var chunkIndexes []int
	var cids []cid.Cid

	for i := uint(0); i < 2*size; i++ {
		blockID, blockHeight, chunkIndex := generateRandomLocation()
		blockIDs = append(blockIDs, blockID)
		blockHeights = append(blockHeights, blockHeight)
		chunkIndexes = append(chunkIndexes, chunkIndex)
		root := unittest.CidFixture()
		cids = append(cids, root)
		cache.Insert(blockID, blockHeight, chunkIndex, [][]cid.Cid{{root}})

		expectedSize := i + 1
		if expectedSize > size {
			expectedSize = size
		}

		assert.Equal(t, cache.BlobRecords(), expectedSize)
		assert.Equal(t, cache.BlobTreeRecords(), expectedSize)

		for j, c := range cids {
			record, err := cache.Get(c)
			bi := blockIDs[j]
			bh := blockHeights[j]
			ci := chunkIndexes[j]

			if len(cids)-j <= int(size) {
				// cid should be in cache
				assert.NoError(t, err)
				assert.Equal(t, record.BlobTreeRecord.BlockHeight, bh)
				assert.Equal(t, record.BlobTreeRecord.BlockID, bi)
				assert.Equal(t, record.BlobTreeRecord.ChunkIndex, ci)
				assert.Equal(t, record.BlobTreeLocation.Height, uint(0))
				assert.Equal(t, record.BlobTreeLocation.Index, uint(0))
			} else {
				// cid should be evicted
				assert.ErrorIs(t, err, state_synchronization.ErrCacheMiss)
			}
		}
	}
}

func TestDuplicateCID(t *testing.T) {
	cache := state_synchronization.NewExecutionDataCIDCache(5)

	c := unittest.CidFixture()
	blockID1, blockHeight1, chunkIndex1 := generateRandomLocation()
	blockID2, blockHeight2, chunkIndex2 := generateRandomLocation()

	cache.Insert(blockID1, blockHeight1, chunkIndex1, [][]cid.Cid{{c}})
	cache.Insert(blockID2, blockHeight2, chunkIndex2, [][]cid.Cid{{c}})

	assert.Equal(t, cache.BlobRecords(), uint(1))
	assert.Equal(t, cache.BlobTreeRecords(), uint(2))

	record, err := cache.Get(c)
	assert.NoError(t, err)
	assert.Equal(t, record.BlobTreeRecord.BlockHeight, blockHeight2)
	assert.Equal(t, record.BlobTreeRecord.BlockID, blockHeight2)
	assert.Equal(t, record.BlobTreeLocation.Height, uint(0))
	assert.Equal(t, record.BlobTreeLocation.Index, uint(0))
}

func TestCacheEvictionWithDuplicateCID(t *testing.T) {
	cache := state_synchronization.NewExecutionDataCIDCache(2)

	sharedCid := unittest.CidFixture()

	blockID1, blockHeight1, chunkIndex1 := generateRandomLocation()
	var blobTree1 state_synchronization.BlobTree
	blobTree1 = append(blobTree1, []cid.Cid{sharedCid})

	blockID2, blockHeight2, chunkIndex2 := generateRandomLocation()
	var blobTree2 state_synchronization.BlobTree
	blobTree2 = append(blobTree2, []cid.Cid{sharedCid})

	blockID3, blockHeight3, chunkIndex3 := generateRandomLocation()
	var blobTree3 state_synchronization.BlobTree
	blobTree3 = append(blobTree3, []cid.Cid{unittest.CidFixture()})

	cache.Insert(blockID1, blockHeight1, chunkIndex1, blobTree1)
	cache.Insert(blockID2, blockHeight2, chunkIndex2, blobTree2)
	cache.Insert(blockID3, blockHeight3, chunkIndex3, blobTree3)

	assert.Equal(t, cache.BlobRecords(), uint(2))
	assert.Equal(t, cache.BlobTreeRecords(), uint(2))

	record, err := cache.Get(sharedCid)
	assert.NoError(t, err)
	assert.Equal(t, record.BlobTreeRecord.BlockHeight, blockHeight2)
	assert.Equal(t, record.BlobTreeRecord.BlockID, blockID2)
	assert.Equal(t, record.BlobTreeLocation.Height, uint(0))
	assert.Equal(t, record.BlobTreeLocation.Index, uint(0))
}
