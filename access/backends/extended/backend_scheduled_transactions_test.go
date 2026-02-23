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
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	executionmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

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

	t.Run("empty filter matches all", func(t *testing.T) {
		filter := ScheduledTransactionFilter{}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("status filter matches", func(t *testing.T) {
		filter := ScheduledTransactionFilter{
			Statuses: []accessmodel.ScheduledTxStatus{accessmodel.ScheduledTxStatusScheduled},
		}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("status filter rejects mismatch", func(t *testing.T) {
		filter := ScheduledTransactionFilter{
			Statuses: []accessmodel.ScheduledTxStatus{accessmodel.ScheduledTxStatusExecuted},
		}
		assert.False(t, filter.Filter()(tx))
	})

	t.Run("status filter matches when one of multiple statuses matches", func(t *testing.T) {
		filter := ScheduledTransactionFilter{
			Statuses: []accessmodel.ScheduledTxStatus{
				accessmodel.ScheduledTxStatusExecuted,
				accessmodel.ScheduledTxStatusScheduled,
			},
		}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("priority filter matches", func(t *testing.T) {
		p := uint8(5)
		filter := ScheduledTransactionFilter{Priority: &p}
		assert.True(t, filter.Filter()(tx))
	})

	t.Run("priority filter rejects mismatch", func(t *testing.T) {
		p := uint8(10)
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
		p := uint8(5)
		start := uint64(1000)
		end := uint64(1000)
		uuid := uint64(99)
		filter := ScheduledTransactionFilter{
			Statuses:                 []accessmodel.ScheduledTxStatus{accessmodel.ScheduledTxStatusScheduled},
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
		p := uint8(5)
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
			store, nil, nil, nil,
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
			store, nil, nil, nil,
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
			store, nil, nil, nil,
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
			store, nil, nil, nil,
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

	t.Run("expand is no-op for scheduled status", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			store, nil, nil, nil,
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

	t.Run("expand is no-op for cancelled status", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			store, nil, nil, nil,
		)

		tx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusCancelled}
		store.On("ByID", uint64(1)).Return(tx, nil).Once()

		result, err := backend.GetScheduledTransaction(
			context.Background(), 1,
			ScheduledTransactionExpandOptions{Result: true, Transaction: true},
			defaultEncoding,
		)
		require.NoError(t, err)
		assert.Nil(t, result.Transaction)
		assert.Nil(t, result.Result)
	})

	// expand result works for executed and failed transactions
	for _, status := range []accessmodel.ScheduledTxStatus{accessmodel.ScheduledTxStatusExecuted, accessmodel.ScheduledTxStatusFailed} {
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
				store, scheduledTxLookup, nil, nil,
			)

			txID := unittest.IdentifierFixture()
			blockHeader := unittest.BlockHeaderFixture()
			blockID := blockHeader.ID()

			storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: status}
			expectedResult := &accessmodel.TransactionResult{
				TransactionID: txID,
				BlockID:       blockID,
				Status:        flow.TransactionStatusSealed,
			}

			store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
			scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(txID, nil).Once()
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
	for _, status := range []accessmodel.ScheduledTxStatus{accessmodel.ScheduledTxStatusExecuted, accessmodel.ScheduledTxStatusFailed} {
		t.Run(fmt.Sprintf("expand transaction body on %s transaction", status), func(t *testing.T) {
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
				store, scheduledTxLookup, nil, nil,
			)

			txBody := unittest.TransactionBodyFixture()
			txID := txBody.ID()
			blockHeader := unittest.BlockHeaderFixture()
			blockID := blockHeader.ID()

			storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: status}

			store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
			scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(txID, nil).Once()
			scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(blockID, nil).Once()
			mockHeaders.On("ByBlockID", blockID).Return(blockHeader, nil).Once()
			mockProvider.On("ScheduledTransactionsByBlockID", mocktestify.Anything, blockHeader).
				Return([]*flow.TransactionBody{&txBody}, nil).Once()

			result, err := backend.GetScheduledTransaction(
				context.Background(), 1,
				ScheduledTransactionExpandOptions{Transaction: true},
				defaultEncoding,
			)
			require.NoError(t, err)
			require.NotNil(t, result.Transaction)
			assert.Equal(t, &txBody, result.Transaction)
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
			store, nil, mockState, mockScriptExecutor,
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

	t.Run("TransactionIDByID error during expand triggers irrecoverable", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		scheduledTxLookup := storagemock.NewScheduledTransactionsReader(t)

		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			store, scheduledTxLookup, nil, nil,
		)

		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted}
		lookupErr := fmt.Errorf("lookup error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(flow.Identifier{}, lookupErr).Once()

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
			store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted}
		blockLookupErr := fmt.Errorf("block lookup error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(txID, nil).Once()
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
			store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		blockID := unittest.IdentifierFixture()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted}
		headerErr := fmt.Errorf("header lookup error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(txID, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(blockID, nil).Once()
		mockHeaders.On("ByBlockID", blockID).Return((*flow.Header)(nil), headerErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1, ScheduledTransactionExpandOptions{Result: true}, defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("ScheduledTransactionsByBlockID error during expand triggers irrecoverable", func(t *testing.T) {
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
			store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()
		blockID := blockHeader.ID()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted}
		providerErr := fmt.Errorf("provider error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(txID, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(blockID, nil).Once()
		mockHeaders.On("ByBlockID", blockID).Return(blockHeader, nil).Once()
		mockProvider.On("ScheduledTransactionsByBlockID", mocktestify.Anything, blockHeader).
			Return(nil, providerErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1, ScheduledTransactionExpandOptions{Transaction: true}, defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("transaction not found in block during expand triggers irrecoverable", func(t *testing.T) {
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
			store, scheduledTxLookup, nil, nil,
		)

		// txID that does NOT match the tx body returned by the provider.
		txID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()
		blockID := blockHeader.ID()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted}
		// otherTxBody.ID() != txID
		otherTxBody := unittest.TransactionBodyFixture()

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(txID, nil).Once()
		scheduledTxLookup.On("BlockIDByTransactionID", txID).Return(blockID, nil).Once()
		mockHeaders.On("ByBlockID", blockID).Return(blockHeader, nil).Once()
		mockProvider.On("ScheduledTransactionsByBlockID", mocktestify.Anything, blockHeader).
			Return([]*flow.TransactionBody{&otherTxBody}, nil).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransaction(
			signalerCtx, 1, ScheduledTransactionExpandOptions{Transaction: true}, defaultEncoding,
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
			store, scheduledTxLookup, nil, nil,
		)

		txID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()
		blockID := blockHeader.ID()
		storedTx := accessmodel.ScheduledTransaction{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted}
		resultErr := fmt.Errorf("result lookup error")

		store.On("ByID", uint64(1)).Return(storedTx, nil).Once()
		scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(txID, nil).Once()
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
			store, nil, mockState, nil,
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
			store, nil, mockState, mockScriptExecutor,
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
			store, nil, mockState, mockScriptExecutor,
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

	t.Run("happy path: returns page from storage", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			store, nil, nil, nil,
		)

		expectedPage := accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{
				{ID: 5, Status: accessmodel.ScheduledTxStatusScheduled},
				{ID: 3, Status: accessmodel.ScheduledTxStatusExecuted},
			},
			NextCursor: &accessmodel.ScheduledTransactionCursor{ID: 3},
		}

		store.On("All", uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(expectedPage, nil).Once()

		page, err := backend.GetScheduledTransactions(
			context.Background(), 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 2)
		assert.Equal(t, expectedPage.NextCursor, page.NextCursor)
	})

	t.Run("default limit applied when limit is 0", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			store, nil, nil, nil,
		)

		store.On("All", defaultConfig.DefaultPageSize, (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, nil).Once()

		_, err := backend.GetScheduledTransactions(
			context.Background(), 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{},
			defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("explicit limit is forwarded to storage", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			store, nil, nil, nil,
		)

		store.On("All", uint32(10), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, nil).Once()

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
			store, nil, nil, nil,
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
			store, nil, nil, nil,
		)

		cursor := &accessmodel.ScheduledTransactionCursor{ID: 100}
		store.On("All", uint32(20), cursor, mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, nil).Once()

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
			store, nil, nil, nil,
		)

		store.On("All", uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, nil).Once()

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
			store, nil, nil, nil,
		)

		store.On("All", uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, storage.ErrNotBootstrapped).Once()

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
			store, nil, nil, nil,
		)

		storageErr := fmt.Errorf("unexpected disk failure")
		store.On("All", uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, storageErr).Once()

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
			store, scheduledTxLookup, nil, nil,
		)

		txPage := accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{
				{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted},
			},
		}
		lookupErr := fmt.Errorf("lookup failed")

		store.On("All", uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(txPage, nil).Once()
		scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(flow.Identifier{}, lookupErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransactions(
			signalerCtx, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{Result: true},
			defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})
}

// TestScheduledTransactionsBackend_GetScheduledTransactionsByAddress tests all code paths for the
// GetScheduledTransactionsByAddress method, including pagination, address scoping, and error handling.
func TestScheduledTransactionsBackend_GetScheduledTransactionsByAddress(t *testing.T) {
	t.Parallel()

	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0
	defaultConfig := DefaultConfig()

	t.Run("happy path: returns page for address", func(t *testing.T) {
		store := storagemock.NewScheduledTransactionsIndexReader(t)
		backend := NewScheduledTransactionsBackend(
			unittest.Logger(), &backendBase{config: defaultConfig},
			store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		expectedPage := accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{
				{ID: 7, Status: accessmodel.ScheduledTxStatusScheduled},
			},
			NextCursor: nil,
		}

		store.On("ByAddress", addr, uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(expectedPage, nil).Once()

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
			store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		store.On("ByAddress", addr, defaultConfig.DefaultPageSize, (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, nil).Once()

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
			store, nil, nil, nil,
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
			store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.ScheduledTransactionCursor{ID: 50}
		store.On("ByAddress", addr, uint32(15), cursor, mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, nil).Once()

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
			store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		store.On("ByAddress", addr, uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, nil).Once()

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
			store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		store.On("ByAddress", addr, uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, storage.ErrNotBootstrapped).Once()

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
			store, nil, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		storageErr := fmt.Errorf("unexpected disk failure")
		store.On("ByAddress", addr, uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ScheduledTransactionsPage{}, storageErr).Once()

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
			store, scheduledTxLookup, nil, nil,
		)

		addr := unittest.RandomAddressFixture()
		txPage := accessmodel.ScheduledTransactionsPage{
			Transactions: []accessmodel.ScheduledTransaction{
				{ID: 1, Status: accessmodel.ScheduledTxStatusExecuted},
			},
		}
		lookupErr := fmt.Errorf("lookup failed")

		store.On("ByAddress", addr, uint32(50), (*accessmodel.ScheduledTransactionCursor)(nil), mocktestify.Anything).
			Return(txPage, nil).Once()
		scheduledTxLookup.On("TransactionIDByID", uint64(1)).Return(flow.Identifier{}, lookupErr).Once()

		signalerCtx, verifyThrown := signalerCtxExpectingThrow(t)

		_, err := backend.GetScheduledTransactionsByAddress(
			signalerCtx, addr, 0, nil,
			ScheduledTransactionFilter{}, ScheduledTransactionExpandOptions{Result: true},
			defaultEncoding,
		)
		require.Error(t, err)
		verifyThrown()
	})
}
