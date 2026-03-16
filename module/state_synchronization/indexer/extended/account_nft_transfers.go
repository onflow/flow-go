package extended

import (
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/transfers"
	"github.com/onflow/flow-go/storage"
)

const accountNFTTransfersIndexerName = "account_nft_transfers"

// NonFungibleTokenTransfers indexes non-fungible token transfer events for a block.
type NonFungibleTokenTransfers struct {
	log       zerolog.Logger
	nftParser *transfers.NFTParser
	nftStore  storage.NonFungibleTokenTransfersBootstrapper
	metrics   module.ExtendedIndexingMetrics
}

type NonFungibleTokenTransfersMetadata struct{}

var _ Indexer = (*NonFungibleTokenTransfers)(nil)
var _ IndexProcessor[access.NonFungibleTokenTransfer, NonFungibleTokenTransfersMetadata] = (*NonFungibleTokenTransfers)(nil)

// NewNonFungibleTokenTransfers creates a new [NonFungibleTokenTransfers] indexer.
func NewNonFungibleTokenTransfers(
	log zerolog.Logger,
	chainID flow.ChainID,
	nftStore storage.NonFungibleTokenTransfersBootstrapper,
	metrics module.ExtendedIndexingMetrics,
) *NonFungibleTokenTransfers {
	return &NonFungibleTokenTransfers{
		log:       log.With().Str("component", "account_nft_transfers_indexer").Logger(),
		nftParser: transfers.NewNFTParser(chainID),
		nftStore:  nftStore,
		metrics:   metrics,
	}
}

// Name returns the name of the indexer.
func (a *NonFungibleTokenTransfers) Name() string {
	return accountNFTTransfersIndexerName
}

// NextHeight returns the next height that the indexer will index.
//
// No error returns are expected during normal operation.
func (a *NonFungibleTokenTransfers) NextHeight() (uint64, error) {
	return nextHeight(a.nftStore)
}

// IndexBlockData indexes NFT transfer data for the given height.
// If the header in `data` does not match the expected height, an error is returned.
//
// The caller must hold the [storage.LockIndexNonFungibleTokenTransfers] lock until the batch is committed.
//
// CAUTION: Not safe for concurrent use.
//
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height.
//   - [ErrFutureHeight]: if the data is for a future height.
func (a *NonFungibleTokenTransfers) IndexBlockData(lctx lockctx.Proof, data BlockData, batch storage.ReaderBatchWriter) error {
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

	nftEntries, _, err := a.ProcessBlockData(data)
	if err != nil {
		return err
	}

	if err := a.nftStore.Store(lctx, batch, data.Header.Height, nftEntries); err != nil {
		return fmt.Errorf("failed to store non-fungible token transfers: %w", err)
	}

	a.metrics.NFTTransferIndexed(len(nftEntries))

	return nil
}

// ProcessBlockData processes the block data and returns the indexed non-fungible token transfer entries.
//
// No error returns are expected during normal operation.
func (a *NonFungibleTokenTransfers) ProcessBlockData(data BlockData) ([]access.NonFungibleTokenTransfer, NonFungibleTokenTransfersMetadata, error) {
	nftEntries, err := a.nftParser.Parse(data.Events, data.Header.Height)
	if err != nil {
		return nil, NonFungibleTokenTransfersMetadata{}, fmt.Errorf("failed to parse non-fungible token transfers: %w", err)
	}
	return nftEntries, NonFungibleTokenTransfersMetadata{}, nil
}
