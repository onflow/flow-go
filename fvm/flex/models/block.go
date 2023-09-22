package models

import (
	"encoding/binary"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

// FlexBlock represents a Flex block.
// It captures block info such as height and state
type FlexBlock struct {
	// Height returns the height of this block
	Height uint64

	// UUID Index
	UUIDIndex uint64

	// holds the total amount of the native token deposited in Flex
	TotalSupply uint64

	// StateRoot returns the EVM root hash of the state after executing this block
	StateRoot gethCommon.Hash

	// EventRoot returns the EVM root hash of the events emitted during execution of this block
	EventRoot gethCommon.Hash
}

const (
	// encodedUInt64Size      = 8
	// encodedHashSize        = 32
	encodedHeightSize      = 8
	encodedUUIDIndexSize   = 8
	encodedTotalSupplySize = 8
	encodedStateRootSize   = 32
	encodedEventRootSize   = 32
	// ( height + uuid + state root + event root)
	encodedBlockSize = encodedHeightSize +
		encodedUUIDIndexSize +
		encodedTotalSupplySize +
		encodedStateRootSize +
		encodedEventRootSize
)

func (b *FlexBlock) ToBytes() []byte {
	encoded := make([]byte, encodedBlockSize)
	var index int
	// encode height first
	binary.BigEndian.PutUint64(encoded[index:index+encodedHeightSize], b.Height)
	index += encodedHeightSize
	// encode the uuid index
	binary.BigEndian.PutUint64(encoded[index:index+encodedUUIDIndexSize], b.UUIDIndex)
	index += encodedUUIDIndexSize
	// encode the total supply
	binary.BigEndian.PutUint64(encoded[index:index+encodedTotalSupplySize], b.TotalSupply)
	index += encodedTotalSupplySize
	// encode state root
	copy(encoded[index:index+encodedStateRootSize], b.StateRoot[:])
	index += encodedStateRootSize
	// encode event root
	copy(encoded[index:index+encodedEventRootSize], b.StateRoot[:])
	return encoded[:]
}

func NewFlexBlock(height, uuidIndex, totalSupply uint64, stateRoot, eventRoot gethCommon.Hash) *FlexBlock {
	return &FlexBlock{
		Height:      height,
		UUIDIndex:   uuidIndex,
		TotalSupply: totalSupply,
		StateRoot:   stateRoot,
		EventRoot:   eventRoot,
	}
}

func NewFlexBlockFromEncoded(encoded []byte) *FlexBlock {
	var index int
	height := binary.BigEndian.Uint64(encoded[index : index+encodedHeightSize])
	index += encodedHeightSize
	uuidIndex := binary.BigEndian.Uint64(encoded[index : index+encodedUUIDIndexSize])
	index += encodedUUIDIndexSize
	totalSupply := binary.BigEndian.Uint64(encoded[index : index+encodedTotalSupplySize])
	index += encodedTotalSupplySize
	stateRoot := gethCommon.BytesToHash(encoded[index : index+encodedStateRootSize])
	index += encodedStateRootSize
	eventRoot := gethCommon.BytesToHash(encoded[index : index+encodedEventRootSize])
	return &FlexBlock{
		Height:      height,
		UUIDIndex:   uuidIndex,
		TotalSupply: totalSupply,
		StateRoot:   stateRoot,
		EventRoot:   eventRoot,
	}
}

var GenesisFlexBlock = &FlexBlock{
	Height:    uint64(0),
	UUIDIndex: uint64(1),
	StateRoot: gethTypes.EmptyRootHash,
	EventRoot: gethTypes.EmptyRootHash,
}
