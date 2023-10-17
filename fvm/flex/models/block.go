package models

import (
	"github.com/ethereum/go-ethereum/common"
	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// FlexBlock represents a Flex block.
// It captures block info such as height and state
type FlexBlock struct {
	// the hash of the parent block
	ParentBlockHash gethCommon.Hash

	// Height returns the height of this block
	Height uint64

	// UUID Index
	UUIDIndex uint64

	// holds the total amount of the native token deposited in Flex
	TotalSupply uint64

	// StateRoot returns the EVM root hash of the state after executing this block
	StateRoot gethCommon.Hash

	// ReceiptRoot returns the root hash of the receipts emitted in this block
	ReceiptRoot gethCommon.Hash
}

func (b *FlexBlock) ToBytes() ([]byte, error) {
	return rlp.EncodeToBytes(b)
}

func (b *FlexBlock) Hash() (common.Hash, error) {
	data, err := b.ToBytes()
	return common.BytesToHash(data), err
}

func NewFlexBlock(height, uuidIndex, totalSupply uint64, stateRoot, receiptRoot gethCommon.Hash) *FlexBlock {
	return &FlexBlock{
		Height:      height,
		UUIDIndex:   uuidIndex,
		TotalSupply: totalSupply,
		StateRoot:   stateRoot,
		ReceiptRoot: receiptRoot,
	}
}

func NewFlexBlockFromBytes(encoded []byte) (*FlexBlock, error) {
	res := &FlexBlock{}
	err := rlp.DecodeBytes(encoded, res)
	return res, err
}

var GenesisFlexBlock = &FlexBlock{
	ParentBlockHash: gethCommon.Hash{},
	Height:          uint64(0),
	UUIDIndex:       uint64(1),
	StateRoot:       gethTypes.EmptyRootHash,
	ReceiptRoot:     gethTypes.EmptyRootHash,
}
