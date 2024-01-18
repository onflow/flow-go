package handler

import (
	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

var FlexLatestBlockKey = "LatestBlock"

type BlockStore struct {
	led           atree.Ledger
	flexAddress   flow.Address
	blockProposal *types.Block
}

var _ types.BlockStore = &BlockStore{}

// NewBlockStore constructs a new block store
func NewBlockStore(led atree.Ledger, flexAddress flow.Address) (*BlockStore, error) {
	return &BlockStore{
		led:         led,
		flexAddress: flexAddress,
	}, nil
}

// BlockProposal returns the block proposal to be updated by the handler
func (bs *BlockStore) BlockProposal() (*types.Block, error) {
	if bs.blockProposal != nil {
		return bs.blockProposal, nil
	}

	lastExecutedBlock, err := bs.LatestBlock()
	if err != nil {
		return nil, err
	}

	parentHash, err := lastExecutedBlock.Hash()
	if err != nil {
		return nil, err
	}

	bs.blockProposal = &types.Block{
		Height:            lastExecutedBlock.Height + 1,
		ParentBlockHash:   parentHash,
		TotalSupply:       lastExecutedBlock.TotalSupply,
		TransactionHashes: make([]gethCommon.Hash, 0),
	}
	return bs.blockProposal, nil
}

// CommitBlockProposal commits the block proposal to the chain
func (bs *BlockStore) CommitBlockProposal() error {
	bp, err := bs.BlockProposal()
	if err != nil {
		return err
	}

	blockBytes, err := bp.ToBytes()
	if err != nil {
		return types.NewFatalError(err)
	}

	err = bs.led.SetValue(bs.flexAddress[:], []byte(FlexLatestBlockKey), blockBytes)
	if err != nil {
		return types.NewFatalError(err)
	}

	bs.blockProposal = nil

	return nil
}

// ResetBlockProposal resets the block proposal
func (bs *BlockStore) ResetBlockProposal() error {
	bs.blockProposal = nil
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
//
// TODO: implement this properly to keep the last 256 block hashes
// and connect use it inside the handler to pass as a config to the emulator
func (bs *BlockStore) BlockHash(height int) (gethCommon.Hash, error) {
	return gethCommon.Hash{}, nil
}
