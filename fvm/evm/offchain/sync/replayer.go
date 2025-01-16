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
func (cr *Replayer) ReplayBlock(
	transactionEvents []events.TransactionEventPayload,
	blockEvent *events.BlockEventPayload,
) (types.ReplayResultCollector, error) {
	res, _, err := cr.ReplayBlockEvents(transactionEvents, blockEvent)
	return res, err
}

// ReplayBlockEvents replays the execution of the transactions of an EVM block
// using the provided transactionEvents and blockEvents,
// which include all the context data for re-executing the transactions, and returns
// the replay result and the result of each transaction.
// the replay result contains the register updates, and the result of each transaction
// contains the execution result of each transaction, which is useful for recontstructing
// the EVM block proposal.
// this method can be called concurrently if underlying storage
// tracer and block snapshot provider support concurrency.
//
// Warning! the list of transaction events has to be sorted based on their
// execution, sometimes the access node might return events out of order
// it needs to be sorted by txIndex and eventIndex respectively.
func (cr *Replayer) ReplayBlockEvents(
	transactionEvents []events.TransactionEventPayload,
	blockEvent *events.BlockEventPayload,
) (types.ReplayResultCollector, []*types.Result, error) {
	// prepare storage
	st, err := cr.storageProvider.GetSnapshotAt(blockEvent.Height)
	if err != nil {
		return nil, nil, err
	}

	// create storage
	state := storage.NewEphemeralStorage(storage.NewReadOnlyStorage(st))

	// get block snapshot
	bs, err := cr.blockProvider.GetSnapshotAt(blockEvent.Height)
	if err != nil {
		return nil, nil, err
	}

	// replay transactions
	results, err := ReplayBlockExecution(
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
		return nil, nil, err
	}

	return state, results, nil
}
