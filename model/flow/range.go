// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

// Range is a height range for which we want to request blocks.
type Range struct {
	From uint64
	To   uint64
}

// Batch is a set of block IDs we want to request.
type Batch struct {
	BlockIDs []Identifier
}
