package sync

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	gethCommon "github.com/onflow/go-ethereum/common"
)

const BlockStoreLatestBlockMetaKey = "LatestBlockMeta"

// Blocks facilitates access to the recent block hash values
// and also the latest executed block meta data
type Blocks struct {
	chainID     flow.ChainID
	storage     types.BackendStorage
	rootAddress flow.Address
	bhl         *handler.BlockHashList
}

// NewBlocks constructs a new blocks type
func NewBlocks(
	chainID flow.ChainID,
	storage types.BackendStorage,
) (*Blocks, error) {
	var err error
	rootAddress := evm.StorageAccountAddress(chainID)
	blocks := &Blocks{
		chainID:     chainID,
		storage:     storage,
		rootAddress: rootAddress,
	}
	blocks.bhl, err = handler.NewBlockHashList(
		storage,
		rootAddress,
		handler.BlockHashListCapacity,
	)
	if err != nil {
		return nil, err
	}
	// if empty insert genesis block hash
	if blocks.bhl.IsEmpty() {
		genesis := types.GenesisBlock(chainID)
		err = blocks.PushBlock(
			genesis.Height,
			genesis.Timestamp,
			genesis.PrevRandao,
			types.GenesisBlockHash(chainID))
		if err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

func (b *Blocks) PushBlock(
	height uint64,
	timestamp uint64,
	prevRandao gethCommon.Hash,
	hash gethCommon.Hash,
) error {
	// check height order
	if height > 0 {
		bm, err := b.LatestBlock()
		if err != nil {
			return err
		}
		if height != bm.Height+1 {
			return fmt.Errorf("out of order block push got: %d, expected %d ", height, bm.Height+1)
		}
	}
	err := b.storeBlockMetaData(&BlockMeta{
		Height:    height,
		Timestamp: timestamp,
		Random:    prevRandao,
	})
	if err != nil {
		return err
	}
	return b.bhl.Push(height, hash)
}

func (b *Blocks) LatestBlock() (*BlockMeta, error) {
	return b.loadBlockMetaData()
}

// BlockHash returns the block hash for the given height
func (b *Blocks) BlockHash(height uint64) (gethCommon.Hash, error) {
	_, hash, err := b.bhl.BlockHashByHeight(height)
	return hash, err
}

// storeBlockMetaData stores the block meta data into storage
func (b *Blocks) storeBlockMetaData(bm *BlockMeta) error {
	// store the encoded data into backend
	return b.storage.SetValue(
		b.rootAddress[:],
		[]byte(BlockStoreLatestBlockMetaKey),
		bm.Encode(),
	)
}

// loadBlockMetaData loads the block meta data from the storage
func (b *Blocks) loadBlockMetaData() (*BlockMeta, error) {
	data, err := b.storage.GetValue(
		b.rootAddress[:],
		[]byte(BlockStoreLatestBlockMetaKey),
	)
	if err != nil {
		return nil, err
	}
	return BlockMetaFromEncoded(data)
}
