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

	ds      *badgerDs.Datastore
	storage *state_synchronization.Storage
	bstore  blockstore.Blockstore

	latestIncorporatedHeight uint64
	storedDataLowerBound     uint64
	pendingHeights           map[uint64]*pendingEdRecord
	completed                map[uint64]*edRecord
	blobTrees                map[uint64]*blobTree
	blobRecords              map[cid.Cid]*blobRecord
	blockIDs                 map[flow.Identifier]uint64
	blobs                    map[cid.Cid]blobs.Blob

	pruneGenerator    *rapid.Generator
	resetGenerator    *rapid.Generator
	blobTreeGenerator *rapid.Generator
	blockIDGenerator  *rapid.Generator
}

func (s *StorageMachine) Init(t *rapid.T) {
	s.maxRange = 20
	s.maxBlobTreeDepth = 5
	s.maxBlobTreeLevelSize = 32

	opts := badgerDs.DefaultOptions
	opts.BypassLockGuard = true
	ds, err := badgerDs.NewDatastore("/tmp/badger", &opts)
	require.NoError(t, err)
	s.ds = ds
	s.storage, err = state_synchronization.OpenStorage(ds.DB, 0)
	require.NoError(t, err)
	s.bstore = blockstore.NewBlockstore(ds)

	s.latestIncorporatedHeight = 0
	s.storedDataLowerBound = 0
	s.pendingHeights = make(map[uint64]*pendingEdRecord)
	s.completed = make(map[uint64]*edRecord)
	s.blobTrees = make(map[uint64]*blobTree)
	s.blobRecords = make(map[cid.Cid]*blobRecord)
	s.blockIDs = make(map[flow.Identifier]uint64)
	s.blobs = make(map[cid.Cid]blobs.Blob)

	pPrune := rapid.Float64Range(0, 50).Draw(t, "p_prune").(float64)
	s.pruneGenerator = rapid.Float64Range(0, 100).
		Map(func(n float64) bool {
			return n < pPrune
		})

	pReset := rapid.Float64Range(0, 5).Draw(t, "p_reset").(float64)
	s.resetGenerator = rapid.Float64Range(0, 100).
		Map(func(n float64) bool {
			return n < pReset
		})

	pResample := rapid.Float64Range(0, 5).Draw(t, "p_resample").(float64)
	newBlobGenerator := rapid.SliceOf(rapid.Byte()).Map(func(blobData []byte) blobs.Blob {
		return blobs.NewBlob(blobData)
	}).Filter(func(blob blobs.Blob) bool {
		_, ok := s.blobs[blob.Cid()]
		if !ok {
			s.blobs[blob.Cid()] = blob
			return true
		}
		return false
	})
	blobGenerator := rapid.Float64Range(0, 100).
		Map(func(n float64) bool {
			return n < pResample
		}).
		Map(func(resample bool) blobs.Blob {
			if resample && len(s.blobs) > 0 {
				var existingBlobs []blobs.Blob
				for _, blob := range s.blobs {
					existingBlobs = append(existingBlobs, blob)
				}
				return rapid.SampledFrom(existingBlobs).Draw(t, "existing_blob").(blobs.Blob)
			}

			newBlob := newBlobGenerator.Draw(t, "new_blob").(blobs.Blob)
			return newBlob
		})
	blobTreeLevelGenerator := rapid.SliceOfN(blobGenerator, 1, s.maxBlobTreeLevelSize)
	s.blobTreeGenerator = rapid.SliceOfN(blobTreeLevelGenerator, 0, s.maxBlobTreeDepth).
		Map(func(levels [][]blobs.Blob) *blobTree {
			root := newBlobGenerator.Draw(t, "blob_tree_root").(blobs.Blob)
			return &blobTree{
				root:   root,
				levels: levels,
			}
		})

	s.blockIDGenerator = rapid.SliceOfN(rapid.Byte(), flow.IdentifierLen, flow.IdentifierLen).
		Map(func(bytes []byte) flow.Identifier {
			return flow.HashToID(bytes)
		}).Filter(func(blockID flow.Identifier) bool {
		_, ok := s.blockIDs[blockID]
		return !ok
	})

}

func (s *StorageMachine) Cleanup() {
	if s.ds != nil {
		s.ds.DB.DropAll()
		s.ds.Close()
		s.ds = nil
	}
}

