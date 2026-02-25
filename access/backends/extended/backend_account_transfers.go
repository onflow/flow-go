package extended

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type AccountTransferExpandOptions struct {
	Transaction bool
	Result      bool
}

func (o *AccountTransferExpandOptions) HasExpand() bool {
	return o.Transaction || o.Result
}

type AccountFTTransferFilter struct {
	TokenType        string
	SourceAddress    flow.Address
	RecipientAddress flow.Address
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
		return true
	}
}

type AccountNFTTransferFilter struct {
	TokenType        string
	SourceAddress    flow.Address
	RecipientAddress flow.Address
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
		return true
	}
}

// AccountTransfersBackend implements the extended API for querying account token transfers.
type AccountTransfersBackend struct {
	*backendBase

	log      zerolog.Logger
	ftStore  storage.FungibleTokenTransfersBootstrapper
	nftStore storage.NonFungibleTokenTransfersBootstrapper

	chain flow.Chain
}

// NewAccountTransfersBackend creates a new AccountTransfersBackend instance.
func NewAccountTransfersBackend(
	log zerolog.Logger,
	base *backendBase,
	ftStore storage.FungibleTokenTransfersBootstrapper,
	nftStore storage.NonFungibleTokenTransfersBootstrapper,
	chain flow.Chain,
) *AccountTransfersBackend {
	return &AccountTransfersBackend{
		backendBase: base,
		log:         log,
		ftStore:     ftStore,
		nftStore:    nftStore,
		chain:       chain,
	}
}

// GetAccountFungibleTokenTransfers returns a paginated list of fungible token transfers for the
// given account address. Results are ordered descending by block height (newest first).
//
// If the account has no transfers, the response will include an empty array and no error.
//
// Expected error returns during normal operations:
//   - [codes.NotFound] if the account is not found
//   - [codes.FailedPrecondition] if the fungible token transfer index has not been initialized
//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
//   - [codes.InvalidArgument] if the query parameters are invalid
func (b *AccountTransfersBackend) GetAccountFungibleTokenTransfers(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.TransferCursor,
	filter AccountFTTransferFilter,
	expandOptions AccountTransferExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.FungibleTokenTransfersPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	if !b.chain.IsValid(address) {
		return nil, status.Errorf(codes.NotFound, "account %s is not valid on chain %s", address, b.chain.ChainID())
	}

	page, err := b.ftStore.ByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "fungible token transfers", err)
	}

	// storage will return an empty page and no error if the account has no transfers indexed.
	// TODO: check if account exists for the chain
	if len(page.Transfers) == 0 {
		return &page, nil
	}

	for i := range page.Transfers {
		t := &page.Transfers[i]

		header, txBody, result, err := b.expand(ctx, t.BlockHeight, t.TransactionID, expandOptions, encodingVersion)
		if err != nil {
			err = fmt.Errorf("failed to populate details for transfer transaction %s: %w", t.TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}

		// only the expended options will be populated
		t.BlockTimestamp = header.Timestamp
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
//   - [codes.NotFound] if the account is not found
//   - [codes.FailedPrecondition] if the non-fungible token transfer index has not been initialized
//   - [codes.OutOfRange] if the cursor references a height outside the indexed range
//   - [codes.InvalidArgument] if the query parameters are invalid
func (b *AccountTransfersBackend) GetAccountNonFungibleTokenTransfers(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.TransferCursor,
	filter AccountNFTTransferFilter,
	expandOptions AccountTransferExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.NonFungibleTokenTransfersPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	if !b.chain.IsValid(address) {
		return nil, status.Errorf(codes.NotFound, "account %s is not valid on chain %s", address, b.chain.ChainID())
	}

	page, err := b.nftStore.ByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "non-fungible token transfers", err)
	}

	// storage will return an empty page and no error if the account has no transfers indexed.
	// TODO: check if account exists for the chain
	if len(page.Transfers) == 0 {
		return &page, nil
	}

	for i := range page.Transfers {
		t := &page.Transfers[i]

		header, txBody, result, err := b.expand(ctx, t.BlockHeight, t.TransactionID, expandOptions, encodingVersion)
		if err != nil {
			err = fmt.Errorf("failed to populate details for transfer transaction %s: %w", t.TransactionID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}

		// only the expended options will be populated
		t.BlockTimestamp = header.Timestamp
		t.Transaction = txBody
		t.Result = result
	}

	return &page, nil
}

// expand adds additional details to the transaction.
//
// Since the extended indexer only indexes sealed data, all transaction and result data should exist
// in storage for the given height.
//
// No error returns are expected during normal operation.
func (b *AccountTransfersBackend) expand(
	ctx context.Context,
	blockHeight uint64,
	transactionID flow.Identifier,
	expandOptions AccountTransferExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*flow.Header, *flow.TransactionBody, *accessmodel.TransactionResult, error) {
	blockID, err := b.headers.BlockIDByHeight(blockHeight)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not retrieve block ID: %w", err)
	}

	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not retrieve block header: %w", err)
	}

	// only add the transaction body and result if requested
	if !expandOptions.HasExpand() {
		return header, nil, nil, nil
	}

	var txBody *flow.TransactionBody
	var isSystemChunkTx bool
	if expandOptions.Transaction {
		txBody, isSystemChunkTx, err = b.getTransactionBody(ctx, header, transactionID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not retrieve transaction body: %w", err)
		}
	}

	var result *accessmodel.TransactionResult
	if expandOptions.Result {
		result, err = b.getTransactionResult(ctx, transactionID, header, isSystemChunkTx, expandOptions.Transaction, encodingVersion)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not retrieve transaction result: %w", err)
		}
	}

	return header, txBody, result, nil
}
