package badger

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// TODO this method should not be included in the master branch.
func TestBackfillClusterBlockByReferenceHeightIndex(t *testing.T) {
	clusters := unittest.ClusterList(1, unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection)))
	me := clusters[0][0]

	// set up mocks for 2 epochs previous=9, current=10
	state := new(protocol.State)
	snap := new(protocol.Snapshot)

	currentEpoch := new(protocol.Epoch)
	currentCluster := new(protocol.Cluster)
	currentChainID := flow.ChainID("current-epoch")
	currentCluster.On("ChainID").Return(currentChainID)
	currentEpoch.On("Counter").Return(uint64(10), nil)
	currentEpoch.On("Clustering").Return(clusters, nil)
	currentEpoch.On("Cluster", uint(0)).Return(currentCluster, nil)

	prevEpoch := new(protocol.Epoch)
	prevCluster := new(protocol.Cluster)
	prevChainID := flow.ChainID("previous-epoch")
	prevCluster.On("ChainID").Return(prevChainID)
	prevEpoch.On("Counter").Return(uint64(9), nil)
	prevEpoch.On("Clustering").Return(clusters, nil)
	prevEpoch.On("Cluster", uint(0)).Return(prevCluster, nil)

	epochs := mocks.NewEpochQuery(t, 10, currentEpoch, prevEpoch)
	state.On("Final").Return(snap)
	snap.On("Epochs").Return(epochs)

	const (
		N_REF_BLOCKS     = 100
		N_CLUSTER_BLOCKS = 1_000
		BACKFILL_NUM     = 900
	)

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("all blocks within current epoch", func(t *testing.T) {
			// create reference blocks
			var refBlocks []*flow.Header
			for height := uint64(0); height < N_REF_BLOCKS; height++ {
				block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
				refBlocks = append(refBlocks, &block)
				err := db.Update(operation.InsertHeader(block.ID(), &block))
				require.NoError(t, err)
			}

			// keep track of expected index
			index := make(map[uint64][]flow.Identifier)

			// create cluster blocks, all within current epoch
			parentID := flow.ZeroID
			for height := uint64(0); height < N_CLUSTER_BLOCKS; height++ {
				block := unittest.ClusterBlockFixture()
				block.Header.ChainID = currentChainID
				block.Header.Height = height
				block.Header.ParentID = parentID
				refBlock := refBlocks[rand.Intn(len(refBlocks))]
				block.SetPayload(cluster.EmptyPayload(refBlock.ID()))
				blockID := block.ID()

				// don't index blocks that won't be backfilled
				if height >= N_CLUSTER_BLOCKS-BACKFILL_NUM {
					index[refBlock.Height] = append(index[refBlock.Height], blockID)
				}

				err := db.Update(func(tx *badger.Txn) error {
					err := procedure.InsertClusterBlock(&block)(tx)
					require.NoError(t, err)
					err = operation.IndexClusterBlockHeight(currentChainID, height, blockID)(tx)
					require.NoError(t, err)
					return nil
				})
				require.NoError(t, err)
				parentID = blockID
			}
			// set the finalized height
			err := db.Update(operation.InsertClusterFinalizedHeight(currentChainID, N_CLUSTER_BLOCKS-1))
			assert.NoError(t, err)

			log, _ := unittest.HookedLogger()
			err = BackfillClusterBlockByReferenceHeightIndex(db, log, me.NodeID, state, BACKFILL_NUM)
			assert.NoError(t, err)

			// make sure the index exists
			t.Run("BACKFILL_NUM many blocks should be indexed", func(t *testing.T) {
				var blockIDs []flow.Identifier
				err = db.View(operation.LookupClusterBlocksByReferenceHeightRange(0, N_REF_BLOCKS, &blockIDs))
				assert.NoError(t, err)
				assert.Len(t, blockIDs, BACKFILL_NUM)
			})

			// check a random sub-range of the index
			t.Run("check random sub-range of the index", func(t *testing.T) {
				min := uint64(rand.Intn(N_REF_BLOCKS))
				max := uint64(rand.Intn(N_REF_BLOCKS))
				if min > max {
					min, max = max, min
				}

				var actual []flow.Identifier
				err = db.View(operation.LookupClusterBlocksByReferenceHeightRange(min, max, &actual))
				assert.NoError(t, err)
				var expected []flow.Identifier
				for height := min; height <= max; height++ {
					expected = append(expected, index[height]...)
				}
				assert.Equal(t, len(expected), len(actual))
				assert.ElementsMatch(t, expected, actual)
			})
		})

		t.Run("should not attempt to re-index", func(t *testing.T) {
			log, hook := unittest.HookedLogger()
			err := BackfillClusterBlockByReferenceHeightIndex(db, log, me.NodeID, state, BACKFILL_NUM)
			assert.NoError(t, err)
			assert.True(t, strings.Contains(hook.Logs(), "cluster block reference height index already exists, exiting..."))
		})
	})

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("blocks split between current and previous epochs", func(t *testing.T) {
			// create reference blocks
			var refBlocks []*flow.Header
			for height := uint64(0); height < N_REF_BLOCKS; height++ {
				block := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
				refBlocks = append(refBlocks, &block)
				err := db.Update(operation.InsertHeader(block.ID(), &block))
				require.NoError(t, err)
			}

			// keep track of expected index
			index := make(map[uint64][]flow.Identifier)

			// create cluster blocks, split between current and previous epoch

			// current epoch cluster blocks
			parentID := flow.ZeroID
			for height := uint64(0); height <= N_CLUSTER_BLOCKS/2; height++ {
				block := unittest.ClusterBlockFixture()
				block.Header.ChainID = currentChainID
				block.Header.Height = height
				block.Header.ParentID = parentID
				refBlock := refBlocks[rand.Intn(len(refBlocks))]
				block.SetPayload(cluster.EmptyPayload(refBlock.ID()))
				blockID := block.ID()

				// genesis isn't backfilled because it is always empty
				if height > 0 {
					index[refBlock.Height] = append(index[refBlock.Height], blockID)
				}

				err := db.Update(func(tx *badger.Txn) error {
					err := procedure.InsertClusterBlock(&block)(tx)
					require.NoError(t, err)
					err = operation.IndexClusterBlockHeight(currentChainID, height, blockID)(tx)
					require.NoError(t, err)
					return nil
				})
				require.NoError(t, err)
				parentID = blockID
			}
			// set the finalized height
			err := db.Update(operation.InsertClusterFinalizedHeight(currentChainID, N_CLUSTER_BLOCKS/2))
			assert.NoError(t, err)

			// previous epoch cluster blocks
			parentID = flow.ZeroID
			for height := uint64(0); height <= N_CLUSTER_BLOCKS/2; height++ {
				block := unittest.ClusterBlockFixture()
				block.Header.ChainID = prevChainID
				block.Header.Height = height
				block.Header.ParentID = parentID
				refBlock := refBlocks[rand.Intn(len(refBlocks))]
				block.SetPayload(cluster.EmptyPayload(refBlock.ID()))
				blockID := block.ID()

				// don't index blocks that won't be backfilled
				if height > N_CLUSTER_BLOCKS-BACKFILL_NUM {
					index[refBlock.Height] = append(index[refBlock.Height], blockID)
				}

				err := db.Update(func(tx *badger.Txn) error {
					err := procedure.InsertClusterBlock(&block)(tx)
					require.NoError(t, err)
					err = operation.IndexClusterBlockHeight(prevChainID, height, blockID)(tx)
					require.NoError(t, err)
					return nil
				})
				require.NoError(t, err)
				parentID = blockID
			}
			// set the finalized height
			err = db.Update(operation.InsertClusterFinalizedHeight(prevChainID, N_CLUSTER_BLOCKS/2))
			assert.NoError(t, err)

			log, hook := unittest.HookedLogger()
			err = BackfillClusterBlockByReferenceHeightIndex(db, log, me.NodeID, state, BACKFILL_NUM)
			assert.NoError(t, err)
			fmt.Println("$$$", hook.Logs())

			// make sure the index exists
			t.Run("BACKFILL_NUM many blocks should be indexed", func(t *testing.T) {
				var blockIDs []flow.Identifier
				err = db.View(operation.LookupClusterBlocksByReferenceHeightRange(0, N_REF_BLOCKS, &blockIDs))
				assert.NoError(t, err)
				assert.Len(t, blockIDs, BACKFILL_NUM)
			})

			// check a random sub-range of the index
			t.Run("check random sub-range of the index", func(t *testing.T) {
				min := uint64(rand.Intn(N_REF_BLOCKS))
				max := uint64(rand.Intn(N_REF_BLOCKS))
				if min > max {
					min, max = max, min
				}

				var actual []flow.Identifier
				err = db.View(operation.LookupClusterBlocksByReferenceHeightRange(min, max, &actual))
				assert.NoError(t, err)
				var expected []flow.Identifier
				for height := min; height <= max; height++ {
					expected = append(expected, index[height]...)
				}
				assert.Equal(t, len(expected), len(actual))
				assert.ElementsMatch(t, expected, actual)
			})
		})

		t.Run("should not attempt to re-index", func(t *testing.T) {
			log, hook := unittest.HookedLogger()
			err := BackfillClusterBlockByReferenceHeightIndex(db, log, me.NodeID, state, BACKFILL_NUM)
			assert.NoError(t, err)
			assert.True(t, strings.Contains(hook.Logs(), "cluster block reference height index already exists, exiting..."))
		})
	})
}
