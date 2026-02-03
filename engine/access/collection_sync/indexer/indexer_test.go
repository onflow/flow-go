package indexer

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// TestIndexCollectionsForBlock verifies the complete indexing workflow:
//
// Verification Process:
// 1. Initial State Check:
//   - Before indexing, GetMissingCollections returns all collections as missing
//   - This confirms the indexer correctly identifies unindexed collections
//
// 2. First Indexing:
//   - IndexCollectionsForBlock successfully stores and indexes all collections
//   - Collections are persisted to storage and can be retrieved by ID
//   - Transactions are stored and can be queried by transactions.ByID
//   - Transaction-to-collection index exists and can be queried by collections.LightByTransactionID
//   - Metrics are called for each collection (CollectionFinalized, CollectionExecuted)
//
// 3. Post-Indexing Verification:
//   - GetMissingCollections returns empty (no missing collections)
//   - This confirms indexing was successful and collections are now available
//
// 4. Idempotency Check:
//   - Indexing the same block again succeeds without error
//   - GetMissingCollections still returns empty after second indexing
//   - This confirms the operation is idempotent and safe to retry
func TestIndexCollectionsForBlock(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		collectionMetrics := modulemock.NewCollectionExecutedMetric(t)
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)

		indexer := NewBlockCollectionIndexer(
			collectionMetrics,
			lockManager,
			db,
			collections,
		)

		g := fixtures.NewGeneratorSuite()
		height := uint64(100)
		collectionList := g.Collections().List(3)

		// Create guarantees and block before indexing
		guarantees := make([]*flow.CollectionGuarantee, len(collectionList))
		for i, collection := range collectionList {
			guarantee := g.Guarantees().Fixture(fixtures.Guarantee.WithCollectionID(collection.ID()))
			guarantees[i] = guarantee
		}

		payload := g.Payloads().Fixture(
			fixtures.Payload.WithGuarantees(guarantees...),
		)
		block := g.Blocks().Fixture(
			fixtures.Block.WithPayload(payload),
		)

		// Step 1: Initial State Check - Before indexing, GetMissingCollections should return all collections as missing
		missing, err := indexer.GetMissingCollections(block)
		require.NoError(t, err)
		require.Len(t, missing, 3)
		require.Equal(t, guarantees[0].CollectionID, missing[0].CollectionID)
		require.Equal(t, guarantees[1].CollectionID, missing[1].CollectionID)
		require.Equal(t, guarantees[2].CollectionID, missing[2].CollectionID)

		// Step 2: First Indexing - Expect metrics to be called for each collection
		for _, collection := range collectionList {
			light := collection.Light()
			collectionMetrics.On("CollectionFinalized", light).Once()
			collectionMetrics.On("CollectionExecuted", light).Once()
		}

		err = indexer.IndexCollectionsForBlock(height, collectionList)
		require.NoError(t, err)

		// Step 2 (continued): Verify all collections are stored and can be retrieved
		for _, collection := range collectionList {
			stored, err := collections.ByID(collection.ID())
			require.NoError(t, err)
			require.Equal(t, collection.ID(), stored.ID())
		}

		// Step 2 (continued): Verify all transactions are stored and can be queried by ID
		for _, collection := range collectionList {
			for _, tx := range collection.Transactions {
				storedTx, err := transactions.ByID(tx.ID())
				require.NoError(t, err)
				require.Equal(t, tx.ID(), storedTx.ID())
			}
		}

		// Step 2 (continued): Verify transaction-to-collection index exists and can be queried
		for _, collection := range collectionList {
			for _, tx := range collection.Transactions {
				lightCollection, err := collections.LightByTransactionID(tx.ID())
				require.NoError(t, err)
				require.Equal(t, collection.ID(), lightCollection.ID())
			}
		}

		// Step 3: Post-Indexing Verification - After indexing, GetMissingCollections should return empty
		missing, err = indexer.GetMissingCollections(block)
		require.NoError(t, err)
		require.Len(t, missing, 0)

		collectionMetrics.AssertExpectations(t)

		// Step 4: Idempotency Check - Indexing again is idempotent and should not error
		// Set up expectations for the second call (metrics are still called even if collections already exist)
		for _, collection := range collectionList {
			light := collection.Light()
			collectionMetrics.On("CollectionFinalized", light).Once()
			collectionMetrics.On("CollectionExecuted", light).Once()
		}

		err = indexer.IndexCollectionsForBlock(height, collectionList)
		require.NoError(t, err)

		collectionMetrics.AssertExpectations(t)

		// Step 4 (continued): After second indexing, GetMissingCollections should still return empty
		missing, err = indexer.GetMissingCollections(block)
		require.NoError(t, err)
		require.Len(t, missing, 0)
	})
}
