package state_synchronization_test

import (
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCacheHit(t *testing.T) {
	cache := state_synchronization.NewExecutionDataCIDCache(10)

	header := unittest.BlockHeaderFixture()
	var blobTree state_synchronization.BlobTree
	blobTree = append(blobTree, []cid.Cid{unittest.CidFixture(), unittest.CidFixture()}, []cid.Cid{unittest.CidFixture()})
	cache.Insert(&header, blobTree)

	for height, cids := range blobTree {
		for index, cid := range cids {
			record, err := cache.Get(cid)
			assert.NoError(t, err)
			assert.Equal(t, record.BlobTreeRecord.BlockHeight, header.Height)
			assert.Equal(t, record.BlobTreeRecord.BlockID, header.ID())
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

	var headers []*flow.Header
	var cids []cid.Cid

	for i := uint(0); i < 2*size; i++ {
		header := unittest.BlockHeaderFixture()
		headers = append(headers, &header)
		root := unittest.CidFixture()
		cids = append(cids, root)
		cache.Insert(&header, [][]cid.Cid{{root}})

		expectedSize := i + 1
		if expectedSize > size {
			expectedSize = size
		}

		assert.Equal(t, cache.BlobRecords(), expectedSize)
		assert.Equal(t, cache.BlobTreeRecords(), expectedSize)

		for j, c := range cids {
			record, err := cache.Get(c)
			h := headers[j]

			if len(cids)-j <= int(size) {
				// cid should be in cache
				assert.NoError(t, err)
				assert.Equal(t, record.BlobTreeRecord.BlockHeight, h.Height)
				assert.Equal(t, record.BlobTreeRecord.BlockID, h.ID())
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
	header1 := unittest.BlockHeaderFixture()
	header2 := unittest.BlockHeaderFixture()

	cache.Insert(&header1, [][]cid.Cid{{c}})
	cache.Insert(&header2, [][]cid.Cid{{c}})

	assert.Equal(t, cache.BlobRecords(), uint(1))
	assert.Equal(t, cache.BlobTreeRecords(), uint(2))

	record, err := cache.Get(c)
	assert.NoError(t, err)
	assert.Equal(t, record.BlobTreeRecord.BlockHeight, header2.Height)
	assert.Equal(t, record.BlobTreeRecord.BlockID, header2.ID())
	assert.Equal(t, record.BlobTreeLocation.Height, uint(0))
	assert.Equal(t, record.BlobTreeLocation.Index, uint(0))
}

func TestCacheEvictionWithDuplicateCID(t *testing.T) {
	cache := state_synchronization.NewExecutionDataCIDCache(2)

	sharedCid := unittest.CidFixture()

	header1 := unittest.BlockHeaderFixture()
	var blobTree1 state_synchronization.BlobTree
	blobTree1 = append(blobTree1, []cid.Cid{sharedCid})

	header2 := unittest.BlockHeaderFixture()
	var blobTree2 state_synchronization.BlobTree
	blobTree2 = append(blobTree2, []cid.Cid{sharedCid})

	header3 := unittest.BlockHeaderFixture()
	var blobTree3 state_synchronization.BlobTree
	blobTree3 = append(blobTree3, []cid.Cid{unittest.CidFixture()})

	cache.Insert(&header1, blobTree1)
	cache.Insert(&header2, blobTree2)
	cache.Insert(&header3, blobTree3)

	assert.Equal(t, cache.BlobRecords(), uint(2))
	assert.Equal(t, cache.BlobTreeRecords(), uint(2))

	record, err := cache.Get(sharedCid)
	assert.NoError(t, err)
	assert.Equal(t, record.BlobTreeRecord.BlockHeight, header2.Height)
	assert.Equal(t, record.BlobTreeRecord.BlockID, header2.ID())
	assert.Equal(t, record.BlobTreeLocation.Height, uint(0))
	assert.Equal(t, record.BlobTreeLocation.Index, uint(0))
}
