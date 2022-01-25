package state_synchronization_test

import (
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
)

// test cache eviction (size)
// test replacing same CID but different block
// test that cache expiry doesn't expire CID's that replaced another one (ie check that the block ID checking logic works)

func TestCacheHit(t *testing.T) {
	cache := state_synchronization.NewExecutionDataCIDCache(10)

	header := unittest.BlockHeaderFixture()
	var blobTree state_synchronization.BlobTree
	blobTree = append(blobTree, []cid.Cid{unittest.CidFixture(), unittest.CidFixture()})
	blobTree = append(blobTree, []cid.Cid{unittest.CidFixture()})
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
	assert.Error(t, err)
}
