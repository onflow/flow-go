package execution

import (
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/ledger"
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
	ExecutableBlock    *entity.ExecutableBlock
	StateSnapshots     []*delta.SpockSnapshot
	StateCommitments   []flow.StateCommitment
	Proofs             [][]byte
	Events             []flow.EventsList
	EventsHashes       []flow.Identifier
	ServiceEvents      flow.EventsList
	TransactionResults []flow.TransactionResult
	ComputationUsed    uint64
	StateReads         uint64
	TrieUpdates        []*ledger.TrieUpdate
	ExecutionDataID    flow.Identifier
}

func (cr *ComputationResult) AddEvents(chunkIndex int, inp []flow.Event) {
	cr.Events[chunkIndex] = append(cr.Events[chunkIndex], inp...)
}

func (cr *ComputationResult) AddServiceEvents(inp []flow.Event) {
	cr.ServiceEvents = append(cr.ServiceEvents, inp...)
}

func (cr *ComputationResult) AddTransactionResult(inp *flow.TransactionResult) {
	cr.TransactionResults = append(cr.TransactionResults, *inp)
}

func (cr *ComputationResult) AddIndexedTransactionResult(inp *flow.TransactionResult, index int) {
	cr.TransactionResults[index] = *inp
}

func (cr *ComputationResult) AddComputationUsed(inp uint64) {
	cr.ComputationUsed += inp
}

func (cr *ComputationResult) AddStateSnapshot(inp *delta.SpockSnapshot) {
	cr.StateSnapshots = append(cr.StateSnapshots, inp)
}
