package blocks

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// BasicProvider implements a ledger-backed basic block snapshot provider
// it assumes sequential progress on blocks and expects a
// a OnBlockReceived call before block execution and
// a follow up OnBlockExecuted call after block execution.
type BasicProvider struct {
	chainID            flow.ChainID
	blks               *Blocks
	rootAddr           flow.Address
	storage            types.BackendStorage
	latestBlockPayload *events.BlockEventPayload
}

var _ types.BlockSnapshotProvider = (*BasicProvider)(nil)

func NewBasicProvider(
	chainID flow.ChainID,
	storage types.BackendStorage,
	rootAddr flow.Address,
) (*BasicProvider, error) {
	blks, err := NewBlocks(chainID, rootAddr, storage)
	if err != nil {
		return nil, err
	}
	return &BasicProvider{
		chainID:  chainID,
		blks:     blks,
		rootAddr: rootAddr,
		storage:  storage,
	}, nil
}

// GetSnapshotAt returns a block snapshot at the given height
// Snapshot at a height is not available until `OnBlockReceived` is called for that height.
func (p *BasicProvider) GetSnapshotAt(height uint64) (
	types.BlockSnapshot,
	error,
) {
	if p.latestBlockPayload.Height != height {
		return nil, fmt.Errorf("active block height doesn't match expected: %d, got: %d", p.latestBlockPayload.Height, height)
	}
	return p.blks, nil
}

// OnBlockReceived should be called before executing blocks.
func (p *BasicProvider) OnBlockReceived(blockEvent *events.BlockEventPayload) error {
	p.latestBlockPayload = blockEvent
	// push the new block meta
	// it should be done before execution so block context creation
	// can be done properly
	return p.blks.PushBlockMeta(
		NewMeta(
			blockEvent.Height,
			blockEvent.Timestamp,
			blockEvent.PrevRandao,
		),
	)
}

// OnBlockExecuted should be called after executing blocks.
func (p *BasicProvider) OnBlockExecuted(
	height uint64,
	resCol types.ReplayResultCollector,
	blockProposal *types.BlockProposal,
) error {
	// we push the block hash after execution, so the behaviour of the blockhash is
	// identical to the evm.handler.
	if p.latestBlockPayload.Height != height {
		return fmt.Errorf("active block height doesn't match expected: %d, got: %d", p.latestBlockPayload.Height, height)
	}

	blockBytes, err := blockProposal.Block.ToBytes()
	if err != nil {
		return types.NewFatalError(err)
	}

	// do the same as handler.CommitBlockProposal
	err = p.storage.SetValue(
		p.rootAddr[:],
		[]byte(handler.BlockStoreLatestBlockKey),
		blockBytes,
	)
	if err != nil {
		return err
	}

	blockProposalBytes, err := blockProposal.ToBytes()
	if err != nil {
		return types.NewFatalError(err)
	}

	// update block proposal
	err = p.storage.SetValue(
		p.rootAddr[:],
		[]byte(handler.BlockStoreLatestBlockProposalKey),
		blockProposalBytes,
	)
	if err != nil {
		return err
	}

	storedHash := p.latestBlockPayload.Hash
	hash, ok := UseBlockHashCorrection(p.chainID, height, height)
	if ok {
		storedHash = hash
	}

	// update block hash list
	return p.blks.PushBlockHash(
		p.latestBlockPayload.Height,
		storedHash,
	)
}
