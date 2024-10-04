package sync

import (
	gethTracers "github.com/onflow/go-ethereum/eth/tracers"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/model/flow"
)

// ChainReplayer consumes EVM transaction and block
// events, re-execute EVM transaction and follows EVM chain.
// this allows using different tracers and storage solutions.
type ChainReplayer struct {
	chainID         flow.ChainID
	logger          zerolog.Logger
	storageProvider storage.StorageProvider
	tracer          *gethTracers.Tracer
	validateResults bool
}

// NewChainReplayer constructs a new ChainReplayer
func NewChainReplayer(
	chainID flow.ChainID,
	sp storage.StorageProvider,
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
	blockEvent *events.BlockEventPayload,
) (ReplayResults, error) {
	// prepare storage
	st, err := cr.storageProvider.GetSnapshotAt(blockEvent.Height)
	if err != nil {
		return nil, err
	}

	// replay transactions
	return ReplayBlockExecution(
		cr.chainID,
		st,
		cr.tracer,
		transactionEvents,
		blockEvent,
		cr.validateResults,
	)
}
