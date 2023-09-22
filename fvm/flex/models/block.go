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

	// StateRoot returns the EVM root hash of the state after executing this block
	StateRoot gethCommon.Hash

	// EventRoot returns the EVM root hash of the events emitted during execution of this block
	EventRoot gethCommon.Hash
}

const (
	encodedUInt64Size = 8
	encodedHashSize   = 32

	// ( height + uuid + state root + event root)
	encodedBlockSize = 8 + 8 + 32 + 32
)

func (b *FlexBlock) ToBytes() []byte {
	encoded := make([]byte, encodedBlockSize)
	var index int
	// encode height first
	binary.BigEndian.PutUint64(encoded[index:index+encodedUInt64Size], b.Height)
	index += encodedUInt64Size
	// encode the uuid index
	binary.BigEndian.PutUint64(encoded[index:index+encodedUInt64Size], b.UUIDIndex)
	index += encodedUInt64Size
	// encode state root
	copy(encoded[index:index+encodedHashSize], b.StateRoot[:])
	index += encodedHashSize
	// encode event root
	copy(encoded[index:index+encodedHashSize], b.StateRoot[:])
	return encoded[:]
}

func NewFlexBlock(height, uuidIndex uint64, stateRoot, eventRoot gethCommon.Hash) *FlexBlock {
	return &FlexBlock{
		Height:    height,
		UUIDIndex: uuidIndex,
		StateRoot: stateRoot,
		EventRoot: eventRoot,
	}
}

func NewFlexBlockFromEncoded(encoded []byte) *FlexBlock {
	var index int
	height := binary.BigEndian.Uint64(encoded[index : index+encodedUInt64Size])
	index += encodedUInt64Size
	uuidIndex := binary.BigEndian.Uint64(encoded[index : index+encodedUInt64Size])
	index += encodedUInt64Size
	stateRoot := gethCommon.BytesToHash(encoded[index : index+encodedHashSize])
	index += encodedUInt64Size
	eventRoot := gethCommon.BytesToHash(encoded[index : index+encodedHashSize])
	return &FlexBlock{
		Height:    height,
		UUIDIndex: uuidIndex,
		StateRoot: stateRoot,
		EventRoot: eventRoot,
	}
}

var GenesisFlexBlock = &FlexBlock{
	Height:    uint64(0),
	UUIDIndex: uint64(1),
	StateRoot: gethTypes.EmptyRootHash,
	EventRoot: gethTypes.EmptyRootHash,
}
