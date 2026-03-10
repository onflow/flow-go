package extended

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	providermock "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider/mock"
	"github.com/onflow/flow-go/fvm/blueprints"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	executionmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// testSchedTxEntry is a simple storage.IteratorEntry implementation for tests.
type testSchedTxEntry struct {
	tx accessmodel.ScheduledTransaction
}

func (e testSchedTxEntry) Cursor() accessmodel.ScheduledTransactionCursor {
	return accessmodel.ScheduledTransactionCursor{ID: e.tx.ID}
}

func (e testSchedTxEntry) Value() (accessmodel.ScheduledTransaction, error) {
	return e.tx, nil
}

// makeScheduledTxIter builds a storage.ScheduledTransactionIterator from a slice of transactions.
func makeScheduledTxIter(txs []accessmodel.ScheduledTransaction) storage.ScheduledTransactionIterator {
	return func(yield func(storage.IteratorEntry[accessmodel.ScheduledTransaction, accessmodel.ScheduledTransactionCursor], error) bool) {
		for _, tx := range txs {
			if !yield(testSchedTxEntry{tx: tx}, nil) {
				return
			}
		}
	}
}

// signalerCtxExpectingThrow creates a context that asserts irrecoverable.Throw is called
// with a non-nil error. Returns the context and a verification function that must be called
// after the operation under test to confirm Throw was invoked.
func signalerCtxExpectingThrow(t *testing.T) (context.Context, func()) {
	t.Helper()
	thrown := make(chan error, 1)
	signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
		irrecoverable.NewMockSignalerContextWithCallback(t, context.Background(), func(err error) {
			select {
			case thrown <- err:
			default:
			}
		}))
	verify := func() {
		t.Helper()
		select {
		case err := <-thrown:
			require.Error(t, err, "irrecoverable.Throw must be called with a non-nil error")
		default:
			t.Fatal("expected irrecoverable.Throw to be called but it was not")
		}
	}
	return signalerCtx, verify
}

// TestTransactionHandlerContract tests the helper that extracts the contract ID from a
// transaction handler type identifier.
func TestTransactionHandlerContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    string
		expectedErr bool
	}{
		{
			name:     "standard type identifier",
			input:    "A.1654653399040a61.MyScheduler.Handler",
			expected: "A.1654653399040a61.MyScheduler",
		},
		{
			name:     "deeply nested type identifier returns A.address.Contract prefix only",
			input:    "A.1654653399040a61.MyScheduler.SubModule.Handler",
			expected: "A.1654653399040a61.MyScheduler",
		},
		{
			name:     "exactly three parts is valid",
			input:    "A.1654653399040a61.MyScheduler",
			expected: "A.1654653399040a61.MyScheduler",
		},
		{
			name:        "fewer than three parts returns error",
			input:       "SomeContract.Handler",
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contractID, err := transactionHandlerContract(tt.input)
			if tt.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, contractID)
		})
	}
}

