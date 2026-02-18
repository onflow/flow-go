package extended

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/transfers"
	"github.com/onflow/flow-go/storage"
)

const accountTransfersIndexerName = "account_transfers"

var bigZero = new(big.Int)

// AccountTransfers indexes both fungible and non-fungible token transfer events for a block.
// Both FT and NFT transfers are written atomically in the same batch, so they are always in sync.
type AccountTransfers struct {
	log       zerolog.Logger
	ftParser  *transfers.FTParser
	nftParser *transfers.NFTParser
	ftStore   storage.FungibleTokenTransfersBootstrapper
	nftStore  storage.NonFungibleTokenTransfersBootstrapper

	flowFeesAddress flow.Address
}

var _ Indexer = (*AccountTransfers)(nil)

// NewAccountTransfers creates a new [AccountTransfers] indexer.
func NewAccountTransfers(
	log zerolog.Logger,
	chainID flow.ChainID,
	ftStore storage.FungibleTokenTransfersBootstrapper,
	nftStore storage.NonFungibleTokenTransfersBootstrapper,
) *AccountTransfers {
	sc := systemcontracts.SystemContractsForChain(chainID)

	return &AccountTransfers{
		log:             log.With().Str("component", "account_transfers_indexer").Logger(),
		ftParser:        transfers.NewFTParser(chainID),
		nftParser:       transfers.NewNFTParser(chainID),
		ftStore:         ftStore,
		nftStore:        nftStore,
		flowFeesAddress: sc.FlowFees.Address,
	}
}

// Name returns the name of the indexer.
func (a *AccountTransfers) Name() string {
	return accountTransfersIndexerName
}

// NextHeight returns the next height that the indexer will index.
// Both stores are always written atomically in the same batch, so they must report the same
// next height. If they disagree, an error is returned indicating corruption.
//
// No error returns are expected during normal operation.
func (a *AccountTransfers) NextHeight() (uint64, error) {
	ftHeight, err := nextHeight(a.ftStore)
	if err != nil {
		return 0, fmt.Errorf("failed to get next height for FT store: %w", err)
	}

	nftHeight, err := nextHeight(a.nftStore)
	if err != nil {
		return 0, fmt.Errorf("failed to get next height for NFT store: %w", err)
	}

	if ftHeight != nftHeight {
		return 0, fmt.Errorf("FT and NFT stores are out of sync: FT next height %d, NFT next height %d", ftHeight, nftHeight)
	}

	return ftHeight, nil
}

// IndexBlockData indexes both FT and NFT transfer data for the given height.
// If the header in `data` does not match the expected height, an error is returned.
// Both FT and NFT stores are written in the same batch to maintain atomicity.
//
// Not safe for concurrent use.
//
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height.
//   - [ErrFutureHeight]: if the data is for a future height.
func (a *AccountTransfers) IndexBlockData(lctx lockctx.Proof, data BlockData, batch storage.ReaderBatchWriter) error {
	expectedHeight, err := a.NextHeight()
	if err != nil {
		return fmt.Errorf("failed to get next height: %w", err)
	}
	if data.Header.Height > expectedHeight {
		return ErrFutureHeight
	}
	if data.Header.Height < expectedHeight {
		return ErrAlreadyIndexed
	}

	ftEntries, err := a.buildFTTransfersFromBlockData(data)
	if err != nil {
		return fmt.Errorf("failed to build fungible token transfers from block data: %w", err)
	}
	ftEntries = a.filterFTTransfers(ftEntries)

	nftEntries, err := a.buildNFTTransfersFromBlockData(data)
	if err != nil {
		return fmt.Errorf("failed to build non-fungible token transfers from block data: %w", err)
	}

	if err := a.ftStore.Store(lctx, batch, data.Header.Height, ftEntries); err != nil {
		return fmt.Errorf("failed to store fungible token transfers: %w", err)
	}

	if err := a.nftStore.Store(lctx, batch, data.Header.Height, nftEntries); err != nil {
		return fmt.Errorf("failed to store non-fungible token transfers: %w", err)
	}

	return nil
}

// buildFTTransfersFromBlockData extracts fungible token transfers from block execution data.
//
// No error returns are expected during normal operation.
func (a *AccountTransfers) buildFTTransfersFromBlockData(data BlockData) ([]access.FungibleTokenTransfer, error) {
	allEvents := flattenEvents(data.Events)
	return a.ftParser.Parse(allEvents, data.Header.Height)
}

// buildNFTTransfersFromBlockData extracts non-fungible token transfers from block execution data.
//
// No error returns are expected during normal operation.
func (a *AccountTransfers) buildNFTTransfersFromBlockData(data BlockData) ([]access.NonFungibleTokenTransfer, error) {
	allEvents := flattenEvents(data.Events)
	return a.nftParser.Parse(allEvents, data.Header.Height)
}

// filterFTTransfers filters out transfers that do not need to be indexed.
func (a *AccountTransfers) filterFTTransfers(transfers []access.FungibleTokenTransfer) []access.FungibleTokenTransfer {
	filtered := make([]access.FungibleTokenTransfer, 0)
	for _, transfer := range transfers {
		// skip flow fee deposits since they occur in every transaction
		if transfer.RecipientAddress == a.flowFeesAddress {
			continue
		}

		// skip zero amount transfers
		if transfer.Amount.Cmp(bigZero) == 0 {
			continue
		}

		filtered = append(filtered, transfer)
	}
	return filtered
}

// flattenEvents flattens a map of events grouped by transaction index into a single slice.
func flattenEvents(eventsByTxIndex map[uint32][]flow.Event) []flow.Event {
	var all []flow.Event
	for _, events := range eventsByTxIndex {
		all = append(all, events...)
	}
	return all
}

// heightProvider is the common interface between FT and NFT bootstrapper stores needed to
// determine the next height to index.
type heightProvider interface {
	LatestIndexedHeight() (uint64, error)
	UninitializedFirstHeight() (uint64, bool)
}

// nextHeight computes the next height for a store that implements [heightProvider].
//
// No error returns are expected during normal operation.
func nextHeight(store heightProvider) (uint64, error) {
	height, err := store.LatestIndexedHeight()
	if err == nil {
		return height + 1, nil
	}

	if !errors.Is(err, storage.ErrNotBootstrapped) {
		return 0, fmt.Errorf("failed to get latest indexed height: %w", err)
	}

	firstHeight, isInitialized := store.UninitializedFirstHeight()
	if isInitialized {
		return 0, fmt.Errorf("failed to get latest indexed height, but index is initialized: %w", err)
	}

	return firstHeight, nil
}
