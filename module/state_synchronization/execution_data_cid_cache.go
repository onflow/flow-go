package state_synchronization

import (
	"errors"
	"sync"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/flow"
)

type ExecutionDataCIDCache interface {
	Get(c cid.Cid) (BlobRecord, error)
	Insert(header *flow.Header, blobTree BlobTree)
	BlobTreeRecords() uint
	BlobRecords() uint
}

type ExecutionDataCIDCacheImpl struct {
	mu            sync.RWMutex
	blobs         map[cid.Cid]BlobRecord
	blobTrees     []*BlobTreeRecord
	blobTreeIndex int
	size          uint
}

func NewExecutionDataCIDCache(size uint) *ExecutionDataCIDCacheImpl {
	return &ExecutionDataCIDCacheImpl{
		blobs:     make(map[cid.Cid]BlobRecord),
		blobTrees: make([]*BlobTreeRecord, 0, size),
		size:      size,
	}
}

var ErrCacheMiss = errors.New("CID not found in cache")

type BlobTreeRecord struct {
	BlockID     flow.Identifier
	BlockHeight uint64
	BlobTree    BlobTree
}

type BlobTreeLocation struct {
	Height uint
	Index  uint
}

type BlobRecord struct {
	BlobTreeRecord   *BlobTreeRecord
	BlobTreeLocation BlobTreeLocation
}

func (e *ExecutionDataCIDCacheImpl) Get(c cid.Cid) (BlobRecord, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if record, ok := e.blobs[c]; ok {
		return record, nil
	} else {
		return BlobRecord{}, ErrCacheMiss
	}
}

func (e *ExecutionDataCIDCacheImpl) BlobTreeRecords() uint {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return uint(len(e.blobTrees))
}

func (e *ExecutionDataCIDCacheImpl) BlobRecords() uint {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return uint(len(e.blobs))
}

func (e *ExecutionDataCIDCacheImpl) insertBlobTree(header *flow.Header, blobTree BlobTree) *BlobTreeRecord {
	blobTreeRecord := &BlobTreeRecord{
		BlockID:     header.ID(),
		BlockHeight: header.Height,
		BlobTree:    blobTree,
	}

	if len(e.blobTrees) < int(e.size) {
		e.blobTrees = append(e.blobTrees, blobTreeRecord)
	} else {
		evictee := e.blobTrees[e.blobTreeIndex]

		for _, cids := range evictee.BlobTree {
			for _, cid := range cids {
				if blobRecord, ok := e.blobs[cid]; ok {
					if blobRecord.BlobTreeRecord == evictee {
						delete(e.blobs, cid)
					}
				}
			}
		}

		e.blobTrees[e.blobTreeIndex] = blobTreeRecord
	}

	e.blobTreeIndex = (e.blobTreeIndex + 1) % int(e.size)

	return blobTreeRecord
}

func (e *ExecutionDataCIDCacheImpl) Insert(header *flow.Header, blobTree BlobTree) {
	e.mu.Lock()
	defer e.mu.Unlock()

	blobTreeRecord := e.insertBlobTree(header, blobTree)

	for height, cids := range blobTree {
		for index, cid := range cids {
			e.blobs[cid] = BlobRecord{
				BlobTreeRecord: blobTreeRecord,
				BlobTreeLocation: BlobTreeLocation{
					Height: uint(height),
					Index:  uint(index),
				},
			}
		}
	}
}