// TestScheduledTransactionFilter tests that ScheduledTransactionFilter.Filter produces a
// predicate that correctly matches or rejects scheduled transactions for each filter field,
// and for combined multi-field filters.
func TestScheduledTransactionFilter(t *testing.T) {
	t.Parallel()

	ownerAddr := unittest.RandomAddressFixture()
	otherAddr := unittest.RandomAddressFixture()
	handlerTypeID := "A.1654653399040a61.MyScheduler.Handler"
	otherTypeID := "A.0000000000000001.OtherScheduler.Handler"

	tx := &accessmodel.ScheduledTransaction{
		ID:                               42,
		Status:                           accessmodel.ScheduledTxStatusScheduled,
		Priority:                         5,
		Timestamp:                        1000,
		TransactionHandlerOwner:          ownerAddr,
		TransactionHandlerTypeIdentifier: handlerTypeID,
		TransactionHandlerUUID:           99,
	}

	t.Run("empty filter returns nil", func(t *testing.T) {
		filter := ScheduledTransactionFilter{}
		assert.Nil(t, filter.Filter())
	})

	t.Run("status filter matches", func(t *testing.T) {
		filter := ScheduledTransactionFilter{
			Statuses: []accessmodel.ScheduledTransactionStatus{accessmodel.ScheduledTxStatusScheduled},
		}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("status filter rejects mismatch", func(t *testing.T) {
		filter := ScheduledTransactionFilter{
			Statuses: []accessmodel.ScheduledTransactionStatus{accessmodel.ScheduledTxStatusExecuted},
		}
		assert.False(t, filter.Filter()(tx))
	})

	t.Run("status filter matches when one of multiple statuses matches", func(t *testing.T) {
		filter := ScheduledTransactionFilter{
			Statuses: []accessmodel.ScheduledTransactionStatus{
				accessmodel.ScheduledTxStatusExecuted,
				accessmodel.ScheduledTxStatusScheduled,
			},
		}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("priority filter matches", func(t *testing.T) {
		p := accessmodel.ScheduledTransactionPriority(5)
		filter := ScheduledTransactionFilter{Priority: &p}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("priority filter rejects mismatch", func(t *testing.T) {
		p := accessmodel.ScheduledTransactionPriority(10)
		filter := ScheduledTransactionFilter{Priority: &p}
		assert.False(t, filter.Filter()(tx))
	})

	t.Run("start time inclusive lower bound matches equal timestamp", func(t *testing.T) {
		start := uint64(1000)
		filter := ScheduledTransactionFilter{StartTime: &start}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("start time rejects timestamp below bound", func(t *testing.T) {
		start := uint64(1001)
		filter := ScheduledTransactionFilter{StartTime: &start}
		assert.False(t, filter.Filter()(tx))
	})

	t.Run("end time inclusive upper bound matches equal timestamp", func(t *testing.T) {
		end := uint64(1000)
		filter := ScheduledTransactionFilter{EndTime: &end}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("end time rejects timestamp above bound", func(t *testing.T) {
		end := uint64(999)
		filter := ScheduledTransactionFilter{EndTime: &end}
		assert.False(t, filter.Filter()(tx))
	})

	t.Run("start and end time window matches timestamp within range", func(t *testing.T) {
		start := uint64(900)
		end := uint64(1100)
		filter := ScheduledTransactionFilter{StartTime: &start, EndTime: &end}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("handler owner filter matches", func(t *testing.T) {
		filter := ScheduledTransactionFilter{TransactionHandlerOwner: &ownerAddr}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("handler owner filter rejects mismatch", func(t *testing.T) {
		filter := ScheduledTransactionFilter{TransactionHandlerOwner: &otherAddr}
		assert.False(t, filter.Filter()(tx))
	})

	t.Run("handler type ID filter matches", func(t *testing.T) {
		filter := ScheduledTransactionFilter{TransactionHandlerTypeID: &handlerTypeID}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("handler type ID filter rejects mismatch", func(t *testing.T) {
		filter := ScheduledTransactionFilter{TransactionHandlerTypeID: &otherTypeID}
		assert.False(t, filter.Filter()(tx))
	})

	t.Run("handler UUID filter matches", func(t *testing.T) {
		uuid := uint64(99)
		filter := ScheduledTransactionFilter{TransactionHandlerUUID: &uuid}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("handler UUID filter rejects mismatch", func(t *testing.T) {
		uuid := uint64(100)
		filter := ScheduledTransactionFilter{TransactionHandlerUUID: &uuid}
		assert.False(t, filter.Filter()(tx))
	})

	t.Run("combined filters all match", func(t *testing.T) {
		p := accessmodel.ScheduledTransactionPriority(5)
		start := uint64(1000)
		end := uint64(1000)
		uuid := uint64(99)
		filter := ScheduledTransactionFilter{
			Statuses:                 []accessmodel.ScheduledTransactionStatus{accessmodel.ScheduledTxStatusScheduled},
			Priority:                 &p,
			StartTime:                &start,
			EndTime:                  &end,
			TransactionHandlerOwner:  &ownerAddr,
			TransactionHandlerTypeID: &handlerTypeID,
			TransactionHandlerUUID:   &uuid,
		}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("combined filters reject on single mismatch", func(t *testing.T) {
		p := accessmodel.ScheduledTransactionPriority(5)
		wrongUUID := uint64(100) // mismatch
		filter := ScheduledTransactionFilter{
			Priority:               &p,
			TransactionHandlerUUID: &wrongUUID,
		}
		assert.False(t, filter.Filter()(tx))
	})
}

// TestScheduledTransactionsBackend_GetScheduledTransaction tests all code paths for the
// GetScheduledTransaction method, including storage error mappings and all expand combinations.
func TestScheduledTransactionsBackend_GetScheduledTransaction(t *testing.T) {
	t.Parallel()

	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0
	defaultConfig := DefaultConfig()

	t.Run("happy path: returns transaction without expand", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		expectedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusScheduled}
		store.On("ByID", uint64(1)).Return(expectedTx, nil).Once()

		result, err := backend.GetScheduledTransaction(
			context.Background(), 1, ScheduledTransactionExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, expectedTx, *result)
		assert.Nil(t, result.Transaction)
		assert.Nil(t, result.Result)
		assert.Nil(t, result.HandlerContract)
	})

	t.Run("ErrNotFound maps to codes.NotFound", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		store.On("ByID", uint64(99)).Return(accessmodel.ScheduledTransaction{}, storage.ErrNotFound).Once()

		_, err := backend.GetScheduledTransaction(
			context.Background(), 99, ScheduledTransactionExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("ErrNotBootstrapped maps to codes.FailedPrecondition", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		store.On("ByID", uint64(1)).Return(accessmodel.ScheduledTransaction{}, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetScheduledTransaction(
			context.Background(), 1, ScheduledTransactionExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("unexpected storage error triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		storageErr := fmt.Errorf("unexpected disk failure")
		store.On("ByID", uint64(1)).Return(accessmodel.ScheduledTransaction{}, storageErr).Once()

		expectedErr := fmt.Errorf("failed to get scheduled transaction: %w", storageErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr))

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1, ScheduledTransactionExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
	})

	// expand is no-op for scheduled status (no block lookups, no result/transaction populated)
	t.Run("expand is no-op for scheduled status", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		tx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusScheduled}
		store.On("ByID", uint64(1)).Return(tx, nil).Once()

		// expand options set but status is Scheduled: no storage lookups expected
		result, err := backend.GetScheduledTransaction(
			context.Background(), 1,
			ScheduledTransactionExpandOptions{Result: true, Transaction: true},
			defaultEncoding,
		)
		require.NoError(t, err)
		assert.Nil(t, result.Transaction)
		assert.Nil(t, result.Result)
	})

	// for cancelled status: block timestamps are looked up, but result/transaction are not expanded
	t.Run("expand is no-op for result/transaction on cancelled status", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)

		cancelledTxID := unittest.IdentifierFixture()
		cancelledCollection := &flow.LightCollection{Transactions: []flow.Identifier{cancelledTxID}}
		cancelledBlock := unittest.BlockFixture()

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:      defaultConfig,
				collections: mockCollections,
				blocks:      mockBlocks,
			},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		tx := accessmodel.ScheduledTransaction{
			ID:                     1,
			Status:                 accessmodel.ScheduledTxStatusCancelled,
			CancelledTransactionID: cancelledTxID,
		}
		store.On("ByID", uint64(1)).Return(tx, nil).Once()
		// CancelledTransactionID is a user tx: not in scheduled lookup, falls back to collection
		scheduledTxLookup.On("BlockIDByTransactionID", cancelledTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", cancelledTxID).Return(cancelledCollection, nil).Once()
		mockBlocks.On("ByCollectionID", cancelledCollection.ID()).Return(cancelledBlock, nil).Once()

		result, err := backend.GetScheduledTransaction(
			context.Background(), 1,
			ScheduledTransactionExpandOptions{Result: true, Transaction: true},
			defaultEncoding,
		)
		require.NoError(t, err)
		assert.Equal(t, cancelledBlock.Timestamp, result.CompletedAt)
		assert.Nil(t, result.Transaction)
		assert.Nil(t, result.Result)
	})

	// expand result works for executed and failed transactions
	for _, status := range []accessmodel.ScheduledTransactionStatus{accessmodel.ScheduledTxStatusExecuted, accessmodel.ScheduledTxStatusFailed} {
		t.Run(fmt.Sprintf("expand result on %s transaction", status), func(t *testing.T) {
			store := storagemock.NewScheduledTransactionsIndexReader(t)
			scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
			mockHeaders := storagemock.NewHeaders(t)
			mockProvider := providermock.NewTransactionProvider(t)

			backend := NewScheduledTransactionsBackend(
				unittest.Logger(),
				&backendBase{
					config:               defaultConfig,
					headers:              mockHeaders,
					transactionsProvider: mockProvider,
				},
				flow.Mainnet, store, scheduledTxLookup, nil, nil,
			)

			txID := unittest.IdentifierFixture()
			blockHeader := unittest.BlockHeaderFixture()
			blockID := blockHeader.ID()

			storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: status, ExecutedTransactionID: txID}
			expectedResult := &accessmodel.TransactionResult{
				TransactionID: txID,
				BlockID:       blockID,
				Status:        flow.TransactionStatusSealed,
			}

			store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
			scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(blockID, nil).Once()
			mockHeaders.On("ByBlockID", blockID).Return(blockHeader, nil).Once()
			mockProvider.On("TransactionResult", mocktestify.Anything, blockHeader, txID, mocktestify.Anything, defaultEncoding).
				Return(expectedResult, nil).Once()

			result, err := backend.GetScheduledTransaction(
				context.Background(), 1,
				ScheduledTransactionExpandOptions{Result: true},
				defaultEncoding,
			)
			require.NoError(t, err)
			require.NotNil(t, result.Result)
			assert.Equal(t, expectedResult, result.Result)
			assert.Nil(t, result.Transaction)
		})
	}

	// expand tx body works for executed and failed transactions
	for _, status := range []accessmodel.ScheduledTransactionStatus{accessmodel.ScheduledTxStatusExecuted, accessmodel.ScheduledTxStatusFailed} {
		t.Run(fmt.Sprintf("expand transaction body on %s transaction", status), func(t *testing.T) {
			store := storagemock.NewScheduledTransactionsIndexReader(t)
			scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
			mockHeaders := storagemock.NewHeaders(t)

			sysCollections, err := systemcollection.NewVersioned(
				flow.Mainnet.Chain(), systemcollection.Default(flow.Mainnet),
			)
			require.NoError(t, err)

			backend := NewScheduledTransactionsBackend(
				unittest.Logger(),
				&backendBase{
					config:            defaultConfig,
					headers:           mockHeaders,
					systemCollections: sysCollections,
				},
				flow.Mainnet, store, scheduledTxLookup, nil, nil,
			)

			// construct the expected tx body using the same path the production code will use
			const scheduledTxID = uint64(42)
			const executionEffort = uint64(1000)
			expectedTxBody, err := blueprints.ExecuteCallbacksTransaction(flow.Mainnet.Chain(), scheduledTxID, executionEffort)
			require.NoError(t, err)
			txID := expectedTxBody.ID()

			blockHeader := unittest.BlockHeaderFixture()
			blockID := blockHeader.ID()

			storedTx := accessmodel.ScheduledTransaction{
				ID: scheduledTxID, Status: status,
				ExecutedTransactionID: txID, ExecutionEffort: executionEffort,
			}

			store.On("ByID", scheduledTxID).Return(storedTx, nil).Once()
			scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(blockID, nil).Once()
			mockHeaders.On("ByBlockID", blockID).Return(blockHeader, nil).Once()

			result, err := backend.GetScheduledTransaction(
				context.Background(), scheduledTxID,
				ScheduledTransactionExpandOptions{Transaction: true},
				defaultEncoding,
			)
			require.NoError(t, err)
			require.NotNil(t, result.Transaction)
			assert.Equal(t, txID, result.Transaction.ID())
			assert.Nil(t, result.Result)
		})
	}

	t.Run("expand handler contract", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		mockState := protocolmock.NewState(t)
		mockSnapshot := protocolmock.NewSnapshot(t)
		mockScriptExecutor := executionmock.NewScriptExecutor(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, mockState, mockScriptExecutor,
		)

		handlerOwner := unittest.RandomAddressFixture()
		handlerTypeID := "A.1654653399040a61.MyScheduler.Handler"
		contractID := "A.1654653399040a61.MyScheduler"
		contractBody := []byte("pub contract MyScheduler {}")
		sealedHeader := unittest.BlockHeaderFixture()

		storedTx := accessmodel.ScheduledTransaction{
			ID:                               1,
			Status:                           accessmodel.ScheduledTxStatusScheduled,
			TransactionHandlerOwner:          handlerOwner,
			TransactionHandlerTypeIdentifier: handlerTypeID,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		mockState.On("Sealed").Return(mockSnapshot).Once()
		mockSnapshot.On("Head").Return(sealedHeader, nil).Once()
		mockScriptExecutor.On("GetAccountAtBlockHeight", mocktestify.Anything, handlerOwner, sealedHeader.Height).
			Return(&flow.Account{
				Contracts: map[string][]byte{contractID: contractBody},
			}, nil).Once()

		result, err := backend.GetScheduledTransaction(
			context.Background(), 1,
			ScheduledTransactionExpandOptions{HandlerContract: true},
			defaultEncoding,
		)
		require.NoError(t, err)
		require.NotNil(t, result.HandlerContract)
		assert.Equal(t, contractID, result.HandlerContract.Identifier)
		assert.Equal(t, string(contractBody), result.HandlerContract.Body)
	})

	t.Run("created tx block not yet available returns zero CreatedAt", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:      defaultConfig,
				collections: mockCollections,
				blocks:      mockBlocks,
			},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		createdTxID := unittest.IdentifierFixture()
		createdCollection := &flow.LightCollection{Transactions: []flow.Identifier{createdTxID}}
		storedTx := accessmodel.ScheduledTransaction{
			ID:                   1,
			Status:               accessmodel.ScheduledTxStatusScheduled,
			CreatedTransactionID: createdTxID,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", createdTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", createdTxID).Return(createdCollection, nil).Once()
		// block not yet indexed by FinalizedBlockProcessor
		mockBlocks.On("ByCollectionID", createdCollection.ID()).Return((*flow.Block)(nil), storage.ErrNotFound).Once()

		result, err := backend.GetScheduledTransaction(context.Background(), 1, ScheduledTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		assert.Zero(t, result.CreatedAt)
	})

	t.Run("cancelled tx block not yet available returns zero CompletedAt", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:      defaultConfig,
				collections: mockCollections,
				blocks:      mockBlocks,
			},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		cancelledTxID := unittest.IdentifierFixture()
		cancelledCollection := &flow.LightCollection{Transactions: []flow.Identifier{cancelledTxID}}
		storedTx := accessmodel.ScheduledTransaction{
			ID:                     1,
			Status:                 accessmodel.ScheduledTxStatusCancelled,
			CancelledTransactionID: cancelledTxID,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", cancelledTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", cancelledTxID).Return(cancelledCollection, nil).Once()
		mockBlocks.On("ByCollectionID", cancelledCollection.ID()).Return((*flow.Block)(nil), storage.ErrNotFound).Once()

		result, err := backend.GetScheduledTransaction(context.Background(), 1, ScheduledTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		assert.Zero(t, result.CompletedAt)
	})

	t.Run("BlockIDByTransactionID error during populateBlockTimestamps triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted, ExecutedTransactionID: txID}
		lookupErr := fmt.Errorf("lookup error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(flow.Identifier{}, lookupErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1, ScheduledTransactionExpandOptions{Result: true}, defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("BlockIDByTransactionID error during expand triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted, ExecutedTransactionID: txID}
		blockLookupErr := fmt.Errorf("block lookup error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(flow.Identifier{}, blockLookupErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1, ScheduledTransactionExpandOptions{Result: true}, defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("ByBlockID error during expand triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig, headers: mockHeaders},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted, ExecutedTransactionID: txID}
		headerErr := fmt.Errorf("header lookup error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(blockID, nil).Once()
		mockHeaders.On("ByBlockID", blockID).Return((*flow.Header)(nil), headerErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1, ScheduledTransactionExpandOptions{Result: true}, defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("transaction ID mismatch during expand triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)

		sysCollections, err := systemcollection.NewVersioned(
			flow.Mainnet.Chain(), systemcollection.Default(flow.Mainnet),
		)
		require.NoError(t, err)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:            defaultConfig,
				headers:           mockHeaders,
				systemCollections: sysCollections,
			},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		// ExecutedTransactionID does NOT match the tx body constructed by systemCollections
		mismatchedTxID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()
		blockID := blockHeader.ID()
		storedTx := accessmodel.ScheduledTransaction{
			ID: 42, Status: accessmodel.ScheduledTxStatusExecuted,
			ExecutedTransactionID: mismatchedTxID, ExecutionEffort: 1000,
		}

		store.On("ByID", uint64(42)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", mismatchedTxID).Return(blockID, nil).Once()
		mockHeaders.On("ByBlockID", blockID).Return(blockHeader, nil).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err = backend.GetScheduledTransaction(
			signalerCtx, 42, ScheduledTransactionExpandOptions{Transaction: true}, defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("TransactionResult error during expand triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		mockProvider := providermock.NewTransactionProvider(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:               defaultConfig,
				headers:              mockHeaders,
				transactionsProvider: mockProvider,
			},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()
		blockID := blockHeader.ID()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted, ExecutedTransactionID: txID}
		resultErr := fmt.Errorf("result lookup error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(blockID, nil).Once()
		mockHeaders.On("ByBlockID", blockID).Return(blockHeader, nil).Once()
		mockProvider.On("TransactionResult", mocktestify.Anything, blockHeader, txID, mocktestify.Anything, defaultEncoding).
			Return(nil, resultErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1, ScheduledTransactionExpandOptions{Result: true}, defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("expandHandlerContract: state.Sealed().Head() error triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		mockState := protocolmock.NewState(t)
		mockSnapshot := protocolmock.NewSnapshot(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, mockState, nil,
		)

		storedTx := accessmodel.ScheduledTransaction{
			ID:                               1,
			Status:                           accessmodel.ScheduledTxStatusScheduled,
			TransactionHandlerOwner:          unittest.RandomAddressFixture(),
			TransactionHandlerTypeIdentifier: "A.1654653399040a61.MyScheduler.Handler",
		}
		headErr := fmt.Errorf("sealed head error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		mockState.On("Sealed").Return(mockSnapshot).Once()
		mockSnapshot.On("Head").Return((*flow.Header)(nil), headErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1,
			ScheduledTransactionExpandOptions{HandlerContract: true},
			defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("expandHandlerContract: scriptExecutor error triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		mockState := protocolmock.NewState(t)
		mockSnapshot := protocolmock.NewSnapshot(t)
		mockScriptExecutor := executionmock.NewScriptExecutor(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, mockState, mockScriptExecutor,
		)

		handlerOwner := unittest.RandomAddressFixture()
		sealedHeader := unittest.BlockHeaderFixture()
		execErr := fmt.Errorf("script executor error")

		storedTx := accessmodel.ScheduledTransaction{
			ID:                               1,
			Status:                           accessmodel.ScheduledTxStatusScheduled,
			TransactionHandlerOwner:          handlerOwner,
			TransactionHandlerTypeIdentifier: "A.1654653399040a61.MyScheduler.Handler",
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		mockState.On("Sealed").Return(mockSnapshot).Once()
		mockSnapshot.On("Head").Return(sealedHeader, nil).Once()
		mockScriptExecutor.On("GetAccountAtBlockHeight", mocktestify.Anything, handlerOwner, sealedHeader.Height).
			Return(nil, execErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1,
			ScheduledTransactionExpandOptions{HandlerContract: true},
			defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("expandHandlerContract: contract not found in account triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		mockState := protocolmock.NewState(t)
		mockSnapshot := protocolmock.NewSnapshot(t)
		mockScriptExecutor := executionmock.NewScriptExecutor(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, mockState, mockScriptExecutor,
		)

		handlerOwner := unittest.RandomAddressFixture()
		sealedHeader := unittest.BlockHeaderFixture()

		storedTx := accessmodel.ScheduledTransaction{
			ID:                               1,
			Status:                           accessmodel.ScheduledTxStatusScheduled,
			TransactionHandlerOwner:          handlerOwner,
			TransactionHandlerTypeIdentifier: "A.1654653399040a61.MyScheduler.Handler",
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		mockState.On("Sealed").Return(mockSnapshot).Once()
		mockSnapshot.On("Head").Return(sealedHeader, nil).Once()
		// Account exists but does not have the expected contract.
		mockScriptExecutor.On("GetAccountAtBlockHeight", mocktestify.Anything, handlerOwner, sealedHeader.Height).
			Return(&flow.Account{Contracts: map[string][]byte{}}, nil).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1,
			ScheduledTransactionExpandOptions{HandlerContract: true},
			defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})
}

// TestScheduledTransactionsBackend_GetScheduledTransactions tests all code paths for the
// GetScheduledTransactions method, including pagination, filtering, and error handling.
func TestScheduledTransactionsBackend_GetScheduledTransactions(t *testing.T) {
	t.Parallel()

	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0
	defaultConfig := DefaultConfig()

	t.Run("happy path: returns transactions without next cursor when fewer than limit", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		txs := []accessmodel.ScheduledTransaction{
			{ID: 5, Status: accessmodel.ScheduledTxStatusScheduled},
			{ID: 3, Status: accessmodel.ScheduledTxStatusScheduled},
		}

		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(txs), nil).Once()

		page, err := backend.GetScheduledTransactions(
			context.Background(), 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 2)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("next cursor set when iterator yields more than limit items", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		// limit=2, provide 3 items: CollectResults collects 2, then peeks at item 3 to build cursor
		txs := []accessmodel.ScheduledTransaction{
			{ID: 5, Status: accessmodel.ScheduledTxStatusScheduled},
			{ID: 3, Status: accessmodel.ScheduledTxStatusScheduled},
			{ID: 1, Status: accessmodel.ScheduledTxStatusScheduled},
		}

		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(txs), nil).Once()

		page, err := backend.GetScheduledTransactions(
			context.Background(), 2, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 2)
		require.NotNil(t, page.NextCursor)
		assert.Equal(t, uint64(1), page.NextCursor.ID)
	})

	t.Run("default limit applied when limit is 0", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(nil), nil).Once()

		_, err := backend.GetScheduledTransactions(
			context.Background(), 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("explicit limit is respected", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(nil), nil).Once()

		_, err := backend.GetScheduledTransactions(
			context.Background(), 10, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("limit exceeding max returns InvalidArgument", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		_, err := backend.GetScheduledTransactions(
			context.Background(), 500, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("cursor is forwarded to storage", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		cursor := &accessmodel.ScheduledTransactionCursor{ID: 100}
		store.On("All", cursor).
			Return(makeScheduledTxIter(nil), nil).Once()

		_, err := backend.GetScheduledTransactions(
			context.Background(), 20, cursor,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("empty result set returns empty page", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(nil), nil).Once()

		page, err := backend.GetScheduledTransactions(
			context.Background(), 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
		assert.Empty(t, page.Transactions)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("ErrNotBootstrapped maps to FailedPrecondition", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(nil, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetScheduledTransactions(
			context.Background(), 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("unexpected storage error triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		storageErr := fmt.Errorf("unexpected disk failure")
		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(nil, storageErr).Once()

		expectedErr := fmt.Errorf("failed to get scheduled transactions: %w", storageErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr))

		_, err := backend.GetScheduledTransactions(
			signalerCtx, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.Error(t, err)
	})

	t.Run("expand error triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		txs := []accessmodel.ScheduledTransaction{
			{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted, ExecutedTransactionID: txID},
		}
		lookupErr := fmt.Errorf("lookup failed")

		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(txs), nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(flow.Identifier{}, lookupErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransactions(
			signalerCtx, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{Result: true},
			defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("tx block not yet available returns zero timestamps", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:      defaultConfig,
				collections: mockCollections,
				blocks:      mockBlocks,
			},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		createdTxID := unittest.IdentifierFixture()
		createdCollection := &flow.LightCollection{Transactions: []flow.Identifier{createdTxID}}
		txs := []accessmodel.ScheduledTransaction{
			{ID: 1, Status: accessmodel.ScheduledTxStatusScheduled, CreatedTransactionID: createdTxID},
		}

		store.On("All", (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(txs), nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", createdTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", createdTxID).Return(createdCollection, nil).Once()
		mockBlocks.On("ByCollectionID", createdCollection.ID()).Return((*flow.Block)(nil), storage.ErrNotFound).Once()

		page, err := backend.GetScheduledTransactions(
			context.Background(), 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)
		assert.Zero(t, page.Transactions[0].CreatedAt)
	})
}

// TestScheduledTransactionsBackend_GetScheduledTransactionsByAddress tests all code paths for the
// GetScheduledTransactionsByAddress method, including pagination, address scoping, and error handling.
func TestScheduledTransactionsBackend_GetScheduledTransactionsByAddress(t *testing.T) {
	t.Parallel()

	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0
	defaultConfig := DefaultConfig()

	t.Run("happy path: returns transactions for address without next cursor when fewer than limit", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		txs := []accessmodel.ScheduledTransaction{
			{ID: 7, Status: accessmodel.ScheduledTxStatusScheduled},
		}

		store.On("ByAddress", addr, (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(txs), nil).Once()

		page, err := backend.GetScheduledTransactionsByAddress(
			context.Background(), addr, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)
		assert.Equal(t, uint64(7), page.Transactions[0].ID)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("default limit applied when limit is 0", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		store.On("ByAddress", addr, (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(nil), nil).Once()

		_, err := backend.GetScheduledTransactionsByAddress(
			context.Background(), addr, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("limit exceeding max returns InvalidArgument", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()

		_, err := backend.GetScheduledTransactionsByAddress(
			context.Background(), addr, 500, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("cursor is forwarded to storage", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.ScheduledTransactionCursor{ID: 50}
		store.On("ByAddress", addr, cursor).
			Return(makeScheduledTxIter(nil), nil).Once()

		_, err := backend.GetScheduledTransactionsByAddress(
			context.Background(), addr, 15, cursor,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("empty result set returns empty page", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		store.On("ByAddress", addr, (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(nil), nil).Once()

		page, err := backend.GetScheduledTransactionsByAddress(
			context.Background(), addr, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
		assert.Empty(t, page.Transactions)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("ErrNotBootstrapped maps to FailedPrecondition", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		store.On("ByAddress", addr, (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(nil, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetScheduledTransactionsByAddress(
			context.Background(), addr, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("unexpected storage error triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		storageErr := fmt.Errorf("unexpected disk failure")
		store.On("ByAddress", addr, (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(nil, storageErr).Once()

		expectedErr := fmt.Errorf("failed to get scheduled transactions: %w", storageErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr))

		_, err := backend.GetScheduledTransactionsByAddress(
			signalerCtx, addr, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.Error(t, err)
	})

	t.Run("expand error triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		txs := []accessmodel.ScheduledTransaction{
			{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted, ExecutedTransactionID: txID},
		}
		lookupErr := fmt.Errorf("lookup failed")

		store.On("ByAddress", addr, (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(txs), nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(flow.Identifier{}, lookupErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransactionsByAddress(
			signalerCtx, addr, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{Result: true},
			defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("tx block not yet available returns zero timestamps", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:      defaultConfig,
				collections: mockCollections,
				blocks:      mockBlocks,
			},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		createdTxID := unittest.IdentifierFixture()
		createdCollection := &flow.LightCollection{Transactions: []flow.Identifier{createdTxID}}
		txs := []accessmodel.ScheduledTransaction{
			{ID: 1, Status: accessmodel.ScheduledTxStatusScheduled, CreatedTransactionID: createdTxID},
		}

		store.On("ByAddress", addr, (*accessmodel.ScheduledTransactionCursor)(nil)).
			Return(makeScheduledTxIter(txs), nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", createdTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", createdTxID).Return(createdCollection, nil).Once()
		mockBlocks.On("ByCollectionID", createdCollection.ID()).Return((*flow.Block)(nil), storage.ErrNotFound).Once()

		page, err := backend.GetScheduledTransactionsByAddress(
			context.Background(), addr, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)
		assert.Zero(t, page.Transactions[0].CreatedAt)
	})
}

// TestScheduledTransactionsBackend_PopulateBlockTimestamps verifies that populateBlockTimestamps
// correctly resolves CreatedAt and CompletedAt from the stored transaction IDs.
func TestScheduledTransactionsBackend_PopulateBlockTimestamps(t *testing.T) {
	t.Parallel()

	defaultConfig := DefaultConfig()
	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0

	// helper builds a full backend with all mocks configured
	makeBackend := func(
		store *storagemock.ScheduledTransactionsIndexReader,
		scheduledTxLookup *storagemock.ScheduledTransactionsReader,
		mockHeaders *storagemock.Headers,
		mockCollections *storagemock.CollectionsReader,
		mockBlocks *storagemock.Blocks,
	) *ScheduledTransactionsBackend {
		return NewScheduledTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:      defaultConfig,
				headers:     mockHeaders,
				blocks:      mockBlocks,
				collections: mockCollections,
			},
			flow.Mainnet, store, scheduledTxLookup, nil, nil,
		)
	}

	t.Run("created_at populated for non-placeholder tx with CreatedTransactionID", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)
		backend := makeBackend(store, scheduledTxLookup, nil, mockCollections, mockBlocks)

		createdTxID := unittest.IdentifierFixture()
		collection := &flow.LightCollection{Transactions: []flow.Identifier{createdTxID}}
		block := unittest.BlockFixture()

		storedTx := accessmodel.ScheduledTransaction{
			ID:                   1,
			Status:               accessmodel.ScheduledTxStatusScheduled,
			CreatedTransactionID: createdTxID,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		// createdTxID is a user transaction: not in scheduled tx lookup, falls back to collection
		scheduledTxLookup.On("BlockIDByTransactionID", createdTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", createdTxID).Return(collection, nil).Once()
		mockBlocks.On("ByCollectionID", collection.ID()).Return(block, nil).Once()

		result, err := backend.GetScheduledTransaction(context.Background(), 1, ScheduledTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		assert.Equal(t, block.Timestamp, result.CreatedAt)
		assert.Zero(t, result.CompletedAt)
	})

	t.Run("created_at absent for placeholder tx", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		backend := makeBackend(store, scheduledTxLookup, nil, nil, nil)

		storedTx := accessmodel.ScheduledTransaction{
			ID:            1,
			Status:        accessmodel.ScheduledTxStatusScheduled,
			IsPlaceholder: true,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		// no lookups expected: placeholder tx skips CreatedAt, Scheduled status skips CompletedAt

		result, err := backend.GetScheduledTransaction(context.Background(), 1, ScheduledTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		assert.Zero(t, result.CreatedAt)
		assert.Zero(t, result.CompletedAt)
	})

	t.Run("completed_at populated for executed tx via scheduledTxLookup", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)
		backend := makeBackend(store, scheduledTxLookup, mockHeaders, mockCollections, mockBlocks)

		createdTxID := unittest.IdentifierFixture()
		executedTxID := unittest.IdentifierFixture()
		createdCollection := &flow.LightCollection{Transactions: []flow.Identifier{createdTxID}}
		createdBlock := unittest.BlockFixture()
		executedBlockHeader := unittest.BlockHeaderFixture()
		executedBlockID := executedBlockHeader.ID()

		storedTx := accessmodel.ScheduledTransaction{
			ID:                    1,
			Status:                accessmodel.ScheduledTxStatusExecuted,
			CreatedTransactionID:  createdTxID,
			ExecutedTransactionID: executedTxID,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		// createdTxID is a user transaction: scheduledTxLookup returns ErrNotFound, falls back to collection
		scheduledTxLookup.On("BlockIDByTransactionID", createdTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", createdTxID).Return(createdCollection, nil).Once()
		mockBlocks.On("ByCollectionID", createdCollection.ID()).Return(createdBlock, nil).Once()
		// executedTxID is a system transaction: scheduledTxLookup succeeds
		scheduledTxLookup.On("BlockIDByTransactionID", executedTxID).Return(executedBlockID, nil).Once()
		mockHeaders.On("ByBlockID", executedBlockID).Return(executedBlockHeader, nil).Once()

		result, err := backend.GetScheduledTransaction(context.Background(), 1, ScheduledTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		assert.Equal(t, createdBlock.Timestamp, result.CreatedAt)
		assert.Equal(t, executedBlockHeader.Timestamp, result.CompletedAt)
	})

	t.Run("completed_at populated for cancelled tx via collection lookup", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)
		backend := makeBackend(store, scheduledTxLookup, nil, mockCollections, mockBlocks)

		createdTxID := unittest.IdentifierFixture()
		cancelledTxID := unittest.IdentifierFixture()
		createdCollection := &flow.LightCollection{Transactions: []flow.Identifier{createdTxID}}
		cancelledCollection := &flow.LightCollection{Transactions: []flow.Identifier{cancelledTxID}}
		createdBlock := unittest.BlockFixture()
		cancelledBlock := unittest.BlockFixture()

		storedTx := accessmodel.ScheduledTransaction{
			ID:                     1,
			Status:                 accessmodel.ScheduledTxStatusCancelled,
			CreatedTransactionID:   createdTxID,
			CancelledTransactionID: cancelledTxID,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		// both are user transactions: not in scheduled tx lookup, fall back to collection
		scheduledTxLookup.On("BlockIDByTransactionID", createdTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", createdTxID).Return(createdCollection, nil).Once()
		mockBlocks.On("ByCollectionID", createdCollection.ID()).Return(createdBlock, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", cancelledTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", cancelledTxID).Return(cancelledCollection, nil).Once()
		mockBlocks.On("ByCollectionID", cancelledCollection.ID()).Return(cancelledBlock, nil).Once()

		result, err := backend.GetScheduledTransaction(context.Background(), 1, ScheduledTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		assert.Equal(t, createdBlock.Timestamp, result.CreatedAt)
		assert.Equal(t, cancelledBlock.Timestamp, result.CompletedAt)
	})

	t.Run("collection lookup error for created tx triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		backend := makeBackend(store, scheduledTxLookup, nil, mockCollections, nil)

		createdTxID := unittest.IdentifierFixture()
		lookupErr := fmt.Errorf("collection lookup error")

		storedTx := accessmodel.ScheduledTransaction{
			ID:                   1,
			Status:               accessmodel.ScheduledTxStatusScheduled,
			CreatedTransactionID: createdTxID,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		// not in scheduled lookup, falls back to collection which errors
		scheduledTxLookup.On("BlockIDByTransactionID", createdTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", createdTxID).Return((*flow.LightCollection)(nil), lookupErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(signalerCtx, 1, ScheduledTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("BlockIDByTransactionID error for executed tx triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockBlocks := storagemock.NewBlocks(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := makeBackend(store, scheduledTxLookup, mockHeaders, mockCollections, mockBlocks)

		createdTxID := unittest.IdentifierFixture()
		executedTxID := unittest.IdentifierFixture()
		createdCollection := &flow.LightCollection{Transactions: []flow.Identifier{createdTxID}}
		createdBlock := unittest.BlockFixture()
		lookupErr := fmt.Errorf("block lookup error")

		storedTx := accessmodel.ScheduledTransaction{
			ID:                    1,
			Status:                accessmodel.ScheduledTxStatusExecuted,
			CreatedTransactionID:  createdTxID,
			ExecutedTransactionID: executedTxID,
		}

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		// createdTxID falls back to collection
		scheduledTxLookup.On("BlockIDByTransactionID", createdTxID).Return(flow.Identifier{}, storage.ErrNotFound).Once()
		mockCollections.On("LightByTransactionID", createdTxID).Return(createdCollection, nil).Once()
		mockBlocks.On("ByCollectionID", createdCollection.ID()).Return(createdBlock, nil).Once()
		// executedTxID lookup returns an unexpected error
		scheduledTxLookup.On("BlockIDByTransactionID", executedTxID).Return(flow.Identifier{}, lookupErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(signalerCtx, 1, ScheduledTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
		verifyThrown()
	})
}
