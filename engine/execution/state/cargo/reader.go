package cargo

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// Reader holds on to the height and blockID
//
// the reason we could not have this on view level
// is view might gets merged or historic values get
// prunned from the oracles, so a reader might
// return error at any time in the future.
type Reader struct {
	height  uint64
	blockID flow.Identifier
	views   *Views
}

func NewReader(block *flow.Header, views *Views) *Reader {
	return &Reader{height: block.Height,
		blockID: block.ID(),
		views:   views,
	}
}

func (r *Reader) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	return r.views.Get(r.height, r.blockID, id)
}

var _ state.StorageSnapshot = &Reader{}
