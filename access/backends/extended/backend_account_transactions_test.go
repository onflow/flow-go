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

func TestBackend_GetAccountTransactions(t *testing.T) {
	t.Parallel()

	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0

	t.Run("happy path returns page from storage", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, mockHeaders, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()

		expectedPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{
					Address:          addr,
					BlockHeight:      blockHeader.Height,
					TransactionID:    txID,
					TransactionIndex: 0,
					Roles:            []accessmodel.TransactionRole{accessmodel.TransactionRoleAuthorizer},
				},
			},
			NextCursor: nil,
		}

		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(expectedPage, nil)
		mockHeaders.On("BlockIDByHeight", blockHeader.Height).Return(blockHeader.ID(), nil)
		mockHeaders.On("ByBlockID", blockHeader.ID()).Return(blockHeader, nil)

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
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, mockHeaders, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		blockHeader := unittest.BlockHeaderFixture()

		nonEmptyPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: blockHeader.Height, TransactionID: unittest.IdentifierFixture()},
			},
		}

		// Expect the default page size (50)
		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(nonEmptyPage, nil)
		mockHeaders.On("BlockIDByHeight", blockHeader.Height).Return(blockHeader.ID(), nil)
		mockHeaders.On("ByBlockID", blockHeader.ID()).Return(blockHeader, nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("max limit cap applied", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, mockHeaders, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		blockHeader := unittest.BlockHeaderFixture()

		nonEmptyPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: blockHeader.Height, TransactionID: unittest.IdentifierFixture()},
			},
		}

		// Request 500, expect capped to 200
		mockStore.On("TransactionsByAddress",
			addr, uint32(200), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(nonEmptyPage, nil)
		mockHeaders.On("BlockIDByHeight", blockHeader.Height).Return(blockHeader.ID(), nil)
		mockHeaders.On("ByBlockID", blockHeader.ID()).Return(blockHeader, nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 500, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("cursor is forwarded to storage", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, mockHeaders, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.AccountTransactionCursor{BlockHeight: 50, TransactionIndex: 3}
		blockHeader := unittest.BlockHeaderFixture(func(h *flow.Header) { h.Height = 50 })

		nonEmptyPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: 50, TransactionID: unittest.IdentifierFixture()},
			},
		}

		mockStore.On("TransactionsByAddress",
			addr, uint32(10), cursor, mocktestify.Anything,
		).Return(nonEmptyPage, nil)
		mockHeaders.On("BlockIDByHeight", uint64(50)).Return(blockHeader.ID(), nil)
		mockHeaders.On("ByBlockID", blockHeader.ID()).Return(blockHeader, nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 10, cursor, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("non-empty filter forwards non-nil filter function to storage", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, mockHeaders, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		blockHeader := unittest.BlockHeaderFixture()
		filter := AccountTransactionFilter{Roles: []accessmodel.TransactionRole{accessmodel.TransactionRoleAuthorizer}}

		page := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: blockHeader.Height, TransactionID: unittest.IdentifierFixture()},
			},
		}

		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil),
			mocktestify.MatchedBy(func(f storage.IndexFilter[*accessmodel.AccountTransaction]) bool {
				return f != nil
			}),
		).Return(page, nil)
		mockHeaders.On("BlockIDByHeight", blockHeader.Height).Return(blockHeader.ID(), nil)
		mockHeaders.On("ByBlockID", blockHeader.ID()).Return(blockHeader, nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, filter, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("next cursor from storage page is returned to caller", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, mockHeaders, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		blockHeader := unittest.BlockHeaderFixture()
		nextCursor := &accessmodel.AccountTransactionCursor{BlockHeight: 99, TransactionIndex: 2}

		expectedPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: blockHeader.Height, TransactionID: unittest.IdentifierFixture()},
			},
			NextCursor: nextCursor,
		}

		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(expectedPage, nil)
		mockHeaders.On("BlockIDByHeight", blockHeader.Height).Return(blockHeader.ID(), nil)
		mockHeaders.On("ByBlockID", blockHeader.ID()).Return(blockHeader, nil)

		page, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.NoError(t, err)
		require.NotNil(t, page.NextCursor)
		assert.Equal(t, nextCursor, page.NextCursor)
	})

	t.Run("ErrNotBootstrapped maps to FailedPrecondition", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()

		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(accessmodel.AccountTransactionsPage{}, storage.ErrNotBootstrapped)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("ErrHeightNotIndexed maps to OutOfRange", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.AccountTransactionCursor{BlockHeight: 999, TransactionIndex: 0}

		mockStore.On("TransactionsByAddress",
			addr, uint32(10), cursor, mocktestify.Anything,
		).Return(accessmodel.AccountTransactionsPage{}, fmt.Errorf("wrapped: %w", storage.ErrHeightNotIndexed))

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
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, mockHeaders, mockCollections, nil, nil, nil, mockProvider)

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()
		blockHeader := unittest.BlockHeaderFixture()
		expectedResult := &accessmodel.TransactionResult{}

		storedPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: blockHeader.Height, TransactionID: txID},
			},
		}

		mockStore.On("TransactionsByAddress", addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything).Return(storedPage, nil)
		mockHeaders.On("BlockIDByHeight", blockHeader.Height).Return(blockHeader.ID(), nil)
		mockHeaders.On("ByBlockID", blockHeader.ID()).Return(blockHeader, nil)
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
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		storageErr := fmt.Errorf("unexpected storage failure")

		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(accessmodel.AccountTransactionsPage{}, storageErr)

		expectedErr := fmt.Errorf("failed to get account transactions: %w", storageErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr))

		_, err := backend.GetAccountTransactions(signalerCtx, addr, 0, nil, AccountTransactionFilter{}, AccountTransactionExpandOptions{}, defaultEncoding)
		require.Error(t, err)
	})
}
