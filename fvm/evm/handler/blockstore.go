package handler

import (
	"fmt"
	"time"

	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	BlockHashListCapacity            = 256
	BlockStoreLatestBlockKey         = "LatestBlock"
	BlockStoreLatestBlockProposalKey = "LatestBlockProposal"
)

type BlockStore struct {
	chainID     flow.ChainID
	backend     types.Backend
	rootAddress flow.Address
}

var _ types.BlockStore = &BlockStore{}

// NewBlockStore constructs a new block store
func NewBlockStore(
	chainID flow.ChainID,
	backend types.Backend,
	rootAddress flow.Address,
) *BlockStore {
	return &BlockStore{
		chainID:     chainID,
		backend:     backend,
		rootAddress: rootAddress,
	}
}

// BlockProposal returns the block proposal to be updated by the handler
func (bs *BlockStore) BlockProposal() (*types.BlockProposal, error) {
	// first fetch it from the storage
	data, err := bs.backend.GetValue(bs.rootAddress[:], []byte(BlockStoreLatestBlockProposalKey))
	if err != nil {
		return nil, err
	}
	if len(data) != 0 {
		return types.NewBlockProposalFromBytes(data)
	}

	// if available construct a new one
	cadenceHeight, err := bs.backend.GetCurrentBlockHeight()
	if err != nil {
		return nil, err
	}

	cadenceBlock, found, err := bs.backend.GetBlockAtHeight(cadenceHeight)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("cadence block not found")
	}

	lastExecutedBlock, err := bs.LatestBlock()
	if err != nil {
		return nil, err
	}

	parentHash, err := lastExecutedBlock.Hash()
	if err != nil {
		return nil, err
	}

	// cadence block timestamp is unix nanoseconds but evm blocks
	// expect timestamps in unix seconds so we convert here
	timestamp := uint64(cadenceBlock.Timestamp / int64(time.Second))

	blockProposal := types.NewBlockProposal(
		parentHash,
		lastExecutedBlock.Height+1,
		timestamp,
		lastExecutedBlock.TotalSupply,
	)
	return blockProposal, nil
}

// UpdateBlockProposal updates the block proposal
func (bs *BlockStore) UpdateBlockProposal(bp *types.BlockProposal) error {
	blockProposalBytes, err := bp.ToBytes()
	if err != nil {
		return types.NewFatalError(err)
	}

	return bs.backend.SetValue(
		bs.rootAddress[:],
		[]byte(BlockStoreLatestBlockProposalKey),
		blockProposalBytes,
	)
}

func (bs *BlockStore) ResetBlockProposal() error {
	return bs.backend.SetValue(
		bs.rootAddress[:],
		[]byte(BlockStoreLatestBlockProposalKey),
		nil,
	)
}

// CommitBlockProposal commits the block proposal to the chain
func (bs *BlockStore) CommitBlockProposal(bp *types.BlockProposal) error {
	bp.PopulateRoots()

	blockBytes, err := bp.Block.ToBytes()
	if err != nil {
		return types.NewFatalError(err)
	}

	err = bs.backend.SetValue(bs.rootAddress[:], []byte(BlockStoreLatestBlockKey), blockBytes)
	if err != nil {
		return err
	}

	hash, err := bp.Block.Hash()
	if err != nil {
		return err
	}

	bhl, err := bs.getBlockHashList()
	if err != nil {
		return err
	}
	err = bhl.Push(bp.Block.Height, hash)
	if err != nil {
		return err
	}

	err = bs.ResetBlockProposal()
	if err != nil {
		return err
	}

	return nil
}

// LatestBlock returns the latest executed block
func (bs *BlockStore) LatestBlock() (*types.Block, error) {
	data, err := bs.backend.GetValue(bs.rootAddress[:], []byte(BlockStoreLatestBlockKey))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return types.GenesisBlock(bs.chainID), nil
	}
	return types.NewBlockFromBytes(data)
}

// BlockHash returns the block hash for the last x blocks
func (bs *BlockStore) BlockHash(height uint64) (gethCommon.Hash, error) {
	bhl, err := bs.getBlockHashList()
	if err != nil {
		return gethCommon.Hash{}, err
	}
	_, hash, err := bhl.BlockHashByHeight(height)
	return hash, err
}

func (bs *BlockStore) getBlockHashList() (*BlockHashList, error) {
	bhl, err := NewBlockHashList(bs.backend, bs.rootAddress, BlockHashListCapacity)
	if err != nil {
		return nil, err
	}

	if bhl.IsEmpty() {
		err = bhl.Push(
			types.GenesisBlock(bs.chainID).Height,
			types.GenesisBlockHash(bs.chainID),
		)
		if err != nil {
			return nil, err
		}
	}

	return bhl, nil
}
