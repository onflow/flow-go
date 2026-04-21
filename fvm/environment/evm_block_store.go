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

	// CommitBlockProposal commits the block proposal and update the chain of blocks
	CommitBlockProposal(*types.BlockProposal) error
}

const (
	BlockHashListCapacity            = 256
	BlockStoreLatestBlockKey         = "LatestBlock"
	BlockStoreLatestBlockProposalKey = "LatestBlockProposal"
)

// BlockStore manages EVM block storage and block proposal lifecycle during Flow block execution.
//
// Storage Keys:
//   - LatestBlock: The last finalized EVM block. Updated only at CommitBlockProposal().
//   - LatestBlockProposal: The in-progress EVM block accumulating transactions.
//     Its parent hash must equal hash(LatestBlock) and height must equal LatestBlock.Height + 1.
//
// This implementation performs storage operations on every call (for metering compatibility)
// but uses caching to avoid repeated deserialization. This ensures metered computation
// matches the old implementation while still providing some performance benefit.
//
// Flow Block K Execution:
//
//	├── Cadence tx 1 (succeed)
//	│   ├── EVM Tx A
//	│   │   ├── BlockProposal()        → storage read (metered), deserialize, cache
//	│   │   └── StageBlockProposal()   → storage write (metered), update cache
//	│   ├── EVM Tx B
//	│   │   ├── BlockProposal()        → storage read (metered), return from cache
//	│   │   └── StageBlockProposal()   → storage write (metered), update cache
//	│   └── [tx end]
//	│       └── FlushBlockProposal()   → no-op (writes already happened)
//	│
//	├── Cadence tx 2 (failed)
//	│   ├── EVM Tx C/D                 → same as above
//	│   └── [tx fail/revert]
//	│       └── Reset()                → clear cache (storage rolled back by FVM)
//	│
//	└── System chunk tx (last)
//	    └── heartbeat()
//	        └── CommitBlockProposal()
//	            ├── write LatestBlock
//	            └── write new LatestBlockProposal (for next flow block)
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

// BlockProposal returns the block proposal to be updated by the handler.
// Always performs storage read for metering compatibility, but returns
// cached value if available to avoid deserialization overhead.
func (bs *BlockStore) BlockProposal() (*types.BlockProposal, error) {
	// Always read from storage for metering purposes
	data, err := bs.storage.GetValue(bs.rootAddress[:], []byte(BlockStoreLatestBlockProposalKey))
	if err != nil {
		return nil, err
	}

	// If cached, return from cache (avoids deserialization)
	if bs.cached != nil {
		return bs.cached, nil
	}

	// First call - deserialize and cache
	if len(data) != 0 {
		bp, err := types.NewBlockProposalFromBytes(data)
		if err != nil {
			return nil, err
		}
		bs.cached = bp
		return bp, nil
	}

	// Storage empty - construct new proposal and write immediately
	bp, err := bs.constructBlockProposal()
	if err != nil {
		return nil, err
	}
	err = bs.updateBlockProposal(bp)
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

	// Construct and store the new block proposal eagerly to maintain
	// state compatibility with the previous implementation.
	newBP, err := bs.constructBlockProposal()
	if err != nil {
		return err
	}
	err = bs.updateBlockProposal(newBP)
	if err != nil {
		return err
	}
	bs.cached = nil
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

// StageBlockProposal writes the block proposal to storage and updates the cache.
// Always performs storage write for metering compatibility.
func (bs *BlockStore) StageBlockProposal(bp *types.BlockProposal) {
	// Always write to storage for metering purposes
	err := bs.updateBlockProposal(bp)
	if err != nil {
		panic(types.NewFatalError(err))
	}
	// Update cache to avoid deserialization on next BlockProposal() call
	bs.cached = bp
}

// FlushBlockProposal is a no-op since StageBlockProposal writes immediately.
// Kept for interface compatibility.
func (bs *BlockStore) FlushBlockProposal() error {
	return nil
}
