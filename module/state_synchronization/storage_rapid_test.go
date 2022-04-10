package state_synchronization_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	badgerDs "github.com/ipfs/go-ds-badger2"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/utils/unittest"
)

type edRecord struct {
	blockID     flow.Identifier
	blockHeight uint64
	rootID      flow.Identifier
}

type pendingEdRecord struct {
	record   *edRecord
	tracker  state_synchronization.StatusTracker
	progress int
}

type blobRecord struct {
	blob             blobs.Blob
	latestHeightSeen uint64
}

type blobTree struct {
	root   blobs.Blob
	levels [][]blobs.Blob
}

type StorageMachine struct {
	maxRange             uint64
	maxBlobTreeDepth     int
	maxBlobTreeLevelSize int

	storage *state_synchronization.Storage
	bstore  blockstore.Blockstore

	latestIncorporatedHeight uint64
	storedDataLowerBound     uint64
	pendingHeights           map[uint64]*pendingEdRecord
	completed                map[uint64]*edRecord
	blobTrees                map[uint64]*blobTree
	blobs                    map[cid.Cid]*blobRecord

	pruneGenerator    *rapid.Generator
	resetGenerator    *rapid.Generator
	blobTreeGenerator *rapid.Generator
	flowIDGenerator   *rapid.Generator
}

func (c *StorageMachine) Init(t *rapid.T) {
	c.maxRange = 20
	c.maxBlobTreeDepth = 5
	c.maxBlobTreeLevelSize = 10

	ds, err := badgerDs.NewDatastore("/tmp/badger", &badgerDs.DefaultOptions)
	require.NoError(t, err)
	c.storage = state_synchronization.NewStorage(ds.DB)
	c.bstore = blockstore.NewBlockstore(ds)

	c.latestIncorporatedHeight = 0
	c.storedDataLowerBound = 0
	c.pendingHeights = make(map[uint64]*pendingEdRecord)
	c.completed = make(map[uint64]*edRecord)
	c.blobTrees = make(map[uint64]*blobTree)
	c.blobs = make(map[cid.Cid]*blobRecord)

	pPrune := rapid.Float64Range(0, 50).Draw(t, "p_prune").(float64)
	pReset := rapid.Float64Range(0, 5).Draw(t, "p_reset").(float64)
	pResample := rapid.Float64Range(0, 5).Draw(t, "p_resample").(float64)

	c.pruneGenerator = rapid.Float64Range(0, 100).
		Map(func(n float64) bool {
			return n < pPrune
		})
	c.resetGenerator = rapid.Float64Range(0, 100).
		Map(func(n float64) bool {
			return n < pReset
		})
	blobGenerator := rapid.Float64Range(0, 100).
		Map(func(n float64) bool {
			return n < pResample
		}).
		Map(func(resample bool) blobs.Blob {
			if resample {
				var existingBlobs []blobs.Blob
				for _, blobRecord := range c.blobs {
					existingBlobs = append(existingBlobs, blobRecord.blob)
				}
				return rapid.SampledFrom(existingBlobs).Draw(t, "existing_blob").(blobs.Blob)
			}

			return rapid.SliceOf(rapid.Byte()).Map(func(blobData []byte) blobs.Blob {
				return blobs.NewBlob(blobData)
			}).Filter(func(blob blobs.Blob) bool {
				_, ok := c.blobs[blob.Cid()]
				return !ok
			}).Draw(t, "new_blob").(blobs.Blob)
		})
	blobTreeLevelGenerator := rapid.SliceOfN(blobGenerator, 1, c.maxBlobTreeLevelSize)
	c.blobTreeGenerator = rapid.SliceOfN(blobTreeLevelGenerator, 0, c.maxBlobTreeDepth).
		Map(func(levels [][]blobs.Blob) *blobTree {
			root := blobGenerator.Draw(t, "blob_tree_root").(blobs.Blob)
			return &blobTree{
				root:   root,
				levels: levels,
			}
		})
	c.flowIDGenerator = rapid.Custom(func(t *rapid.T) flow.Identifier {
		return unittest.IdentifierFixture()
	})
}

