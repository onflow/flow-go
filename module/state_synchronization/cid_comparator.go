package state_synchronization

import (
	"container/heap"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type ExecutionDataCIDComparator interface {
	// Compare returns true if a has higher priority than b
	Compare(a, b cid.Cid) bool
}

type blobRecord struct {
	cid         cid.Cid
	isRoot      bool
	chunkIndex  uint
	height      uint
	blockID     flow.Identifier
	blockHeight uint64
}

// compareTo returns a negative integer, zero, or a positive integer as
// a has higher, equal, or lower priority than b
func (a *blobRecord) compareTo(b *blobRecord) int {
	// more recent block has higher priority
	if a.blockHeight > b.blockHeight {
		return 1
	} else if a.blockHeight == b.blockHeight {
		if a.blockID == b.blockID {
			// deeper node in the same blob tree has higher priority

			if a.isRoot {
				if b.isRoot {
					return 0
				} else {
					return -1
				}
			} else if b.isRoot {
				return 1
			}

			return int(b.height - a.height)
		}

		// nodes in different blob trees at the same block height have equal priority
		return 0
	} else {
		return -1
	}
}

type blobHeap []*blobRecord

func (h blobHeap) Len() int {
	return len(h)
}

func (h blobHeap) Less(i, j int) bool {
	// we want to evict lower priority elements first
	return h[i].compareTo(h[j]) < 0
}

func (h blobHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *blobHeap) Push(x interface{}) {
	*h = append(*h, x.(*blobRecord))
}

func (h *blobHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type ExecutionDataCIDComparatorImpl struct {
	mu    sync.RWMutex
	blobs sync.Map
	blobHeap
	size    uint
	inserts chan *insertRequest
}

type insertRequest struct {
	cids        []cid.Cid
	isRoot      bool
	chunkIndex  uint
	height      uint
	blockID     flow.Identifier
	blockHeight uint64
}

func NewExecutionDataCIDComparator(size uint) *ExecutionDataCIDComparatorImpl {
	return &ExecutionDataCIDComparatorImpl{
		blobHeap: make(blobHeap, size),
		size:     size,
		inserts:  make(chan *insertRequest, 16),
	}
}

func (e *ExecutionDataCIDComparatorImpl) Compare(a, b cid.Cid) bool {
	ra, ok := e.blobs.Load(a)
	if !ok {
		return false
	}

	rb, ok := e.blobs.Load(b)
	if !ok {
		return true
	}

	return ra.(*blobRecord).compareTo(rb.(*blobRecord)) > 0
}

func (e *ExecutionDataCIDComparatorImpl) processBlobRecord(ctx irrecoverable.SignalerContext, c cid.Cid, br *blobRecord) {
	previous, loaded := e.blobs.Load(c)

	if loaded && previous.(*blobRecord).compareTo(br) >= 0 {
		// an entry with higher or equal priority already exists for this cid
		return
	}

	e.blobs.Store(c, br)
	heap.Push(&e.blobHeap, br)

	// evict the lowest priority element if the heap is full
	if e.blobHeap.Len() > int(e.size) {
		evictee := heap.Pop(&e.blobHeap).(*blobRecord)

		if record, ok := e.blobs.Load(evictee.cid); ok {
			if record == evictee {
				e.blobs.Delete(evictee.cid)
			}
		} else {
			// this should never happen
			ctx.Throw(fmt.Errorf("attempted to evict cid that was not in the cache: %v", evictee.cid))
		}
	}
}

func (e *ExecutionDataCIDComparatorImpl) processInserts(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		case insert := <-e.inserts:
			for _, c := range insert.cids {
				e.processBlobRecord(ctx, c, &blobRecord{
					cid:         c,
					isRoot:      insert.isRoot,
					chunkIndex:  insert.chunkIndex,
					height:      insert.height,
					blockID:     insert.blockID,
					blockHeight: insert.blockHeight,
				})
			}
		}
	}
}

func (e *ExecutionDataCIDComparatorImpl) insert(
	cids []cid.Cid,
	isRoot bool,
	chunkIndex uint,
	height uint,
	blockID flow.Identifier,
	blockHeight uint64,
) {
	e.inserts <- &insertRequest{
		cids:        cids,
		isRoot:      isRoot,
		chunkIndex:  chunkIndex,
		height:      height,
		blockID:     blockID,
		blockHeight: blockHeight,
	}
}

func (e *ExecutionDataCIDComparatorImpl) GetCIDCacher(blockID flow.Identifier, blockHeight uint64) CIDCacher {
	return &cidCacher{
		blockID:     blockID,
		blockHeight: blockHeight,
		cache:       e,
	}
}

type cidCacher struct {
	blockID     flow.Identifier
	blockHeight uint64
	cache       *ExecutionDataCIDComparatorImpl
}

func (c *cidCacher) InsertRootCID(rootCID cid.Cid) {
	c.cache.insert([]cid.Cid{rootCID}, true, 0, 0, c.blockID, c.blockHeight)
}

func (c *cidCacher) InsertBlobTreeLevel(chunkIndex, height int, cids []cid.Cid) {
	c.cache.insert(cids, false, uint(chunkIndex), uint(height), c.blockID, c.blockHeight)
}
