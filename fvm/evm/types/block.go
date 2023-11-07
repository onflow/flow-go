package types

import (
	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// Block represents a evm block.
// It captures block info such as height and state
type Block struct {
	// the hash of the parent block
	ParentBlockHash gethCommon.Hash

	// Height returns the height of this block
	Height uint64

	// holds the total amount of the native token deposited in the evm side.
	TotalSupply uint64

	// StateRoot returns the EVM root hash of the state after executing this block
	StateRoot gethCommon.Hash

	// ReceiptRoot returns the root hash of the receipts emitted in this block
	ReceiptRoot gethCommon.Hash

	// transaction hashes
	TransactionHashes []gethCommon.Hash
}

func (b *Block) ToBytes() ([]byte, error) {
	return rlp.EncodeToBytes(b)
}

func (b *Block) Hash() (gethCommon.Hash, error) {
	data, err := b.ToBytes()
	return gethCommon.BytesToHash(data), err
}

// NewBlock constructs a new block
func NewBlock(height, uuidIndex, totalSupply uint64,
	stateRoot, receiptRoot gethCommon.Hash,
	txHashes []gethCommon.Hash,
) *Block {
	return &Block{
		Height:            height,
		TotalSupply:       totalSupply,
		StateRoot:         stateRoot,
		ReceiptRoot:       receiptRoot,
		TransactionHashes: txHashes,
	}
}

func NewBlockFromBytes(encoded []byte) (*Block, error) {
	res := &Block{}
	err := rlp.DecodeBytes(encoded, res)
	return res, err
}

// GenesisBlock is the genesis block in the EVM environment
var GenesisBlock = &Block{
	ParentBlockHash: gethCommon.Hash{},
	Height:          uint64(0),
	StateRoot:       gethTypes.EmptyRootHash,
	ReceiptRoot:     gethTypes.EmptyRootHash,
}

// BlockChain stores the chain of blocks
type BlockChain interface {
	// Appends a block to the block chain
	AppendBlock(block *Block) error

	// LatestBlock returns the latest appended block
	LatestBlock() (*Block, error)

	// returns the hash of the block at the given height
	BlockHash(height int) (gethCommon.Hash, error)
}
