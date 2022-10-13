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
	ExecutableBlock        *entity.ExecutableBlock
	StateSnapshots         []*delta.SpockSnapshot
	StateCommitments       []flow.StateCommitment
	Proofs                 [][]byte
	Events                 []flow.EventsList
	EventsHashes           []flow.Identifier
	ServiceEvents          flow.EventsList
	TransactionResults     []flow.TransactionResult
	TransactionResultIndex []int
	TrieUpdates            []*ledger.TrieUpdate
	ExecutionDataID        flow.Identifier
}

func NewEmptyComputationResult(block *entity.ExecutableBlock) *ComputationResult {
	numberOfChunks := len(block.CompleteCollections) + 1
	return &ComputationResult{
		ExecutableBlock:        block,
		Events:                 make([]flow.EventsList, numberOfChunks),
		ServiceEvents:          make(flow.EventsList, 0),
		TransactionResults:     make([]flow.TransactionResult, 0),
		TransactionResultIndex: make([]int, 0),
		StateCommitments:       make([]flow.StateCommitment, 0, numberOfChunks),
		Proofs:                 make([][]byte, 0, numberOfChunks),
		TrieUpdates:            make([]*ledger.TrieUpdate, 0, numberOfChunks),
		EventsHashes:           make([]flow.Identifier, 0, numberOfChunks),
	}
}

func (cr *ComputationResult) AddEvents(chunkIndex int, inp []flow.Event) {
	cr.Events[chunkIndex] = append(cr.Events[chunkIndex], inp...)
}

func (cr *ComputationResult) AddServiceEvents(inp []flow.Event) {
	cr.ServiceEvents = append(cr.ServiceEvents, inp...)
}

// Update this
func (cr *ComputationResult) AddTransactionResult(inp *flow.TransactionResult) {
	cr.TransactionResults = append(cr.TransactionResults, *inp)
}

func (cr *ComputationResult) UpdateTransactionResultIndex(txCounts int) {
	lastIndex := 0
	if len(cr.TransactionResultIndex) > 0 {
		lastIndex = cr.TransactionResultIndex[len(cr.TransactionResultIndex)-1]
	}
	cr.TransactionResultIndex = append(cr.TransactionResultIndex, lastIndex+txCounts)
}

func (cr *ComputationResult) AddStateSnapshot(inp *delta.SpockSnapshot) {
	cr.StateSnapshots = append(cr.StateSnapshots, inp)
}

func (cr *ComputationResult) ChunkEventCountsAndSize(chunkIndex int) (int, int) {
	return len(cr.Events[chunkIndex]), cr.Events[chunkIndex].ByteSize()
}

func (cr *ComputationResult) BlockEventCountsAndSize() (int, int) {
	totalSize := cr.ServiceEvents.ByteSize()
	totalCounts := len(cr.ServiceEvents)
	for _, events := range cr.Events {
		totalSize += events.ByteSize()
		totalCounts += len(events)
	}
	return totalCounts, totalSize
}

func (cr *ComputationResult) ChunkComputationAndMemoryUsed(chunkIndex int) (uint64, uint64) {
	var startTxIndex int
	if chunkIndex > 0 {
		startTxIndex = cr.TransactionResultIndex[chunkIndex-1]
	}
	endTxIndex := cr.TransactionResultIndex[chunkIndex]

	var totalComputationUsed uint64
	var totalMemoryUsed uint64
	for i := startTxIndex; i < endTxIndex; i++ {
		totalComputationUsed += cr.TransactionResults[i].ComputationUsed
		totalMemoryUsed += cr.TransactionResults[i].MemoryUsed
	}
	return totalComputationUsed, totalMemoryUsed
}

func (cr *ComputationResult) BlockComputationAndMemoryUsed() (uint64, uint64) {
	var totalComputationUsed uint64
	var totalMemoryUsed uint64
	for i := 0; i < len(cr.TransactionResults); i++ {
		totalComputationUsed += cr.TransactionResults[i].ComputationUsed
		totalMemoryUsed += cr.TransactionResults[i].MemoryUsed
	}
	return totalComputationUsed, totalMemoryUsed
}
