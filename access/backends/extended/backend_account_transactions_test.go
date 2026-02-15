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

	accessmodel "github.com/onflow/flow-go/model/access"
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
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), "", mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()

		expectedPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{
					Address:          addr,
					BlockHeight:      100,
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

		page, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, false, defaultEncoding)
		require.NoError(t, err)
		require.Len(t, page.Transactions, 1)
		assert.Equal(t, txID, page.Transactions[0].TransactionID)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("default limit applied when limit is 0", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), "", mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()

		nonEmptyPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: 1, TransactionID: unittest.IdentifierFixture()},
			},
		}

		// Expect the default page size (50)
		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(nonEmptyPage, nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, false, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("max limit cap applied", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), "", mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()

		nonEmptyPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: 1, TransactionID: unittest.IdentifierFixture()},
			},
		}

		// Request 500, expect capped to 200
		mockStore.On("TransactionsByAddress",
			addr, uint32(200), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(nonEmptyPage, nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 500, nil, AccountTransactionFilter{}, false, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("cursor is forwarded to storage", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), "", mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.AccountTransactionCursor{BlockHeight: 50, TransactionIndex: 3}

		nonEmptyPage := accessmodel.AccountTransactionsPage{
			Transactions: []accessmodel.AccountTransaction{
				{Address: addr, BlockHeight: 50, TransactionID: unittest.IdentifierFixture()},
			},
		}

		mockStore.On("TransactionsByAddress",
			addr, uint32(10), cursor, mocktestify.Anything,
		).Return(nonEmptyPage, nil)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 10, cursor, AccountTransactionFilter{}, false, defaultEncoding)
		require.NoError(t, err)
	})

	t.Run("ErrNotBootstrapped maps to FailedPrecondition", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), "", mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()

		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(accessmodel.AccountTransactionsPage{}, storage.ErrNotBootstrapped)

		_, err := backend.GetAccountTransactions(context.Background(), addr, 0, nil, AccountTransactionFilter{}, false, defaultEncoding)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("ErrHeightNotIndexed maps to OutOfRange", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), "", mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.AccountTransactionCursor{BlockHeight: 999, TransactionIndex: 0}

		mockStore.On("TransactionsByAddress",
			addr, uint32(10), cursor, mocktestify.Anything,
		).Return(accessmodel.AccountTransactionsPage{}, fmt.Errorf("wrapped: %w", storage.ErrHeightNotIndexed))

		_, err := backend.GetAccountTransactions(context.Background(), addr, 10, cursor, AccountTransactionFilter{}, false, defaultEncoding)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.OutOfRange, st.Code())
	})

	t.Run("unexpected error triggers irrecoverable", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsReader(t)
		backend := NewAccountTransactionsBackend(unittest.Logger(), DefaultConfig(), "", mockStore, nil, nil, nil, nil, nil, nil)

		addr := unittest.RandomAddressFixture()
		storageErr := fmt.Errorf("unexpected storage failure")

		mockStore.On("TransactionsByAddress",
			addr, uint32(50), (*accessmodel.AccountTransactionCursor)(nil), mocktestify.Anything,
		).Return(accessmodel.AccountTransactionsPage{}, storageErr)

		expectedErr := fmt.Errorf("failed to get account transactions: %w", storageErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr))

		_, err := backend.GetAccountTransactions(signalerCtx, addr, 0, nil, AccountTransactionFilter{}, false, defaultEncoding)
		require.Error(t, err)
	})
}
