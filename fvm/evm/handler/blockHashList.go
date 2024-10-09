package handler

import (
	"encoding/binary"
	"fmt"

	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	blockHashListMetaKey         = "BlockHashListMeta"
	blockHashListBucketKeyFormat = "BlockHashListBucket%d"

	hashCountPerBucket   = 16
	hashEncodingSize     = 32
	capacityEncodingSize = 8
	tailEncodingSize     = 8
	countEncodingSize    = 8
	heightEncodingSize   = 8
	metaEncodingSize     = capacityEncodingSize +
		tailEncodingSize +
		countEncodingSize +
		heightEncodingSize
)

// BlockHashList stores the last `capacity` number of block hashes
//
// Under the hood it breaks the list of hashes into
// smaller fixed size buckets to minimize the
// number of bytes read and written during set/get operations.
type BlockHashList struct {
	backend     types.Backend
	rootAddress flow.Address

	// cached meta data
	capacity int
	tail     int    // index to write to
	count    int    // number of elements (count <= capacity)
	height   uint64 // keeps the height of last added block
}

// NewBlockHashList creates a block hash list
// It tries to load the metadata from the backend
// and if not exist it creates one
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
	// check the loaded capacity against the one provided
	if bhl.capacity != capacity {
		return nil, fmt.Errorf(
			"capacity doesn't match, expected: %d, got: %d",
			bhl.capacity,
			capacity,
		)
	}
	return bhl, nil
}

// Push pushes a block hash for the next height to the list.
// If the list has reached to the capacity, it overwrites the oldest element.
func (bhl *BlockHashList) Push(height uint64, bh gethCommon.Hash) error {
	// handle the very first block
	if bhl.IsEmpty() && height != 0 {
		return fmt.Errorf("out of the order block hash, expected: 0, got: %d", height)
	}
	// check the block heights before pushing
	if !bhl.IsEmpty() && height != bhl.height+1 {
		return fmt.Errorf("out of the order block hash, expected: %d, got: %d", bhl.height+1, height)
	}
	// updates the block hash stored at index
	err := bhl.updateBlockHashAt(bhl.tail, bh)
	if err != nil {
		return err
	}

	// update meta data
	bhl.tail = (bhl.tail + 1) % bhl.capacity
	bhl.height = height
	if bhl.count != bhl.capacity {
		bhl.count++
	}
	return bhl.storeMetaData()
}

// IsEmpty returns true if the list is empty
func (bhl *BlockHashList) IsEmpty() bool {
	return bhl.count == 0
}

// LastAddedBlockHash returns the last block hash added to the list
// for empty list it returns empty hash value
func (bhl *BlockHashList) LastAddedBlockHash() (gethCommon.Hash, error) {
	if bhl.IsEmpty() {
		// return empty hash
		return gethCommon.Hash{}, nil
	}
	index := (bhl.tail + bhl.capacity - 1) % bhl.capacity
	return bhl.getBlockHashAt(index)
}

// MinAvailableHeight returns the min available height in the list
func (bhl *BlockHashList) MinAvailableHeight() uint64 {
	if bhl.IsEmpty() {
		return 0
	}
	return bhl.height - (uint64(bhl.count) - 1)
}

// MaxAvailableHeight returns the max available height in the list
func (bhl *BlockHashList) MaxAvailableHeight() uint64 {
	return bhl.height
}

// BlockHashByIndex returns the block hash by block height
func (bhl *BlockHashList) BlockHashByHeight(height uint64) (found bool, bh gethCommon.Hash, err error) {
	if bhl.IsEmpty() ||
		height > bhl.MaxAvailableHeight() ||
		height < bhl.MinAvailableHeight() {
		return false, gethCommon.Hash{}, nil
	}
	// calculate the index to lookup
	diff := bhl.height - height
	index := (bhl.tail - int(diff) - 1 + bhl.capacity) % bhl.capacity
	bh, err = bhl.getBlockHashAt(index)
	return true, bh, err
}

// updateBlockHashAt updates block hash at index
func (bhl *BlockHashList) updateBlockHashAt(idx int, bh gethCommon.Hash) error {
	// fetch the bucket
	bucketNumber := idx / hashCountPerBucket
	bucket, err := bhl.fetchBucket(bucketNumber)
	if err != nil {
		return err
	}
	// update the block hash
	start := (idx % hashCountPerBucket) * hashEncodingSize
	end := start + hashEncodingSize
	copy(bucket[start:end], bh.Bytes())

	// store bucket
	return bhl.backend.SetValue(
		bhl.rootAddress[:],
		[]byte(fmt.Sprintf(blockHashListBucketKeyFormat, bucketNumber)),
		bucket,
	)
}

// fetchBucket fetches the bucket
func (bhl *BlockHashList) fetchBucket(num int) ([]byte, error) {
	data, err := bhl.backend.GetValue(
		bhl.rootAddress[:],
		[]byte(fmt.Sprintf(blockHashListBucketKeyFormat, num)),
	)
	if err != nil {
		return nil, err
	}
	// if not exist create and return an new empty buffer
	if len(data) == 0 {
		return make([]byte, hashCountPerBucket*hashEncodingSize), nil
	}
	return data, err
}

// returns the block hash at the given index
func (bhl *BlockHashList) getBlockHashAt(idx int) (gethCommon.Hash, error) {
	// fetch the bucket first
	bucket, err := bhl.fetchBucket(idx / hashCountPerBucket)
	if err != nil {
		return gethCommon.Hash{}, err
	}
	// return the hash from the bucket
	start := (idx % hashCountPerBucket) * hashEncodingSize
	end := start + hashEncodingSize
	return gethCommon.BytesToHash(bucket[start:end]), nil
}

// loadMetaData loads the meta data from the storage
func (bhl *BlockHashList) loadMetaData() error {
	data, err := bhl.backend.GetValue(
		bhl.rootAddress[:],
		[]byte(blockHashListMetaKey),
	)
	if err != nil {
		return err
	}
	// if data doesn't exist
	// return and keep the default values
	if len(data) == 0 {
		return nil
	}
	// check the data size
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

// storeMetaData stores the meta data into storage
func (bhl *BlockHashList) storeMetaData() error {
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
	binary.BigEndian.PutUint64(buffer[pos:], bhl.height)

	// store the encoded data into backend
	return bhl.backend.SetValue(
		bhl.rootAddress[:],
		[]byte(blockHashListMetaKey),
		buffer,
	)
}
