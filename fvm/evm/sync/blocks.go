package sync

import (
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	gethCommon "github.com/onflow/go-ethereum/common"
)

// blocks wraps a blockHash list to provide a subset
// of block store functionality that is needed for replaying
// transactions
type blocks struct {
	bhl *handler.BlockHashList
}

// newBlocks constructs a new blocks type
func newBlocks(
	chainID flow.ChainID,
	storage types.BackendStorage,
) (*blocks, error) {
	bhl, err := handler.NewBlockHashList(
		storage,
		evm.StorageAccountAddress(chainID),
		handler.BlockHashListCapacity,
	)
	if err != nil {
		return nil, err
	}
	// if empty insert genesis block hash
	if bhl.IsEmpty() {
		err = bhl.Push(0, types.GenesisBlockHash(chainID))
		if err != nil {
			return nil, err
		}
	}
	return &blocks{bhl}, nil
}

// LastAddedHeight returns the last added height
func (b *blocks) LastAddedHeight() uint64 {
	// this assumes we always push a genesis
	return b.bhl.MaxAvailableHeight()
}

// PushBlockHash pushes a block hash to the blocks
func (b *blocks) PushBlockHash(h gethCommon.Hash) error {
	return b.bhl.Push(b.bhl.MaxAvailableHeight()+1, h)
}

// BlockHash returns the block hash for the given height
func (b *blocks) BlockHash(height uint64) (gethCommon.Hash, error) {
	_, hash, err := b.bhl.BlockHashByHeight(height)
	return hash, err
}
