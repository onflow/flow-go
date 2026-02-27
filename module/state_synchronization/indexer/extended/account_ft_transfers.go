package extended

import (
	"fmt"
	"math/big"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/transfers"
	"github.com/onflow/flow-go/storage"
)

const (
	accountFTTransfersIndexerName = "account_ft_transfers"

	// omitFlowFees set whether or not to omit flow fees transfers from the index.
	omitFlowFees = true
)

var bigZero = new(big.Int)

// FungibleTokenTransfers indexes fungible token transfer events for a block.
type FungibleTokenTransfers struct {
	log      zerolog.Logger
	ftParser *transfers.FTParser
	ftStore  storage.FungibleTokenTransfersBootstrapper
}

var _ Indexer = (*FungibleTokenTransfers)(nil)

// NewFungibleTokenTransfers creates a new [FungibleTokenTransfers] indexer.
func NewFungibleTokenTransfers(
	log zerolog.Logger,
	chainID flow.ChainID,
	ftStore storage.FungibleTokenTransfersBootstrapper,
) *FungibleTokenTransfers {
	return &FungibleTokenTransfers{
		log:      log.With().Str("component", "account_ft_transfers_indexer").Logger(),
		ftParser: transfers.NewFTParser(chainID, omitFlowFees),
		ftStore:  ftStore,
	}
}

// Name returns the name of the indexer.
func (a *FungibleTokenTransfers) Name() string {
	return accountFTTransfersIndexerName
}

// NextHeight returns the next height that the indexer will index.
//
// No error returns are expected during normal operation.
func (a *FungibleTokenTransfers) NextHeight() (uint64, error) {
	return nextHeight(a.ftStore)
}

// IndexBlockData indexes FT transfer data for the given height.
// If the header in `data` does not match the expected height, an error is returned.
//
// Not safe for concurrent use.
//
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height.
//   - [ErrFutureHeight]: if the data is for a future height.
func (a *FungibleTokenTransfers) IndexBlockData(lctx lockctx.Proof, data BlockData, batch storage.ReaderBatchWriter) error {
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

	ftEntries, err := a.ftParser.Parse(data.Events, data.Header.Height)
	if err != nil {
		return fmt.Errorf("failed to parse fungible token transfers: %w", err)
	}
	ftEntries = a.filterFTTransfers(ftEntries)

	if err := a.ftStore.Store(lctx, batch, data.Header.Height, ftEntries); err != nil {
		return fmt.Errorf("failed to store fungible token transfers: %w", err)
	}

	return nil
}

// filterFTTransfers filters out transfers that do not need to be indexed.
func (a *FungibleTokenTransfers) filterFTTransfers(transfers []access.FungibleTokenTransfer) []access.FungibleTokenTransfer {
	filtered := make([]access.FungibleTokenTransfer, 0)
	for _, transfer := range transfers {
		// skip zero amount transfers
		if transfer.Amount == nil || transfer.Amount.Cmp(bigZero) == 0 {
			continue
		}
		// skip self transfers
		if transfer.SourceAddress == transfer.RecipientAddress {
			continue
		}
		filtered = append(filtered, transfer)
	}
	return filtered
}
