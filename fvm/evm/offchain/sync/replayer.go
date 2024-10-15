package sync

import (
	gethTracers "github.com/onflow/go-ethereum/eth/tracers"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
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
	tracer          *gethTracers.Tracer
	validateResults bool
}

// NewReplayer constructs a new Replayer
func NewReplayer(
	chainID flow.ChainID,
	rootAddr flow.Address,
	sp types.StorageProvider,
	logger zerolog.Logger,
	tracer *gethTracers.Tracer,
	validateResults bool,
) *Replayer {
	return &Replayer{
		chainID:         chainID,
		rootAddr:        rootAddr,
		storageProvider: sp,
		logger:          logger,
		tracer:          tracer,
		validateResults: validateResults,
	}
}

// OnBlockReceived is called when a new block is received
// (including all the related transaction executed events)
//
// currently this version of replayer requires
// sequential calls to the OnBlockReceived
// in the future if move the blocks logic to outside
// we can pass blockSnapshotProvider and have the ability
// to call OnBlockReceived concurrently.
func (cr *Replayer) OnBlockReceived(
	transactionEvents []events.TransactionEventPayload,
	blockEvent *events.BlockEventPayload,
) (types.ReplayResults, error) {
	// prepare storage
	st, err := cr.storageProvider.GetSnapshotAt(blockEvent.Height)
	if err != nil {
		return nil, err
	}

	// create storage
	state := storage.NewEphemeralStorage(storage.NewReadOnlyStorage(st))

	// prepare blocks
	blks, err := blocks.NewBlocks(cr.chainID, cr.rootAddr, state)
	if err != nil {
		return nil, err
	}

	// push the new block meta
	// it should be done before execution so block context creation
	// can be done properly
	err = blks.PushBlockMeta(
		blocks.NewMeta(
			blockEvent.Height,
			blockEvent.Timestamp,
			blockEvent.PrevRandao,
		),
	)
	if err != nil {
		return nil, err
	}

	// replay transactions
	err = ReplayBlockExecution(
		cr.chainID,
		cr.rootAddr,
		state,
		blks,
		cr.tracer,
		transactionEvents,
		blockEvent,
		cr.validateResults,
	)
	if err != nil {
		return nil, err
	}

	// push block hash
	// we push the block hash after execution, so the behaviour of the blockhash is
	// identical to the evm.handler.
	err = blks.PushBlockHash(
		blockEvent.Height,
		blockEvent.Hash,
	)
	if err != nil {
		return nil, err
	}

	return state, nil
}
