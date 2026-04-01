package environment

import (
	"fmt"
	"time"

	gethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// BlockStore stores the chain of blocks
type EVMBlockStore interface {
	// LatestBlock returns the latest appended block
	LatestBlock() (*types.Block, error)

	// BlockHash returns the hash of the block at the given height
	BlockHash(height uint64) (gethCommon.Hash, error)

	// BlockProposal returns the active block proposal
	BlockProposal() (*types.BlockProposal, error)

	// StageBlockProposal updates the in-memory block proposal without writing to
	// storage. Persistence happens at transaction end via FlushBlockProposal.
	StageBlockProposal(*types.BlockProposal)

	// FlushBlockProposal writes the cached block proposal to storage. Called by the
	// fvm.Environment at the end of each Cadence transaction.
	FlushBlockProposal() error

	// CommitBlockProposal commits the block proposal and update the chain of blocks
	CommitBlockProposal(*types.BlockProposal) error

	// Reset discards any staged but unflushed block proposal. Called by the
	// fvm.Environment when a transaction fails.
	ResetBlockProposal()
}

const (
	BlockHashListCapacity            = 256
	BlockStoreLatestBlockKey         = "LatestBlock"
	BlockStoreLatestBlockProposalKey = "LatestBlockProposal"
)

type BlockStore struct {
	chainID     flow.ChainID
	storage     ValueStore
	blockInfo   BlockInfo
	randGen     RandomGenerator
	rootAddress flow.Address
	cached      *types.BlockProposal
}

var _ EVMBlockStore = &BlockStore{}

// NewBlockStore constructs a new block store
func NewBlockStore(
	chainID flow.ChainID,
	storage ValueStore,
	blockInfo BlockInfo,
	randGen RandomGenerator,
	rootAddress flow.Address,
) *BlockStore {
	return &BlockStore{
		chainID:     chainID,
		storage:     storage,
		blockInfo:   blockInfo,
		randGen:     randGen,
		rootAddress: rootAddress,
	}
}

// BlockProposal returns the block proposal to be updated by the handler
func (bs *BlockStore) BlockProposal() (*types.BlockProposal, error) {
	if bs.cached != nil {
		return bs.cached, nil
	}
	// first fetch it from the storage
	data, err := bs.storage.GetValue(bs.rootAddress[:], []byte(BlockStoreLatestBlockProposalKey))
	if err != nil {
		return nil, err
	}
	if len(data) != 0 {
		bp, err := types.NewBlockProposalFromBytes(data)
		if err != nil {
			return nil, err
		}
		bs.cached = bp
		return bp, nil
	}
	bp, err := bs.constructBlockProposal()
	if err != nil {
		return nil, err
	}
	bs.cached = bp
	return bp, nil
}

func (bs *BlockStore) constructBlockProposal() (*types.BlockProposal, error) {
	// if available construct a new one
	cadenceHeight, err := bs.blockInfo.GetCurrentBlockHeight()
	if err != nil {
		return nil, err
	}

	cadenceBlock, found, err := bs.blockInfo.GetBlockAtHeight(cadenceHeight)
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

	// read a random value for block proposal
	prevrandao := gethCommon.Hash{}
	err = bs.randGen.ReadRandom(prevrandao[:])
	if err != nil {
		return nil, err
	}

	blockProposal := types.NewBlockProposal(
		parentHash,
		lastExecutedBlock.Height+1,
		timestamp,
		lastExecutedBlock.TotalSupply,
		prevrandao,
	)

	return blockProposal, nil
}

// UpdateBlockProposal updates the block proposal
func (bs *BlockStore) updateBlockProposal(bp *types.BlockProposal) error {
	blockProposalBytes, err := bp.ToBytes()
	if err != nil {
		return types.NewFatalError(err)
	}

	return bs.storage.SetValue(
		bs.rootAddress[:],
		[]byte(BlockStoreLatestBlockProposalKey),
		blockProposalBytes,
	)
}

// CommitBlockProposal commits the block proposal to the chain
func (bs *BlockStore) CommitBlockProposal(bp *types.BlockProposal) error {
	bp.PopulateRoots()

	blockBytes, err := bp.Block.ToBytes()
	if err != nil {
		return types.NewFatalError(err)
	}

	err = bs.storage.SetValue(bs.rootAddress[:], []byte(BlockStoreLatestBlockKey), blockBytes)
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

	// construct a new block proposal and store
	newBP, err := bs.constructBlockProposal()
	if err != nil {
		return err
	}
	err = bs.updateBlockProposal(newBP)
	if err != nil {
		return err
	}
	bs.cached = newBP
	return nil
}

// LatestBlock returns the latest executed block
func (bs *BlockStore) LatestBlock() (*types.Block, error) {
	data, err := bs.storage.GetValue(bs.rootAddress[:], []byte(BlockStoreLatestBlockKey))
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
	bhl, err := NewBlockHashList(bs.storage, bs.rootAddress, BlockHashListCapacity)
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

func (bs *BlockStore) ResetBlockProposal() {
	bs.cached = nil
}

func (bs *BlockStore) StageBlockProposal(bp *types.BlockProposal) {
	bs.cached = bp
}

func (bs *BlockStore) FlushBlockProposal() error {
	if bs.cached == nil {
		return nil
	}
	err := bs.updateBlockProposal(bs.cached)
	if err != nil {
		return err
	}
	return nil
}

type NoEVMBlockStore struct{}

var _ EVMBlockStore = &NoEVMBlockStore{}

func (bs NoEVMBlockStore) BlockProposal() (*types.BlockProposal, error) {
	return nil, nil
}

func (bs NoEVMBlockStore) CommitBlockProposal(bp *types.BlockProposal) error {
	return nil
}

func (bs NoEVMBlockStore) LatestBlock() (*types.Block, error) {
	return nil, nil
}

func (bs NoEVMBlockStore) BlockHash(height uint64) (gethCommon.Hash, error) {
	return gethCommon.Hash{}, nil
}

func (bs NoEVMBlockStore) ResetBlockProposal() {}

func (bs NoEVMBlockStore) StageBlockProposal(bp *types.BlockProposal) {}

func (bs NoEVMBlockStore) FlushBlockProposal() error {
	return nil
}