func (s *StorageMachine) DoSomething(t *rapid.T) {
	if s.resetGenerator.Draw(t, "reset").(bool) {
		for _, p := range s.pendingHeights {
			p.progress = 0
		}
	}

	if s.storedDataLowerBound < s.latestIncorporatedHeight && s.pruneGenerator.Draw(t, "prune").(bool) {
		pruneHeight := rapid.Uint64Range(s.storedDataLowerBound+1, s.latestIncorporatedHeight).
			Draw(t, "prune_height").(uint64)

		var expectedDeletedCids []cid.Cid
		for h := s.storedDataLowerBound + 1; h <= pruneHeight; h++ {
			blobTree := s.blobTrees[h]
			delete(s.blobTrees, h)

			cidsToDelete := make(map[cid.Cid]struct{})
			cidsToDelete[blobTree.root.Cid()] = struct{}{}
			for _, level := range blobTree.levels {
				for _, blob := range level {
					cidsToDelete[blob.Cid()] = struct{}{}
				}
			}

			for c := range cidsToDelete {
				if s.blobRecords[c].latestHeightSeen == h {
					delete(s.blobs, c)
					expectedDeletedCids = append(expectedDeletedCids, c)
				}
			}
		}

		actualDeletedCids, err := s.storage.Prune(pruneHeight)
		require.NoError(t, err)

		assert.ElementsMatch(t, expectedDeletedCids, actualDeletedCids)

		s.storedDataLowerBound = pruneHeight
	} else {
		h := rapid.Uint64Range(s.latestIncorporatedHeight+1, s.latestIncorporatedHeight+s.maxRange).
			Filter(func(height uint64) bool {
				_, ok := s.completed[height]
				return !ok
			}).Draw(t, "pending_height").(uint64)

		r, ok := s.pendingHeights[h]
		if !ok {
			bTree := s.blobTreeGenerator.Draw(t, "blob_tree").(*blobTree)
			s.blobTrees[h] = bTree

			rootID, err := flow.CidToId(bTree.root.Cid())
			require.NoError(t, err)

			blockID := s.blockIDGenerator.Draw(t, "block_id").(flow.Identifier)
			s.blockIDs[blockID] = h

			r = &pendingEdRecord{
				record: &edRecord{
					blockID:     blockID,
					blockHeight: h,
					rootID:      rootID,
				},
				progress: 0,
			}
			s.pendingHeights[h] = r
		}

		bTree := s.blobTrees[r.record.blockHeight]

		t.Logf("progress: %d", r.progress)
		if r.progress == 0 {
			tracker := s.storage.GetStatusTracker(r.record.blockID, r.record.blockHeight, r.record.rootID)
			r.tracker = tracker
			err := tracker.StartTransfer()
			require.NoError(t, err)
			s.updateBlobRecord(bTree.root, h)
		} else {
			if r.progress == 2*(len(bTree.levels)+1) {
				latestIncorporatedHeight, err := r.tracker.FinishTransfer()
				require.NoError(t, err)

				delete(s.pendingHeights, r.record.blockHeight)
				s.completed[r.record.blockHeight] = r.record

				for {
					_, ok := s.completed[s.latestIncorporatedHeight+1]
					if !ok {
						break
					}

					delete(s.completed, s.latestIncorporatedHeight+1)
					s.latestIncorporatedHeight++
				}

				assert.Equal(t, s.latestIncorporatedHeight, latestIncorporatedHeight)
			} else if r.progress%2 == 0 {
				level := bTree.levels[r.progress/2-1]
				var cids []cid.Cid
				for _, blob := range level {
					cids = append(cids, blob.Cid())
					s.updateBlobRecord(blob, h)
				}
				err := r.tracker.TrackBlobs(cids)
				require.NoError(t, err)
			} else if r.progress == 1 {
				err := s.bstore.Put(context.Background(), bTree.root)
				require.NoError(t, err)
			} else {
				blobs := bTree.levels[r.progress/2-1]
				err := s.bstore.PutMany(context.Background(), blobs)
				require.NoError(t, err)
			}
		}

		r.progress++
	}
}

func (s *StorageMachine) updateBlobRecord(blob blobs.Blob, height uint64) {
	br, ok := s.blobRecords[blob.Cid()]
	if !ok {
		br = &blobRecord{
			blob:             blob,
			latestHeightSeen: height,
		}
		s.blobRecords[blob.Cid()] = br
	} else if br.latestHeightSeen < height {
		br.latestHeightSeen = height
	}
}

func (s *StorageMachine) Check(t *rapid.T) {
	latestIncorporatedHeight, err := s.storage.GetLatestIncorporatedHeight()
	require.NoError(t, err)
	assert.Equal(t, s.latestIncorporatedHeight, latestIncorporatedHeight)

	storedDataLowerBound, err := s.storage.GetStoredDataLowerBound()
	require.NoError(t, err)
	assert.Equal(t, s.storedDataLowerBound, storedDataLowerBound)

	err = s.storage.Check(state_synchronization.CheckOptions{
		Extended: true,
	})
	assert.NoError(t, err)

	trackedItems, latestIncorporatedHeight, err := s.storage.LoadState()
	require.NoError(t, err)

	var numCompleted int
	var numInProgress int
	previousHeight := latestIncorporatedHeight
	var missingHeights []uint64
	for _, item := range trackedItems {
		if item.Completed {
			numCompleted++
			if assert.Contains(t, s.completed, item.BlockHeight) {
				record := s.completed[item.BlockHeight]
				assert.Equal(t, record.blockHeight, item.BlockHeight)
				assert.Equal(t, record.blockID, item.BlockID)
				assert.Equal(t, record.rootID, item.RootID)
			}
		} else {
			numInProgress++
			if assert.Contains(t, s.pendingHeights, item.BlockHeight) {
				pendingRecord := s.pendingHeights[item.BlockHeight]
				assert.NotNil(t, pendingRecord.tracker)
				assert.NotNil(t, pendingRecord.record)
				assert.Equal(t, pendingRecord.record.blockHeight, item.BlockHeight)
				assert.Equal(t, pendingRecord.record.blockID, item.BlockID)
				assert.Equal(t, pendingRecord.record.rootID, item.RootID)
			}
		}

		for h := previousHeight + 1; h < item.BlockHeight; h++ {
			missingHeights = append(missingHeights, h)
		}

		previousHeight = item.BlockHeight
	}

	for _, h := range missingHeights {
		assert.NotContains(t, s.pendingHeights, h)
	}

	assert.Len(t, s.pendingHeights, numInProgress)
	assert.Len(t, s.completed, numCompleted)
}

func TestStorage(t *testing.T) {
	rapid.Check(t, rapid.Run(&StorageMachine{}))
}
