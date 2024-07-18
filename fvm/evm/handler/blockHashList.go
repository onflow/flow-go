package handler

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	gethCommon "github.com/onflow/go-ethereum/common"
)

const (
	hashCountPerBucket   = 16
	capacityEncodingSize = 8
	tailEncodingSize     = 8
	countEncodingSize    = 8
	heightEncodingSize   = 8
	metaEncodingSize     = capacityEncodingSize +
		tailEncodingSize +
		countEncodingSize +
		heightEncodingSize
	hashEncodingSize = 32

	BlockHashListMetaKey         = "BlockHashListMeta"
	BlockHashListBucketKeyFormat = "BlockHashListBucket%d"
)

// TODO check legacy,
// - check if the size of capacity doesn't match migrate

// BlockHashList holds the last `capacity` number of block hashes in the list
type BlockHashList struct {
	// blocks      []gethCommon.Hash
	backend     types.Backend
	rootAddress flow.Address

	// cached meta data
	capacity int
	tail     int    // index to write to
	count    int    // number of elements (count <= capacity)
	height   uint64 // keeps the height of last added block
}

// NewBlockHashList constructs a new block hash list
// it tries to load the data from the backend
// and if not exist it will create one
func NewBlockHashList(
	backend types.Backend,
	rootAddress flow.Address,
	capacity int,
) (*BlockHashList, error) {
	bhl := &BlockHashList{
		backend:     backend,
		rootAddress: rootAddress,
		capacity:    capacity,
		tail:        0,
		count:       0,
		height:      0,
	}
	err := bhl.loadMetaData()
	if err != nil {
		return nil, err
	}
	// check capacity
	if bhl.capacity != capacity {
		return nil, fmt.Errorf("capacity error expected: %d, got: %d", bhl.capacity, capacity)
	}
	return bhl, nil
}

// Push pushes a block hash for the next height to the list.
// If the list is full, it overwrites the oldest element.
func (bhl *BlockHashList) Push(height uint64, bh gethCommon.Hash) error {
	if bhl.IsEmpty() && height != 0 {
		return fmt.Errorf("out of the order block hash, expected: 0, got: %d", height)
	}
	if !bhl.IsEmpty() && height != bhl.height+1 {
		return fmt.Errorf("out of the order block hash, expected: %d, got: %d", bhl.height+1, height)
	}
	err := bhl.updateBlockHashAt(bhl.tail, bh)
	if err != nil {
		return err
	}
	bhl.tail = (bhl.tail + 1) % bhl.capacity
	bhl.height = height
	if bhl.count != bhl.capacity {
		bhl.count++
	}
	return bhl.updateMetaData()
}

// IsEmpty returns true if the list is empty
func (bhl *BlockHashList) IsEmpty() bool {
	return bhl.count == 0
}

// LastAddedBlockHash returns the last block hash added to the list
// for empty list it returns empty hash value
func (bhl *BlockHashList) LastAddedBlockHash() (gethCommon.Hash, error) {
	if bhl.count == 0 {
		// return empty hash
		return gethCommon.Hash{}, nil
	}
	index := bhl.tail - 1
	if index < 0 {
		index = bhl.capacity - 1
	}
	return bhl.getBlockHashAt(index)
}

// MinAvailableHeight returns the min available height in the list
func (bhl *BlockHashList) MinAvailableHeight() uint64 {
	return bhl.height - (uint64(bhl.count) - 1)
}

// MaxAvailableHeight returns the max available height in the list
func (bhl *BlockHashList) MaxAvailableHeight() uint64 {
	return bhl.height
}

// BlockHashByIndex returns the block hash by block height
func (bhl *BlockHashList) BlockHashByHeight(height uint64) (found bool, bh gethCommon.Hash, err error) {
	if bhl.count == 0 || // empty
		height > bhl.height || // height too high
		height < bhl.MinAvailableHeight() { // height too low
		return false, gethCommon.Hash{}, nil
	}
	diff := bhl.height - height
	index := bhl.tail - int(diff) - 1
	if index < 0 {
		index = bhl.capacity + index
	}
	bh, err = bhl.getBlockHashAt(index)
	return true, bh, err
}

func (bhl *BlockHashList) updateBlockHashAt(idx int, bh gethCommon.Hash) error {
	// fetch the bucket
	bucketNumber := idx / hashCountPerBucket
	bucket, err := bhl.fetchBucket(bucketNumber)
	if err != nil {
		return err
	}
	start := (idx % hashCountPerBucket) * hashEncodingSize
	end := start + hashEncodingSize
	copy(bucket[start:end], bh.Bytes())
	return bhl.storeBucket(bucketNumber, bucket)
}

func (bhl *BlockHashList) fetchBucket(num int) ([]byte, error) {
	data, err := bhl.backend.GetValue(
		bhl.rootAddress[:],
		[]byte(fmt.Sprintf(BlockHashListBucketKeyFormat, num)),
	)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return make([]byte, hashCountPerBucket*hashEncodingSize), nil
	}
	return data, err
}

func (bhl *BlockHashList) storeBucket(num int, bucket []byte) error {
	return bhl.backend.SetValue(
		bhl.rootAddress[:],
		[]byte(fmt.Sprintf(BlockHashListBucketKeyFormat, num)),
		bucket,
	)
}

func (bhl *BlockHashList) getBlockHashAt(idx int) (gethCommon.Hash, error) {
	// fetch the bucket
	bucket, err := bhl.fetchBucket(idx / hashCountPerBucket)
	if err != nil {
		return gethCommon.Hash{}, err
	}
	// return the hash
	start := (idx % hashCountPerBucket) * hashEncodingSize
	end := start + hashEncodingSize
	return gethCommon.BytesToHash(bucket[start:end]), nil
}

func (bhl *BlockHashList) loadMetaData() error {
	data, err := bhl.backend.GetValue(
		bhl.rootAddress[:],
		[]byte(BlockHashListMetaKey),
	)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		// don't do anything and return
		return nil
	}

	if len(data) < metaEncodingSize {
		return fmt.Errorf("encoded input too short: %d < %d", len(data), metaEncodingSize)
	}

	pos := 0
	// decode capacity
	bhl.capacity = int(binary.BigEndian.Uint64(data[pos:]))
	pos += capacityEncodingSize

	// decode tail
	bhl.tail = int(binary.BigEndian.Uint64(data[pos:]))
	pos += tailEncodingSize

	// decode count
	bhl.count = int(binary.BigEndian.Uint64(data[pos:]))
	pos += countEncodingSize

	// decode height
	bhl.height = binary.BigEndian.Uint64(data[pos:])

	return nil
}

func (bhl *BlockHashList) updateMetaData() error {
	// encode meta data
	buffer := make([]byte, metaEncodingSize)
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

	return bhl.backend.SetValue(
		bhl.rootAddress[:],
		[]byte(BlockHashListMetaKey),
		buffer,
	)
}
