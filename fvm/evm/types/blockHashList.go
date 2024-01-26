package types

import (
	"encoding/binary"
	"fmt"

	gethCommon "github.com/ethereum/go-ethereum/common"
)

const (
	capacityEncodingSize = 8
	tailEncodingSize     = 8
	countEncodingSize    = 8
	heightEncodingSize   = 8
	hashEncodingSize     = 32
	minEncodedByteSize   = capacityEncodingSize +
		tailEncodingSize +
		countEncodingSize +
		heightEncodingSize
)

// BlockHashList holds the last `capacity` number of block hashes in the list
type BlockHashList struct {
	blocks   []gethCommon.Hash
	capacity int
	tail     int // element index to write to
	count    int // number of elements (count <= capacity)
	height   int // keeps the height of last added block
}

// NewBlockHashList constructs a new block hash list of the given capacity
func NewBlockHashList(capacity int) *BlockHashList {
	return &BlockHashList{
		blocks:   make([]gethCommon.Hash, capacity),
		capacity: capacity,
		tail:     0,
		count:    0,
		height:   0,
	}
}

// Push pushes a block hash for the next height to the list.
// If the list is full, it overwrites the oldest element.
func (bhl *BlockHashList) Push(height int, bh gethCommon.Hash) error {
	if bhl.IsEmpty() && height != 0 {
		return fmt.Errorf("out of our block hash push expected: 0, got: %d", height)
	}
	if !bhl.IsEmpty() && height != bhl.height+1 {
		return fmt.Errorf("out of our block hash push expected: %d, got: %d", bhl.height+1, height)
	}
	bhl.blocks[bhl.tail] = bh
	bhl.tail = (bhl.tail + 1) % bhl.capacity
	bhl.height = height
	if bhl.count != bhl.capacity {
		bhl.count++
	}
	return nil
}

// IsEmpty returns true if the list is empty
func (bhl *BlockHashList) IsEmpty() bool {
	return bhl.count == 0
}

// LastAddedBlockHash returns the last block hash added to the list
// for empty list it returns empty hash value
func (bhl *BlockHashList) LastAddedBlockHash() gethCommon.Hash {
	if bhl.count == 0 {
		// return empty hash
		return gethCommon.Hash{}
	}
	indx := bhl.tail - 1
	if indx < 0 {
		indx = bhl.capacity - 1
	}
	return bhl.blocks[indx]
}

// BlockHashByIndex returns the block hash by block height
func (bhl *BlockHashList) BlockHashByHeight(height int) (found bool, bh gethCommon.Hash) {
	if bhl.count == 0 {
		return false, gethCommon.Hash{}
	}
	if height > bhl.height {
		return false, gethCommon.Hash{}
	}
	if height < bhl.height-(bhl.count-1) {
		return false, gethCommon.Hash{}
	}
	diff := bhl.height - height
	indx := bhl.tail - int(diff) - 1
	if indx < 0 {
		indx = bhl.capacity + indx
	}
	return true, bhl.blocks[indx]
}

func (bhl *BlockHashList) Encode() []byte {
	encodedByteSize := capacityEncodingSize +
		tailEncodingSize +
		countEncodingSize +
		heightEncodingSize +
		len(bhl.blocks)*hashEncodingSize

	buffer := make([]byte, encodedByteSize)
	pos := 0

	// encode capacity
	binary.BigEndian.PutUint64(buffer[pos:], uint64(bhl.capacity))
	pos += capacityEncodingSize

	// encode tail
	binary.BigEndian.PutUint64(buffer[pos:], uint64(bhl.tail))
	pos += tailEncodingSize

	// encode count
	binary.BigEndian.PutUint64(buffer[pos:], uint64(bhl.count))
	pos += countEncodingSize

	// encode height
	binary.BigEndian.PutUint64(buffer[pos:], uint64(bhl.height))
	pos += heightEncodingSize

	// encode hashes
	for i := 0; i < bhl.count; i++ {
		copy(buffer[pos:pos+hashEncodingSize], bhl.blocks[i][:])
		pos += hashEncodingSize
	}
	return buffer
}

func NewBlockHashListFromEncoded(encoded []byte) (*BlockHashList, error) {
	if len(encoded) < minEncodedByteSize {
		return nil, fmt.Errorf("encoded input too short: %d < %d", len(encoded), minEncodedByteSize)
	}

	pos := 0
	// decode capacity
	capacity := binary.BigEndian.Uint64(encoded[pos:])
	pos += capacityEncodingSize

	// create bhl
	bhl := NewBlockHashList(int(capacity))

	// decode tail
	bhl.tail = int(binary.BigEndian.Uint64(encoded[pos:]))
	pos += tailEncodingSize

	// decode count
	bhl.count = int(binary.BigEndian.Uint64(encoded[pos:]))
	pos += countEncodingSize

	// decode height
	bhl.height = int(binary.BigEndian.Uint64(encoded[pos:]))
	pos += heightEncodingSize

	// decode hashes
	if len(encoded[pos:]) < bhl.count*hashEncodingSize {
		return nil, fmt.Errorf("encoded input too short: %d < %d", len(encoded), minEncodedByteSize)
	}
	for i := 0; i < bhl.count; i++ {
		bhl.blocks[i] = gethCommon.BytesToHash(encoded[pos : pos+hashEncodingSize])
		pos += hashEncodingSize
	}

	return bhl, nil
}
