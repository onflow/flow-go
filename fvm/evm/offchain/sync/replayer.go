package sync

import (
	gethTracers "github.com/onflow/go-ethereum/eth/tracers"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// Replayer consumes EVM transaction and block
// events, re-execute EVM transaction and follows EVM chain.
// this allows using different tracers and storage solutions.
type Replayer struct {
	chainID         flow.ChainID
	rootAddr        flow.Address
	logger          zerolog.Logger
	storageProvider types.StorageProvider
	blockProvider   types.BlockSnapshotProvider
	tracer          *gethTracers.Tracer
	validateResults bool
}

// NewReplayer constructs a new Replayer
func NewReplayer(
	chainID flow.ChainID,
	rootAddr flow.Address,
	sp types.StorageProvider,
	bp types.BlockSnapshotProvider,
	logger zerolog.Logger,
	tracer *gethTracers.Tracer,
	validateResults bool,
) *Replayer {
	return &Replayer{
		chainID:         chainID,
		rootAddr:        rootAddr,
		storageProvider: sp,
		blockProvider:   bp,
		logger:          logger,
		tracer:          tracer,
		validateResults: validateResults,
	}
}

// ReplayBlock replays the execution of the transactions of an EVM block
// using the provided transactionEvents and blockEvents,
// which include all the context data for re-executing the transactions, and returns the replay result.
// this method can be called concurrently if underlying storage
// tracer and block snapshot provider support concurrency.
func (cr *Replayer) ReplayBlock(
	transactionEvents []events.TransactionEventPayload,
	blockEvent *events.BlockEventPayload,
) (types.ReplayResultCollector, error) {
	// prepare storage
	st, err := cr.storageProvider.GetSnapshotAt(blockEvent.Height)
	if err != nil {
		return nil, err
	}

	// create storage
	state := storage.NewEphemeralStorage(storage.NewReadOnlyStorage(st))

	// get block snapshot
	bs, err := cr.blockProvider.GetSnapshotAt(blockEvent.Height)
	if err != nil {
		return nil, err
	}

	// replay transactions
	err = ReplayBlockExecution(
		cr.chainID,
		cr.rootAddr,
		state,
		bs,
		cr.tracer,
		transactionEvents,
		blockEvent,
		cr.validateResults,
	)
	if err != nil {
		return nil, err
	}

	return state, nil
}
