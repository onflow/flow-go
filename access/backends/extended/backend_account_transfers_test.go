package extended

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBackend_GetAccountFungibleTokenTransfers(t *testing.T) {
	t.Parallel()

	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0
	defaultConfig := DefaultConfig()

	t.Run("happy path returns page from storage", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: DefaultConfig(), headers: mockHeaders}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()

		expectedPage := accessmodel.FungibleTokenTransfersPage{
			Transfers: []accessmodel.FungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      100,
					TransactionIndex: 0,
					TokenType:        "A.1654653399040a61.FlowToken",
					Amount:           big.NewInt(1000),
					SourceAddress:    addr,
					RecipientAddress: unittest.RandomAddressFixture(),
				},
			},
			NextCursor: nil,
		}

		blockID := unittest.IdentifierFixture()
		ftStore.On("ByAddress", addr, uint32(50), (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(expectedPage, nil)
		mockHeaders.On("BlockIDByHeight", uint64(100)).Return(blockID, nil)
		mockHeaders.On("ByBlockID", blockID).Return(unittest.BlockHeaderFixture(), nil)

		page, err := backend.GetAccountFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
		require.Len(t, page.Transfers, 1)
		assert.Equal(t, txID, page.Transfers[0].TransactionID)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("default limit applied when limit is 0", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig, headers: mockHeaders}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		nonEmptyPage := accessmodel.FungibleTokenTransfersPage{
			Transfers: []accessmodel.FungibleTokenTransfer{
				{BlockHeight: 1, TransactionID: unittest.IdentifierFixture()},
			},
		}

		blockID := unittest.IdentifierFixture()
		// Expect the default page size (50)
		ftStore.On("ByAddress", addr, defaultConfig.DefaultPageSize, (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(nonEmptyPage, nil)
		mockHeaders.On("BlockIDByHeight", uint64(1)).Return(blockID, nil)
		mockHeaders.On("ByBlockID", blockID).Return(unittest.BlockHeaderFixture(), nil)

		_, err := backend.GetAccountFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("limit exceeding max returns InvalidArgument", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		_, err := backend.GetAccountFungibleTokenTransfers(
			context.Background(), addr, 500, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("cursor is forwarded to storage", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig, headers: mockHeaders}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.TransferCursor{BlockHeight: 50, TransactionIndex: 3, EventIndex: 1}

		nonEmptyPage := accessmodel.FungibleTokenTransfersPage{
			Transfers: []accessmodel.FungibleTokenTransfer{
				{BlockHeight: 50, TransactionID: unittest.IdentifierFixture()},
			},
		}

		blockID := unittest.IdentifierFixture()
		ftStore.On("ByAddress", addr, uint32(10), cursor, mocktestify.Anything).
			Return(nonEmptyPage, nil)
		mockHeaders.On("BlockIDByHeight", uint64(50)).Return(blockID, nil)
		mockHeaders.On("ByBlockID", blockID).Return(unittest.BlockHeaderFixture(), nil)

		_, err := backend.GetAccountFungibleTokenTransfers(
			context.Background(), addr, 10, cursor, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("invalid address returns NotFound", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.InvalidAddressFixture()

		_, err := backend.GetAccountFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("empty results with valid address returns empty page", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		ftStore.On("ByAddress", addr, uint32(50), (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(accessmodel.FungibleTokenTransfersPage{}, nil)

		page, err := backend.GetAccountFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
		assert.Empty(t, page.Transfers)
	})

	t.Run("ErrNotBootstrapped maps to FailedPrecondition", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		ftStore.On("ByAddress", addr, uint32(50), (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(accessmodel.FungibleTokenTransfersPage{}, storage.ErrNotBootstrapped)

		_, err := backend.GetAccountFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("ErrHeightNotIndexed maps to OutOfRange", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.TransferCursor{BlockHeight: 999, TransactionIndex: 0, EventIndex: 0}

		ftStore.On("ByAddress", addr, uint32(10), cursor, mocktestify.Anything).
			Return(accessmodel.FungibleTokenTransfersPage{}, fmt.Errorf("wrapped: %w", storage.ErrHeightNotIndexed))

		_, err := backend.GetAccountFungibleTokenTransfers(
			context.Background(), addr, 10, cursor, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.OutOfRange, st.Code())
	})

	t.Run("unexpected error triggers irrecoverable", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		storageErr := fmt.Errorf("unexpected storage failure")

		ftStore.On("ByAddress", addr, uint32(50), (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(accessmodel.FungibleTokenTransfersPage{}, storageErr)

		expectedErr := fmt.Errorf("failed to get fungible token transfers: %w", storageErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr))

		_, err := backend.GetAccountFungibleTokenTransfers(
			signalerCtx, addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
	})
}

func TestBackend_GetAccountNonFungibleTokenTransfers(t *testing.T) {
	t.Parallel()

	defaultEncoding := entities.EventEncodingVersion_JSON_CDC_V0
	defaultConfig := DefaultConfig()

	t.Run("happy path returns page from storage", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig, headers: mockHeaders}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		txID := unittest.IdentifierFixture()

		expectedPage := accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{
				{
					TransactionID:    txID,
					BlockHeight:      100,
					TransactionIndex: 0,
					TokenType:        "A.1654653399040a61.MyNFT",
					ID:               42,
					SourceAddress:    addr,
					RecipientAddress: unittest.RandomAddressFixture(),
				},
			},
			NextCursor: nil,
		}

		blockID := unittest.IdentifierFixture()
		nftStore.On("ByAddress", addr, uint32(50), (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(expectedPage, nil)
		mockHeaders.On("BlockIDByHeight", uint64(100)).Return(blockID, nil)
		mockHeaders.On("ByBlockID", blockID).Return(unittest.BlockHeaderFixture(), nil)

		page, err := backend.GetAccountNonFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
		require.Len(t, page.Transfers, 1)
		assert.Equal(t, txID, page.Transfers[0].TransactionID)
		assert.Nil(t, page.NextCursor)
	})

	t.Run("default limit applied when limit is 0", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig, headers: mockHeaders}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		nonEmptyPage := accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{
				{BlockHeight: 1, TransactionID: unittest.IdentifierFixture()},
			},
		}

		blockID := unittest.IdentifierFixture()
		nftStore.On("ByAddress", addr, defaultConfig.DefaultPageSize, (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(nonEmptyPage, nil)
		mockHeaders.On("BlockIDByHeight", uint64(1)).Return(blockID, nil)
		mockHeaders.On("ByBlockID", blockID).Return(unittest.BlockHeaderFixture(), nil)

		_, err := backend.GetAccountNonFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("limit exceeding max returns InvalidArgument", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		_, err := backend.GetAccountNonFungibleTokenTransfers(
			context.Background(), addr, 500, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("cursor is forwarded to storage", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		mockHeaders := storagemock.NewHeaders(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig, headers: mockHeaders}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.TransferCursor{BlockHeight: 50, TransactionIndex: 3, EventIndex: 1}

		nonEmptyPage := accessmodel.NonFungibleTokenTransfersPage{
			Transfers: []accessmodel.NonFungibleTokenTransfer{
				{BlockHeight: 50, TransactionID: unittest.IdentifierFixture()},
			},
		}

		blockID := unittest.IdentifierFixture()
		nftStore.On("ByAddress", addr, uint32(10), cursor, mocktestify.Anything).
			Return(nonEmptyPage, nil)
		mockHeaders.On("BlockIDByHeight", uint64(50)).Return(blockID, nil)
		mockHeaders.On("ByBlockID", blockID).Return(unittest.BlockHeaderFixture(), nil)

		_, err := backend.GetAccountNonFungibleTokenTransfers(
			context.Background(), addr, 10, cursor, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
	})

	t.Run("invalid address returns NotFound", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.InvalidAddressFixture()

		_, err := backend.GetAccountNonFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("empty results with valid address returns empty page", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		nftStore.On("ByAddress", addr, uint32(50), (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(accessmodel.NonFungibleTokenTransfersPage{}, nil)

		page, err := backend.GetAccountNonFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.NoError(t, err)
		assert.Empty(t, page.Transfers)
	})

	t.Run("ErrNotBootstrapped maps to FailedPrecondition", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()

		nftStore.On("ByAddress", addr, uint32(50), (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(accessmodel.NonFungibleTokenTransfersPage{}, storage.ErrNotBootstrapped)

		_, err := backend.GetAccountNonFungibleTokenTransfers(
			context.Background(), addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("ErrHeightNotIndexed maps to OutOfRange", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		cursor := &accessmodel.TransferCursor{BlockHeight: 999, TransactionIndex: 0, EventIndex: 0}

		nftStore.On("ByAddress", addr, uint32(10), cursor, mocktestify.Anything).
			Return(accessmodel.NonFungibleTokenTransfersPage{}, fmt.Errorf("wrapped: %w", storage.ErrHeightNotIndexed))

		_, err := backend.GetAccountNonFungibleTokenTransfers(
			context.Background(), addr, 10, cursor, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.OutOfRange, st.Code())
	})

	t.Run("unexpected error triggers irrecoverable", func(t *testing.T) {
		ftStore := storagemock.NewFungibleTokenTransfersBootstrapper(t)
		nftStore := storagemock.NewNonFungibleTokenTransfersBootstrapper(t)
		backend := NewAccountTransfersBackend(unittest.Logger(), &backendBase{config: defaultConfig}, ftStore, nftStore, flow.Testnet.Chain())

		addr := unittest.RandomAddressFixture()
		storageErr := fmt.Errorf("unexpected storage failure")

		nftStore.On("ByAddress", addr, uint32(50), (*accessmodel.TransferCursor)(nil), mocktestify.Anything).
			Return(accessmodel.NonFungibleTokenTransfersPage{}, storageErr)

		expectedErr := fmt.Errorf("failed to get non-fungible token transfers: %w", storageErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
			irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr))

		_, err := backend.GetAccountNonFungibleTokenTransfers(
			signalerCtx, addr, 0, nil, AccountTransferFilter{}, AccountTransferExpandOptions{}, defaultEncoding,
		)
		require.Error(t, err)
	})
}

func TestAccountFTTransferFilter(t *testing.T) {
	t.Parallel()

	senderAddr := unittest.RandomAddressFixture()
	recipientAddr := unittest.RandomAddressFixture()
	otherAddr := unittest.RandomAddressFixture()

	transfer := &accessmodel.FungibleTokenTransfer{
		TokenType:        "A.1654653399040a61.FlowToken",
		SourceAddress:    senderAddr,
		RecipientAddress: recipientAddr,
	}

	t.Run("empty filter matches all", func(t *testing.T) {
		filter := AccountTransferFilter{}
		assert.True(t, filter.FTFilter()(transfer))
	})

	t.Run("token type filter matches", func(t *testing.T) {
		filter := AccountTransferFilter{TokenType: "A.1654653399040a61.FlowToken"}
		assert.True(t, filter.FTFilter()(transfer))
	})

	t.Run("token type filter rejects mismatch", func(t *testing.T) {
		filter := AccountTransferFilter{TokenType: "A.0xOther.USDC"}
		assert.False(t, filter.FTFilter()(transfer))
	})

	t.Run("source address filter matches", func(t *testing.T) {
		filter := AccountTransferFilter{SourceAddress: senderAddr}
		assert.True(t, filter.FTFilter()(transfer))
	})

	t.Run("source address filter rejects mismatch", func(t *testing.T) {
		filter := AccountTransferFilter{SourceAddress: otherAddr}
		assert.False(t, filter.FTFilter()(transfer))
	})

	t.Run("recipient address filter matches", func(t *testing.T) {
		filter := AccountTransferFilter{RecipientAddress: recipientAddr}
		assert.True(t, filter.FTFilter()(transfer))
	})

	t.Run("recipient address filter rejects mismatch", func(t *testing.T) {
		filter := AccountTransferFilter{RecipientAddress: otherAddr}
		assert.False(t, filter.FTFilter()(transfer))
	})

	t.Run("sender role matches when account is source", func(t *testing.T) {
		filter := AccountTransferFilter{
			SourceAddress: senderAddr,
		}
		assert.True(t, filter.FTFilter()(transfer))
	})

	t.Run("sender role rejects when account is not source", func(t *testing.T) {
		filter := AccountTransferFilter{
			SourceAddress: recipientAddr,
		}
		assert.False(t, filter.FTFilter()(transfer))
	})

	t.Run("recipient role matches when account is recipient", func(t *testing.T) {
		filter := AccountTransferFilter{
			RecipientAddress: recipientAddr,
		}
		assert.True(t, filter.FTFilter()(transfer))
	})

	t.Run("recipient role rejects when account is not recipient", func(t *testing.T) {
		filter := AccountTransferFilter{
			RecipientAddress: senderAddr,
		}
		assert.False(t, filter.FTFilter()(transfer))
	})

	t.Run("combined filters all match", func(t *testing.T) {
		filter := AccountTransferFilter{
			TokenType:        "A.1654653399040a61.FlowToken",
			SourceAddress:    senderAddr,
			RecipientAddress: recipientAddr,
		}
		assert.True(t, filter.FTFilter()(transfer))
	})

	t.Run("combined filters reject on first mismatch", func(t *testing.T) {
		filter := AccountTransferFilter{
			TokenType:     "A.0xOther.USDC", // mismatch
			SourceAddress: senderAddr,       // match
		}
		assert.False(t, filter.FTFilter()(transfer))
	})

	t.Run("empty address fields are ignored", func(t *testing.T) {
		filter := AccountTransferFilter{
			SourceAddress:    flow.EmptyAddress,
			RecipientAddress: flow.EmptyAddress,
		}
		assert.True(t, filter.FTFilter()(transfer))
	})
}

func TestAccountTransferFilter(t *testing.T) {
	t.Parallel()

	senderAddr := unittest.RandomAddressFixture()
	recipientAddr := unittest.RandomAddressFixture()
	otherAddr := unittest.RandomAddressFixture()

	transfer := &accessmodel.NonFungibleTokenTransfer{
		TokenType:        "A.1654653399040a61.MyNFT",
		SourceAddress:    senderAddr,
		RecipientAddress: recipientAddr,
	}

	t.Run("empty filter matches all", func(t *testing.T) {
		filter := AccountTransferFilter{}
		assert.True(t, filter.NFTFilter()(transfer))
	})

	t.Run("token type filter matches", func(t *testing.T) {
		filter := AccountTransferFilter{TokenType: "A.1654653399040a61.MyNFT"}
		assert.True(t, filter.NFTFilter()(transfer))
	})

	t.Run("token type filter rejects mismatch", func(t *testing.T) {
		filter := AccountTransferFilter{TokenType: "A.0xOther.OtherNFT"}
		assert.False(t, filter.NFTFilter()(transfer))
	})

	t.Run("source address filter matches", func(t *testing.T) {
		filter := AccountTransferFilter{SourceAddress: senderAddr}
		assert.True(t, filter.NFTFilter()(transfer))
	})

	t.Run("source address filter rejects mismatch", func(t *testing.T) {
		filter := AccountTransferFilter{SourceAddress: otherAddr}
		assert.False(t, filter.NFTFilter()(transfer))
	})

	t.Run("recipient address filter matches", func(t *testing.T) {
		filter := AccountTransferFilter{RecipientAddress: recipientAddr}
		assert.True(t, filter.NFTFilter()(transfer))
	})

	t.Run("recipient address filter rejects mismatch", func(t *testing.T) {
		filter := AccountTransferFilter{RecipientAddress: otherAddr}
		assert.False(t, filter.NFTFilter()(transfer))
	})

	t.Run("sender role matches when account is source", func(t *testing.T) {
		filter := AccountTransferFilter{
			SourceAddress: senderAddr,
		}
		assert.True(t, filter.NFTFilter()(transfer))
	})

	t.Run("sender role rejects when account is not source", func(t *testing.T) {
		filter := AccountTransferFilter{
			SourceAddress: recipientAddr,
		}
		assert.False(t, filter.NFTFilter()(transfer))
	})

	t.Run("recipient role matches when account is recipient", func(t *testing.T) {
		filter := AccountTransferFilter{
			RecipientAddress: recipientAddr,
		}
		assert.True(t, filter.NFTFilter()(transfer))
	})

	t.Run("recipient role rejects when account is not recipient", func(t *testing.T) {
		filter := AccountTransferFilter{
			RecipientAddress: senderAddr,
		}
		assert.False(t, filter.NFTFilter()(transfer))
	})

	t.Run("combined filters all match", func(t *testing.T) {
		filter := AccountTransferFilter{
			TokenType:        "A.1654653399040a61.MyNFT",
			SourceAddress:    senderAddr,
			RecipientAddress: recipientAddr,
		}
		assert.True(t, filter.NFTFilter()(transfer))
	})

	t.Run("combined filters reject on first mismatch", func(t *testing.T) {
		filter := AccountTransferFilter{
			TokenType:     "A.0xOther.OtherNFT", // mismatch
			SourceAddress: senderAddr,           // match
		}
		assert.False(t, filter.NFTFilter()(transfer))
	})

	t.Run("empty address fields are ignored", func(t *testing.T) {
		filter := AccountTransferFilter{
			SourceAddress:    flow.EmptyAddress,
			RecipientAddress: flow.EmptyAddress,
		}
		assert.True(t, filter.NFTFilter()(transfer))
	})
}
