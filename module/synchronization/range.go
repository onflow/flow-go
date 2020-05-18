// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Range is a range for which we want to request blocks.
type Range struct {
	From uint64
	To   uint64
}

type Batch struct {
	BlockIDs []flow.Identifier
}
