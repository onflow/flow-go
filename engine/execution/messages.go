package execution

import (
	"sync"

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
	GasUsed            uint64
	StateReads         uint64
	lock               sync.Mutex
}

func (cr *ComputationResult) AddEvents(inp []flow.Event) {
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cr.Events = append(cr.Events, inp...)
}

func (cr *ComputationResult) AddServiceEvents(inp []flow.Event) {
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cr.ServiceEvents = append(cr.ServiceEvents, inp...)
}

func (cr *ComputationResult) AddTransactionResult(inp *flow.TransactionResult) {
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cr.TransactionResults = append(cr.TransactionResults, *inp)
}

func (cr *ComputationResult) AddGasUsed(inp uint64) {
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cr.GasUsed += inp
}

func (cr *ComputationResult) AddStateSnapshot(inp *delta.SpockSnapshot) {
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cr.StateSnapshots = append(cr.StateSnapshots, inp)
}

// TODO change this when state commitment is an array
func (cr *ComputationResult) AddStateCommitment(inp flow.StateCommitment) {
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cr.StateCommitments = append(cr.StateCommitments, inp)
}

func (cr *ComputationResult) AddProof(inp []byte) {
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cr.Proofs = append(cr.Proofs, inp)
}
