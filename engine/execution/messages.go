package execution

import (
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// TODO If the executor will be a separate process/machine we would need to rework
// sending view as local data, but that would be much greater refactor of storage anyway

type ComputationOrder struct {
	Block      *entity.ExecutableBlock
	View       *delta.View
	StartState flow.StateCommitment
}

type ComputationResult struct {
	ExecutableBlock   *entity.ExecutableBlock
	StateSnapshots    []*delta.SpockSnapshot
	Events            []flow.Event
	ServiceEvents     []flow.Event
	TransactionResult []flow.TransactionResult
	GasUsed           uint64
	StateReads        uint64

	CollectionConflicts      [][]flow.RegisterID
	BlockConflicts           [][]flow.RegisterID
	TotalTx                  int
	ConflictingBlockTxs      int
	ConflictingCollectionTxs int
}
