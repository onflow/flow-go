package blocks

import (
	"fmt"

	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
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

var _ types.BlockSnapshot = (*Blocks)(nil)

// NewBlocks constructs a new blocks type
func NewBlocks(
	chainID flow.ChainID,
	rootAddress flow.Address,
	storage types.BackendStorage,
) (*Blocks, error) {
	var err error
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
		err = blocks.PushBlockMeta(
			NewMeta(
				genesis.Height,
				genesis.Timestamp,
				genesis.PrevRandao,
			))
		if err != nil {
			return nil, err
		}
		// push block hash
		err = blocks.PushBlockHash(
			genesis.Height,
			types.GenesisBlockHash(chainID))
		if err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

// PushBlock pushes a new block into the storage
func (b *Blocks) PushBlockMeta(
	meta *Meta,
) error {
	// check height order
	if meta.Height > 0 {
		bm, err := b.LatestBlock()
		if err != nil {
			return err
		}
		if meta.Height != bm.Height+1 {
			return fmt.Errorf("out of order block meta push! got: %d, expected %d ", meta.Height, bm.Height+1)
		}
	}
	return b.storeBlockMetaData(meta)
}

// PushBlockHash pushes a new block block hash into the storage
func (b *Blocks) PushBlockHash(
	height uint64,
	hash gethCommon.Hash,
) error {
	return b.bhl.Push(height, hash)
}

func (b *Blocks) LatestBlock() (*Meta, error) {
	return b.loadBlockMetaData()
}

// BlockHash returns the block hash for the given height
func (b *Blocks) BlockHash(height uint64) (gethCommon.Hash, error) {
	_, hash, err := b.bhl.BlockHashByHeight(height)
	return hash, err
}

// BlockContext constructs a block context for the latest block
func (b *Blocks) BlockContext() (types.BlockContext, error) {
	bm, err := b.LatestBlock()
	if err != nil {
		return types.BlockContext{}, err
	}

	return NewBlockContext(
		b.chainID,
		bm.Height,
		bm.Timestamp,
		func(n uint64) gethCommon.Hash {
			hash, err := b.BlockHash(n)
			if err != nil {
				panic(err)
			}
			return UseFixedHashList(b.chainID, bm.Height, n, hash)
		},
		bm.Random,
		nil,
	)
}

// storeBlockMetaData stores the block meta data into storage
func (b *Blocks) storeBlockMetaData(bm *Meta) error {
	// store the encoded data into backend
	return b.storage.SetValue(
		b.rootAddress[:],
		[]byte(BlockStoreLatestBlockMetaKey),
		bm.Encode(),
	)
}

// loadBlockMetaData loads the block meta data from the storage
func (b *Blocks) loadBlockMetaData() (*Meta, error) {
	data, err := b.storage.GetValue(
		b.rootAddress[:],
		[]byte(BlockStoreLatestBlockMetaKey),
	)
	if err != nil {
		return nil, err
	}
	return MetaFromEncoded(data)
}
