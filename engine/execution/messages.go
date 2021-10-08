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
	ExecutableBlock    *entity.ExecutableBlock
	StateSnapshots     []*delta.SpockSnapshot
	StateCommitments   []flow.StateCommitment
	Proofs             [][]byte
	Events             []flow.Event
	ServiceEvents      []flow.Event
	TransactionResults []flow.TransactionResult
	ComputationUsed    uint64
	StateReads         uint64
}

func (cr *ComputationResult) AddEvents(inp []flow.Event) {
	cr.Events = append(cr.Events, inp...)
}

func (cr *ComputationResult) AddServiceEvents(inp []flow.Event) {
	cr.ServiceEvents = append(cr.ServiceEvents, inp...)
}

func (cr *ComputationResult) AddTransactionResult(inp *flow.TransactionResult) {
	cr.TransactionResults = append(cr.TransactionResults, *inp)
}

func (cr *ComputationResult) AddComputationUsed(inp uint64) {
	cr.ComputationUsed += inp
}

func (cr *ComputationResult) AddStateSnapshot(inp *delta.SpockSnapshot) {
	cr.StateSnapshots = append(cr.StateSnapshots, inp)
}
