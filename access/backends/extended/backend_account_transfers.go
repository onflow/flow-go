package extended

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type AccountFTTransferFilter struct {
	AccountAddress   flow.Address
	TokenType        string
	SourceAddress    flow.Address
	RecipientAddress flow.Address
	TransferRole     accessmodel.TransferRole
}

func (f *AccountFTTransferFilter) Filter() storage.IndexFilter[*accessmodel.FungibleTokenTransfer] {
	return func(transfer *accessmodel.FungibleTokenTransfer) bool {
		if f.TokenType != "" && transfer.TokenType != f.TokenType {
			return false
		}
		if f.SourceAddress != flow.EmptyAddress && transfer.SourceAddress != f.SourceAddress {
			return false
		}
		if f.RecipientAddress != flow.EmptyAddress && transfer.RecipientAddress != f.RecipientAddress {
			return false
		}
		if f.TransferRole == accessmodel.TransferRoleSender && f.AccountAddress != transfer.SourceAddress {
			return false
		}
		if f.TransferRole == accessmodel.TransferRoleRecipient && f.AccountAddress != transfer.RecipientAddress {
			return false
		}
		return true
	}
}

type AccountNFTTransferFilter struct {
	AccountAddress   flow.Address
	TokenType        string
	SourceAddress    flow.Address
	RecipientAddress flow.Address
	TransferRole     accessmodel.TransferRole
}

func (f *AccountNFTTransferFilter) Filter() storage.IndexFilter[*accessmodel.NonFungibleTokenTransfer] {
	return func(transfer *accessmodel.NonFungibleTokenTransfer) bool {
		if f.TokenType != "" && transfer.TokenType != f.TokenType {
			return false
		}
		if f.SourceAddress != flow.EmptyAddress && transfer.SourceAddress != f.SourceAddress {
			return false
		}
		if f.RecipientAddress != flow.EmptyAddress && transfer.RecipientAddress != f.RecipientAddress {
			return false
		}
		if f.TransferRole == accessmodel.TransferRoleSender && f.AccountAddress != transfer.SourceAddress {
			return false
		}
		if f.TransferRole == accessmodel.TransferRoleRecipient && f.AccountAddress != transfer.RecipientAddress {
			return false
		}
		return true
	}
}

// AccountTransfersBackend implements the extended API for querying account token transfers.
type AccountTransfersBackend struct {
	*backendBase

	log      zerolog.Logger
	ftStore  storage.FungibleTokenTransfersBootstrapper
	nftStore storage.NonFungibleTokenTransfersBootstrapper
}

// NewAccountTransfersBackend creates a new AccountTransfersBackend instance.
func NewAccountTransfersBackend(
	log zerolog.Logger,
	base *backendBase,
	ftStore storage.FungibleTokenTransfersBootstrapper,
	nftStore storage.NonFungibleTokenTransfersBootstrapper,
) *AccountTransfersBackend {
	return &AccountTransfersBackend{
		backendBase: base,
		log:         log,
		ftStore:     ftStore,
		nftStore:    nftStore,
	}
}

// GetAccountFungibleTokenTransfers returns a paginated list of fungible token transfers for the
// given account address. Results are ordered descending by block height (newest first).
//
// If the account has no transfers, the response will include an empty array and no error.
//
// Expected error returns during normal operations:
//   - [codes.NotFound] if the account is found but has no transfers
//   - [codes.FailedPrecondition] if the fungible token transfer index has not been initialized
//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
//   - [codes.InvalidArgument] if the query parameters are invalid
func (b *AccountTransfersBackend) GetAccountFungibleTokenTransfers(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.TransferCursor,
	filter AccountFTTransferFilter,
	expandResults bool,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.FungibleTokenTransfersPage, error) {
	limit = b.normalizeLimit(limit)

	page, err := b.ftStore.ByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "fungible token transfers", err)
	}

	// storage will return an empty page and no error if the account has no transfers indexed.
	if len(page.Transfers) == 0 && cursor != nil {
		return nil, status.Errorf(codes.NotFound, "no fungible token transfers found for account %s", address)
	}

	for i := range page.Transfers {
		t := &page.Transfers[i]

		header, err := b.headers.ByHeight(t.BlockHeight)
		if err != nil {
			err = fmt.Errorf("failed to retrieve block header for transfer transaction %s: %w", t.TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
		t.BlockTimestamp = header.Timestamp

		if !expandResults {
			continue
		}

		txBody, result, err := b.lookupTransactionDetails(ctx, t.TransactionID, header, encodingVersion)
		if err != nil {
			err = fmt.Errorf("failed to populate details for transfer transaction %s: %w", t.TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
		t.Transaction = txBody
		t.Result = result
	}

	return &page, nil
}

// GetAccountNonFungibleTokenTransfers returns a paginated list of non-fungible token transfers for
// the given account address. Results are ordered descending by block height (newest first).
//
// If the account has no transfers, the response will include an empty array and no error.
//
// Expected error returns during normal operations:
//   - [codes.NotFound] if the account is found but has no transfers
//   - [codes.FailedPrecondition] if the non-fungible token transfer index has not been initialized
//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
//   - [codes.InvalidArgument] if the query parameters are invalid
func (b *AccountTransfersBackend) GetAccountNonFungibleTokenTransfers(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.TransferCursor,
	filter AccountNFTTransferFilter,
	expandResults bool,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.NonFungibleTokenTransfersPage, error) {
	limit = b.normalizeLimit(limit)

	page, err := b.nftStore.ByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "non-fungible token transfers", err)
	}

	// storage will return an empty page and no error if the account has no transfers indexed.
	if len(page.Transfers) == 0 && cursor != nil {
		return nil, status.Errorf(codes.NotFound, "no non-fungible token transfers found for account %s", address)
	}

	for i := range page.Transfers {
		t := &page.Transfers[i]

		header, err := b.headers.ByHeight(t.BlockHeight)
		if err != nil {
			err = fmt.Errorf("failed to retrieve block header for transfer transaction %s: %w", t.TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
		t.BlockTimestamp = header.Timestamp

		if !expandResults {
			continue
		}

		txBody, result, err := b.lookupTransactionDetails(ctx, t.TransactionID, header, encodingVersion)
		if err != nil {
			err = fmt.Errorf("failed to populate details for transfer transaction %s: %w", t.TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
		t.Transaction = txBody
		t.Result = result
	}

	return &page, nil
}
