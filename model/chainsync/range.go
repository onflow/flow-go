package chainsync

import "github.com/onflow/flow-go/model/flow"

// Range is a height range for which we want to request blocks. inclusive [from, to]
type Range struct {
	From uint64
	To   uint64
}

func (r *Range) Len() int {
	return int(r.To - r.From + 1)
}

// Batch is a set of block IDs we want to request.
type Batch struct {
	BlockIDs []flow.Identifier
}
