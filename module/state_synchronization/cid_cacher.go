package state_synchronization

import (
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/flow"
)

type CIDCacher interface {
	InsertRootCID(rootCID cid.Cid)
	InsertBlobTreeLevel(chunkIndex, height int, cids []cid.Cid)
}

type CIDCacherFactory interface {
	GetCIDCacher(blockID flow.Identifier, blockHeight uint64) CIDCacher
}
