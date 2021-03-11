package badger_test

import (
	"errors"

	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
)

func TestPayloadStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()

		index := badgerstorage.NewIndex(metrics, db)
		seals := badgerstorage.NewSeals(metrics, db)
		guarantees := badgerstorage.NewGuarantees(metrics, db)
		results := badgerstorage.NewExecutionResults(metrics, db)
		receipts := badgerstorage.NewExecutionReceipts(metrics, db, results)
		store := badgerstorage.NewPayloads(db, index, guarantees, seals, receipts)

		blockID := unittest.IdentifierFixture()
		expected := unittest.PayloadFixture()
		expected.Receipts = make([]*flow.ExecutionReceiptMeta, 0)
		expected.Results = make([]*flow.ExecutionResult, 0)

		// store payload
		err := store.Store(blockID, expected)
		require.NoError(t, err)

		// fetch payload
		payload, err := store.ByBlockID(blockID)
		require.NoError(t, err)
		require.Equal(t, expected, payload)
	})
}

func TestPayloadStoreERAlreadyIncluded(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()

		index := badgerstorage.NewIndex(metrics, db)
		seals := badgerstorage.NewSeals(metrics, db)
		guarantees := badgerstorage.NewGuarantees(metrics, db)
		results := badgerstorage.NewExecutionResults(metrics, db)
		receipts := badgerstorage.NewExecutionReceipts(metrics, db, results)
		store := badgerstorage.NewPayloads(db, index, guarantees, seals, receipts)

		result := unittest.ExecutionResultFixture()
		receipt1 := unittest.ExecutionReceiptFixture(unittest.WithResult(result))
		payload1 := unittest.PayloadFixture()
		payload1.Receipts = []*flow.ExecutionReceiptMeta{receipt1.Meta()}
		payload1.Results = []*flow.ExecutionResult{result}
		err := store.Store(unittest.IdentifierFixture(), payload1)
		require.NoError(t, err)

		receipt2 := unittest.ExecutionReceiptFixture(unittest.WithResult(result))
		payload2 := unittest.PayloadFixture()
		payload2.Receipts = []*flow.ExecutionReceiptMeta{receipt2.Meta()}
		err = store.Store(unittest.IdentifierFixture(), payload2)
		require.NoError(t, err)
	})
}

func TestPayloadRetreiveWithoutStore(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()

		index := badgerstorage.NewIndex(metrics, db)
		seals := badgerstorage.NewSeals(metrics, db)
		guarantees := badgerstorage.NewGuarantees(metrics, db)
		results := badgerstorage.NewExecutionResults(metrics, db)
		receipts := badgerstorage.NewExecutionReceipts(metrics, db, results)
		store := badgerstorage.NewPayloads(db, index, guarantees, seals, receipts)

		blockID := unittest.IdentifierFixture()

		_, err := store.ByBlockID(blockID)
		require.True(t, errors.Is(err, storage.ErrNotFound))
	})
}
