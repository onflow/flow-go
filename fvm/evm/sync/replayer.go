package sync

import (
	gethTracers "github.com/onflow/go-ethereum/eth/tracers"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/model/flow"
)

// ChainReplayer consumes EVM transaction and block
// events, re-execute EVM transaction and follows EVM chain.
// this allows using different tracers and storage solutions.
type ChainReplayer struct {
	chainID         flow.ChainID
	logger          zerolog.Logger
	storageProvider StorageProvider
	tracer          *gethTracers.Tracer
	validateResults bool
}

// NewChainReplayer constructs a new ChainReplayer
func NewChainReplayer(
	chainID flow.ChainID,
	sp StorageProvider,
	logger zerolog.Logger,
	tracer *gethTracers.Tracer,
	validateResults bool,
) *ChainReplayer {
	return &ChainReplayer{
		chainID:         chainID,
		storageProvider: sp,
		logger:          logger,
		tracer:          tracer,
	}
}

// OnBlockReceived is called when a new block is received
// (including all the related transaction executed events)
func (cr *ChainReplayer) OnBlockReceived(
	transactionEvents []events.TransactionEventPayload,
	blockEvent events.BlockEventPayload,
	onTransactionReplayed OnTransactionReplayed,
) error {
	// prepare storage
	st, err := cr.storageProvider.GetStorageByHeight(blockEvent.Height)
	if err != nil {
		return err
	}
	storage := NewEphemeralStorage(st)

	// create blocks
	blocks, err := NewBlocks(cr.chainID, storage)
	if err != nil {
		return err
	}

	// push the new block
	err = blocks.PushBlock(
		blockEvent.Height,
		blockEvent.Timestamp,
		blockEvent.PrevRandao,
		blockEvent.Hash,
	)
	if err != nil {
		return err
	}

	// replay transactions
	err = ReplayBlockExecution(
		cr.chainID,
		storage,
		blocks,
		cr.tracer,
		transactionEvents,
		blockEvent,
		cr.validateResults,
		onTransactionReplayed,
	)
	if err != nil {
		return err
	}
	// if everything successful commit changes
	return storage.Commit()
}
