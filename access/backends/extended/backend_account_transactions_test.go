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
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// txEntry is a test implementation of IteratorEntry for AccountTransaction.
type txEntry struct {
	tx accessmodel.AccountTransaction
}

func (e txEntry) Cursor() (accessmodel.AccountTransactionCursor, error) {
	return accessmodel.AccountTransactionCursor{
		Address:          e.tx.Address,
		BlockHeight:      e.tx.BlockHeight,
		TransactionIndex: e.tx.TransactionIndex,
	}, nil
}

func (e txEntry) Value() (accessmodel.AccountTransaction, error) {
	return e.tx, nil
}

func newSliceIter(txs []accessmodel.AccountTransaction) storage.AccountTransactionIterator {
	return func(yield func(storage.IteratorEntry[accessmodel.AccountTransaction, accessmodel.AccountTransactionCursor]) bool) {
		for _, tx := range txs {
			if !yield(txEntry{tx: tx}) {
				return
			}
		}
	}
}

func TestBackend_GetAccountTransactions(t *testing.T) {
	t.Parallel()

	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0

	t.Run("happy path returns page from storage", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig(), headers: mockHeaders}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()

		txs := []accessmodel.AccountTransaction{
			{
				Address:          addr,
				BlockHeight:      blockHeader.Height,
				TransactionID:    txID,
				TransactionIndex: 0,
				Roles:            []accessmodel.TransactionRole{accessmodel.TransactionRoleAuthorizer},
			},
		}

		mockStore.On("ByAddress", addr, (*accessmodel.AccountTransactionCursor)(nil)).Return(newSliceIter(txs), nil)
		mockHeaders.On("ByHeight", blockHeader.Height).Return(blockHeader, nil)

		page, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)
		assert.Equal(t, txID, page.Transactions[0].TransactionID)
		assert.Equal(t, blockHeader.Timestamp, page.Transactions[0].BlockTimestamp)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("default limit applied when limit is 0", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig(), headers: mockHeaders}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		blockHeader := unittest.BlockHeaderFixture()

		txs := []accessmodel.AccountTransaction{
			{Address: addr, BlockHeight: blockHeader.Height, TransactionID: unittest.IdentifierFixture()},
		}

		mockStore.On("ByAddress", addr, (*accessmodel.AccountTransactionCursor)(nil)).Return(newSliceIter(txs), nil)
		mockHeaders.On("ByHeight", blockHeader.Height).Return(unittest.BlockHeaderFixture(), nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("limit exceeding max returns InvalidArgument", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		_, err := backend.GetAccountTransactions(context.Background(), addr, 500, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("cursor is forwarded to storage", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig(), headers: mockHeaders}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.AccountTransactionCursor{BlockHeight: 50, TransactionIndex: 3}
		blockHeader := unittest.BlockHeaderFixture(func(h *flow.Header) { h.Height = 50 })

		txs := []accessmodel.AccountTransaction{
			{Address: addr, BlockHeight: 50, TransactionID: unittest.IdentifierFixture()},
		}

		mockStore.On("ByAddress", addr, cursor).Return(newSliceIter(txs), nil)
		mockHeaders.On("ByHeight", uint64(50)).Return(blockHeader, nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 10, cursor, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("filter applied by backend: only matching transactions returned", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig(), headers: mockHeaders}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		blockHeader := unittest.BlockHeaderFixture()
		filter := AccountTransactionFilter{Roles: []accessmodel.TransactionRole{accessmodel.TransactionRoleAuthorizer}}

		txID := unittest.IdentifierFixture()
		// Iterator yields one authorizer and one payer tx; only the authorizer should be returned.
		txs := []accessmodel.AccountTransaction{
			{Address: addr, BlockHeight: blockHeader.Height, TransactionID: txID, Roles: []accessmodel.TransactionRole{accessmodel.TransactionRoleAuthorizer}},
			{Address: addr, BlockHeight: blockHeader.Height, TransactionID: unittest.IdentifierFixture(), Roles: []accessmodel.TransactionRole{accessmodel.TransactionRolePayer}},
		}

		mockStore.On("ByAddress", addr, (*accessmodel.AccountTransactionCursor)(nil)).Return(newSliceIter(txs), nil)
		mockHeaders.On("ByHeight", blockHeader.Height).Return(blockHeader, nil)

		page, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, filter, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)
		assert.Equal(t, txID, page.Transactions[0].TransactionID)
	})

	t.Run("next cursor set when iterator has more results than limit", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig(), headers: mockHeaders}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		blockHeader := unittest.BlockHeaderFixture()

		// Iterator yields limit+1 transactions; the extra one becomes the next cursor.
		txs := []accessmodel.AccountTransaction{
			{Address: addr, BlockHeight: blockHeader.Height, TransactionID: unittest.IdentifierFixture(), TransactionIndex: 0},
			{Address: addr, BlockHeight: 99, TransactionID: unittest.IdentifierFixture(), TransactionIndex: 2},
		}

		mockStore.On("ByAddress", addr, (*accessmodel.AccountTransactionCursor)(nil)).Return(newSliceIter(txs), nil)
		mockHeaders.On("ByHeight", blockHeader.Height).Return(blockHeader, nil)

		page, err := backend.GetAccountTransactions(context.Background(), addr, 1, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)
		require.NotNil(t, page.NextCursor)
		assert.Equal(t, uint64(99), page.NextCursor.BlockHeight)
		assert.Equal(t, uint32(2), page.NextCursor.TransactionIndex)
	})

	t.Run("ErrNotBootstrapped maps to FailedPrecondition", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore, flow.Testnet.Chain())

		addr := unittest.InvalidAddressFixture()

		_, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("empty results with valid address returns empty page", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		mockStore.On("ByAddress", addr, (*accessmodel.AccountTransactionCursor)(nil)).Return(newSliceIter(nil), nil)

		page, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		assert.Empty(t, page.Transactions)
	})

	t.Run("ErrNotBootstrapped maps to FailedPrecondition", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		mockStore.On("ByAddress", addr, (*accessmodel.AccountTransactionCursor)(nil)).Return(nil, storage.ErrNotBootstrapped)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("ErrHeightNotIndexed maps to OutOfRange", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.AccountTransactionCursor{BlockHeight: 999, TransactionIndex: 0}

		mockStore.On("ByAddress", addr, cursor).Return(nil, fmt.Errorf("wrapped: %w", storage.ErrHeightNotIndexed))

		_, err := backend.GetAccountTransactions(context.Background(), addr, 10, cursor, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.OutOfRange, st.Code())
	})

	t.Run("expand result without expand transaction succeeds when collection not indexed", func(t *testing.T) {
		// Regression test: when expandOptions.Result=true and expandOptions.Transaction=false,
		// getTransactionResult is called with isSystemChunkTx=false (since getTransactionBody is
		// skipped). If LightByTransactionID returns ErrNotFound (e.g. asynchronous index not yet
		// available), the old code called collection.ID() on a nil pointer, causing a panic.
		// The fixed code skips the assignment and uses a zero collectionID instead.
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		mockCollections := storagemock.NewCollectionsReader(t)
		mockProvider := providermock.NewTransactionProvider(t)
		backend := NewAccountTransactionsBackend(
			unittest.Logger(),
			&backendBase{
				config:               DefaultConfig(),
				headers:              mockHeaders,
				collections:          mockCollections,
				transactionsProvider: mockProvider,
			},
			mockStore,
			flow.Testnet.Chain(),
		)

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()
		expectedResult := &accessmodel.TransactionResult{}

		storedPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: blockHeader.Height, TransactionID: txID},
			},
		}

		mockStore.On("ByAddress", addr, (*accessmodel.AccountTransactionCursor)(nil)).Return(newSliceIter(storedPage.Transactions), nil)
		mockHeaders.On("ByHeight", blockHeader.Height).Return(blockHeader, nil)
		// Collection is not yet indexed; LightByTransactionID returns ErrNotFound.
		mockCollections.On("LightByTransactionID", txID).Return((*flow.LightCollection)(nil), storage.ErrNotFound).Once()
		// Expects zero collectionID since the collection lookup returned ErrNotFound.
		mockProvider.On("TransactionResult", mocktestify.Anything, blockHeader, txID, flow.ZeroID, defaultEncoding).Return(expectedResult, nil).Once()

		expandOptions := AccountTransactionExpandOptions{Result: true, Transaction: false}
		page, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, expandOptions, defaultEncoding)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)
		assert.Equal(t, expectedResult, page.Transactions[0].Result)
	})

	t.Run("unexpected error triggers irrecoverable", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		storageErr := fmt.Errorf("unexpected storage failure")

		mockStore.On("ByAddress", addr, (*accessmodel.AccountTransactionCursor)(nil)).Return(nil, storageErr)

		expectedErr := fmt.Errorf("failed to get account transactions: %w", storageErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr))

		_, err := backend.GetAccountTransactions(signalerCtx, addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
	})
}
