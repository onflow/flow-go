package flex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/model/flow"
)

var FlexLatestBlockKey = "LatestBlock"

type BlockStore struct {
	led         atree.Ledger
	flexAddress flow.Address
}

var _ models.BlockChain = &BlockStore{}

// NewBlockStore constructs a new block store
func NewBlockStore(led atree.Ledger, flexAddress flow.Address) (*BlockStore, error) {
	return &BlockStore{
		led:         led,
		flexAddress: flexAddress,
	}, nil
}

// AppendBlock appends a block to the chain
func (bs *BlockStore) AppendBlock(block *models.FlexBlock) error {
	blockBytes, err := block.ToBytes()
	if err != nil {
		return models.NewFatalError(err)
	}
	err = bs.led.SetValue(bs.flexAddress[:], []byte(FlexLatestBlockKey), blockBytes)
	if err != nil {
		return models.NewFatalError(err)
	}
	return nil
}

// LatestBlock returns the latest executed block
func (bs *BlockStore) LatestBlock() (*models.FlexBlock, error) {
	data, err := bs.led.GetValue(bs.flexAddress[:], []byte(FlexLatestBlockKey))
	if len(data) == 0 {
		return models.GenesisFlexBlock, err
	}
	if err != nil {
		return nil, models.NewFatalError(err)
	}
	return models.NewFlexBlockFromBytes(data)
}

// BlockHash returns the block hash for the last x blocks
// TODO: implement this properly to keep the last 256 block hashes
// and connect use it inside the handler to pass as a config to the emulator
func (bs *BlockStore) BlockHash(height int) (common.Hash, error) {
	return common.Hash{}, nil
}
