package blocks

import (
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// BasicProvider implements a ledger based block provider
type BasicProvider struct {
	blks *Blocks
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
	return &BasicProvider{blks: blks}, nil
}

func (p *BasicProvider) GetSnapshotAt(height uint64) (
	types.BlockSnapshot,
	error,
) {
	return p.blks, nil
}

// OnBlockReceived should be called before
// executing blocks.
func (p *BasicProvider) OnBlockReceived(blockEvent *events.BlockEventPayload) error {
	// prepare blocks
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

func (p *BasicProvider) OnBlockExecuted(blockEvent *events.BlockEventPayload) error {
	// push block hash
	// we push the block hash after execution, so the behaviour of the blockhash is
	// identical to the evm.handler.
	return p.blks.PushBlockHash(
		blockEvent.Height,
		blockEvent.Hash,
	)
}
