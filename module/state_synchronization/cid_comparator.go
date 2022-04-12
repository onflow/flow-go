package state_synchronization

import (
	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/model/flow"
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
