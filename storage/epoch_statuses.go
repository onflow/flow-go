// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type EpochStatuses[tx Transaction] interface {

	// StoreTx stores a new epoch state in a DB transaction while going through the cache.
	StoreTx(blockID flow.Identifier, state *flow.EpochStatus) func(TransactionContext[tx]) error

	// ByBlockID will return the epoch status for the given block
	// Error returns:
	// * storage.ErrNotFound if EpochStatus for the block does not exist
	ByBlockID(flow.Identifier) (*flow.EpochStatus, error)
}
