package handler

import (
	gehtCommon "github.com/ethereum/go-ethereum/common"
	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

var FlexLatestBlockKey = "LatestBlock"

type BlockStore struct {
	led         atree.Ledger
	flexAddress flow.Address
}

var _ types.BlockChain = &BlockStore{}

// NewBlockStore constructs a new block store
func NewBlockStore(led atree.Ledger, flexAddress flow.Address) (*BlockStore, error) {
	return &BlockStore{
		led:         led,
		flexAddress: flexAddress,
	}, nil
}

// AppendBlock appends a block to the chain
func (bs *BlockStore) AppendBlock(block *types.Block) error {
	blockBytes, err := block.ToBytes()
	if err != nil {
		return types.NewFatalError(err)
	}
	err = bs.led.SetValue(bs.flexAddress[:], []byte(FlexLatestBlockKey), blockBytes)
	if err != nil {
		return types.NewFatalError(err)
	}
	return nil
}

// LatestBlock returns the latest executed block
func (bs *BlockStore) LatestBlock() (*types.Block, error) {
	data, err := bs.led.GetValue(bs.flexAddress[:], []byte(FlexLatestBlockKey))
	if len(data) == 0 {
		return types.GenesisBlock, err
	}
	if err != nil {
		return nil, types.NewFatalError(err)
	}
	return types.NewBlockFromBytes(data)
}

// BlockHash returns the block hash for the last x blocks
// TODO: implement this properly to keep the last 256 block hashes
// and connect use it inside the handler to pass as a config to the emulator
func (bs *BlockStore) BlockHash(height int) (gehtCommon.Hash, error) {
	return gehtCommon.Hash{}, nil
}