func (c *StorageMachine) DoSomething(t *rapid.T) {
	if c.resetGenerator.Draw(t, "reset").(bool) {
		for _, p := range c.pendingHeights {
			p.progress = 0
		}
	}

	if c.storedDataLowerBound < c.latestIncorporatedHeight && c.pruneGenerator.Draw(t, "prune").(bool) {
		pruneHeight := rapid.Uint64Range(c.storedDataLowerBound+1, c.latestIncorporatedHeight).
			Draw(t, "prune_height").(uint64)

		var expectedDeletedCids []cid.Cid
		for h := c.storedDataLowerBound + 1; h <= pruneHeight; h++ {
			blobTree := c.blobTrees[h]
			delete(c.blobTrees, h)

			if c.blobs[blobTree.root.Cid()].latestHeightSeen == h {
				delete(c.blobs, blobTree.root.Cid())
				expectedDeletedCids = append(expectedDeletedCids, blobTree.root.Cid())
			}

			for _, level := range blobTree.levels {
				for _, blob := range level {
					if c.blobs[blob.Cid()].latestHeightSeen == h {
						delete(c.blobs, blob.Cid())
						expectedDeletedCids = append(expectedDeletedCids, blob.Cid())
					}
				}
			}
		}

		actualDeletedCids, err := c.storage.Prune(pruneHeight)
		require.NoError(t, err)

		assert.ElementsMatch(t, expectedDeletedCids, actualDeletedCids)

		c.storedDataLowerBound = pruneHeight
	} else {
		h := rapid.Uint64Range(c.latestIncorporatedHeight+1, c.latestIncorporatedHeight+c.maxRange).
			Filter(func(height uint64) bool {
				_, ok := c.completed[height]
				return !ok
			}).Draw(t, "pending_height").(uint64)

		r, ok := c.pendingHeights[h]
		if !ok {
			r = &pendingEdRecord{
				record: &edRecord{
					blockID:     c.flowIDGenerator.Draw(t, "block_id").(flow.Identifier),
					blockHeight: h,
					rootID:      c.flowIDGenerator.Draw(t, "root_id").(flow.Identifier),
				},
				progress: 0,
			}
			c.pendingHeights[h] = r
			bTree := c.blobTreeGenerator.Draw(t, "blob_tree").(*blobTree)
			c.blobTrees[r.record.blockHeight] = bTree
		}

		if r.progress == 0 {
			tracker := c.storage.GetStatusTracker(r.record.blockID, r.record.blockHeight, r.record.rootID)
			r.tracker = tracker
			err := tracker.StartTransfer()
			require.NoError(t, err)
		} else {
			bTree := c.blobTrees[r.record.blockHeight]

			if r.progress == 2*(len(bTree.levels)+1) {
				latestIncorporatedHeight, err := r.tracker.FinishTransfer()
				require.NoError(t, err)

				delete(c.pendingHeights, r.record.blockHeight)
				c.completed[r.record.blockHeight] = r.record

				for {
					_, ok := c.completed[c.latestIncorporatedHeight+1]
					if !ok {
						break
					}

					delete(c.completed, c.latestIncorporatedHeight+1)
					c.latestIncorporatedHeight++
				}

				assert.Equal(t, c.latestIncorporatedHeight, latestIncorporatedHeight)
			} else if r.progress%2 == 0 {
				var cids []cid.Cid
				for _, blob := range bTree.levels[r.progress/2-1] {
					cids = append(cids, blob.Cid())
				}
				err := r.tracker.TrackBlobs(cids)
				require.NoError(t, err)
			} else {
				blobs := bTree.levels[r.progress/2-1]
				err := c.bstore.PutMany(context.Background(), blobs)
				require.NoError(t, err)
			}
		}

		r.progress++
	}
}

func (c *StorageMachine) Check(t *rapid.T) {
	latestIncorporatedHeight, err := c.storage.GetLatestIncorporatedHeight()
	require.NoError(t, err)
	assert.Equal(t, c.latestIncorporatedHeight, latestIncorporatedHeight)

	storedDataLowerBound, err := c.storage.GetStoredDataLowerBound()
	require.NoError(t, err)
	assert.Equal(t, c.storedDataLowerBound, storedDataLowerBound)

	err = c.storage.Check(state_synchronization.CheckOptions{
		Extended: true,
	})
	assert.NoError(t, err)

	trackedItems, pendingHeights, err := c.storage.LoadState()
	require.NoError(t, err)

	var numCompleted int
	var numInProgress int
	for _, item := range trackedItems {
		if item.Completed {
			numCompleted++
			if assert.Contains(t, c.completed, item.BlockHeight) {
				record := c.completed[item.BlockHeight]
				assert.Equal(t, record.blockHeight, item.BlockHeight)
				assert.Equal(t, record.blockID, item.BlockID)
				assert.Equal(t, record.rootID, item.RootID)
			}
		} else {
			numInProgress++
			if assert.Contains(t, c.pendingHeights, item.BlockHeight) {
				pendingRecord := c.pendingHeights[item.BlockHeight]
				assert.NotNil(t, pendingRecord.tracker)
				assert.NotNil(t, pendingRecord.record)
				assert.Equal(t, pendingRecord.record.blockHeight, item.BlockHeight)
				assert.Equal(t, pendingRecord.record.blockID, item.BlockID)
				assert.Equal(t, pendingRecord.record.rootID, item.RootID)
			}
		}
	}

	for _, h := range pendingHeights {
		assert.NotContains(t, c.pendingHeights, h)
	}

	assert.Len(t, c.pendingHeights, len(pendingHeights)+numInProgress)
	assert.Len(t, c.completed, numCompleted)
}

func TestStorage(t *testing.T) {
	rapid.Check(t, rapid.Run(&StorageMachine{}))
}
