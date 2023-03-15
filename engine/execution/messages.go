package execution

import (
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// TODO If the executor will be a separate process/machine we would need to rework
// sending view as local data, but that would be much greater refactor of storage anyway

// TODO(patrick): rm unaccessed fields
type ComputationResult struct {
	*entity.ExecutableBlock
	StateSnapshots         []state.ExecutionSnapshot
	StateCommitments       []flow.StateCommitment
	Events                 []flow.EventsList
	EventsHashes           []flow.Identifier
	ServiceEvents          flow.EventsList
	TransactionResults     []flow.TransactionResult
	TransactionResultIndex []int

	// TODO(patrick): switch this to execution snapshot
	ComputationIntensities meter.MeteredComputationIntensities

	ChunkDataPacks []*flow.ChunkDataPack
	EndState       flow.StateCommitment

	*execution_data.BlockExecutionData
	*flow.ExecutionReceipt
}

func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
) *ComputationResult {
	numCollections := len(block.CompleteCollections) + 1
	return &ComputationResult{
		ExecutableBlock:        block,
		StateSnapshots:         make([]state.ExecutionSnapshot, 0, numCollections),
		StateCommitments:       make([]flow.StateCommitment, 0, numCollections),
		Events:                 make([]flow.EventsList, numCollections),
		EventsHashes:           make([]flow.Identifier, 0, numCollections),
		ServiceEvents:          make(flow.EventsList, 0),
		TransactionResults:     make([]flow.TransactionResult, 0),
		TransactionResultIndex: make([]int, 0),
		ComputationIntensities: make(meter.MeteredComputationIntensities),
		ChunkDataPacks:         make([]*flow.ChunkDataPack, 0, numCollections),
		EndState:               *block.StartState,
		BlockExecutionData: &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: make(
				[]*execution_data.ChunkExecutionData,
				0,
				numCollections),
		},
	}
}

func (cr *ComputationResult) CollectionStats(
	collectionIndex int,
) module.ExecutionResultStats {
	var startTxnIndex int
	if collectionIndex > 0 {
		startTxnIndex = cr.TransactionResultIndex[collectionIndex-1]
	}
	endTxnIndex := cr.TransactionResultIndex[collectionIndex]

	var computationUsed uint64
	var memoryUsed uint64
	for _, txn := range cr.TransactionResults[startTxnIndex:endTxnIndex] {
		computationUsed += txn.ComputationUsed
		memoryUsed += txn.MemoryUsed
	}

	events := cr.Events[collectionIndex]
	snapshot := cr.StateSnapshots[collectionIndex]

	numTouched := len(snapshot.AllRegisterIDs())
	bytesWritten := 0
	for _, entry := range snapshot.UpdatedRegisters() {
		bytesWritten += len(entry.Value)
	}

	return module.ExecutionResultStats{
		ComputationUsed:                 computationUsed,
		MemoryUsed:                      memoryUsed,
		EventCounts:                     len(events),
		EventSize:                       events.ByteSize(),
		NumberOfRegistersTouched:        numTouched,
		NumberOfBytesWrittenToRegisters: bytesWritten,
		NumberOfCollections:             1,
		NumberOfTransactions:            endTxnIndex - startTxnIndex,
	}
}

func (cr *ComputationResult) BlockStats() module.ExecutionResultStats {
	stats := module.ExecutionResultStats{}
	for idx := 0; idx < len(cr.StateCommitments); idx++ {
		stats.Merge(cr.CollectionStats(idx))
	}

	return stats
}

func (cr *ComputationResult) CollectionResult(colIndex int) *ColResSnapshot {
	if colIndex < 0 && colIndex > len(cr.CompleteCollections) {
		return nil
	}
	return &ColResSnapshot{
		BlockHeaderFunc: func() *flow.Header {
			return cr.Block.Header
		},
		CollectionFunc: func() *flow.Collection {
			return &flow.Collection{
				Transactions: cr.CollectionAt(colIndex).Transactions,
			}
		},
		UpdatedRegistersFunc: func() flow.RegisterEntries {
			return cr.StateSnapshots[colIndex].UpdatedRegisters()
		},
		TouchedRegistersFunc: func() flow.RegisterIDs {
			return cr.StateSnapshots[colIndex].AllRegisterIDs()
		},
		EmittedEventsFunc: func() flow.EventsList {
			return cr.Events[colIndex]
		},
		TransactionResultsFunc: func() flow.TransactionResults {
			var startTxnIndex int
			if colIndex > 0 {
				startTxnIndex = cr.TransactionResultIndex[colIndex-1]
			}
			endTxnIndex := cr.TransactionResultIndex[colIndex]
			return cr.TransactionResults[startTxnIndex:endTxnIndex]
		},
	}

}

type ColResSnapshot struct {
	BlockHeaderFunc        func() *flow.Header
	CollectionFunc         func() *flow.Collection
	UpdatedRegistersFunc   func() flow.RegisterEntries
	TouchedRegistersFunc   func() flow.RegisterIDs
	EmittedEventsFunc      func() flow.EventsList
	TransactionResultsFunc func() flow.TransactionResults
}

func (cr *ColResSnapshot) BlockHeader() *flow.Header {
	return cr.BlockHeaderFunc()
}

func (cr *ColResSnapshot) Collection() *flow.Collection {
	return cr.CollectionFunc()
}

func (cr *ColResSnapshot) UpdatedRegisters() flow.RegisterEntries {
	return cr.UpdatedRegistersFunc()
}

func (cr *ColResSnapshot) TouchedRegisters() flow.RegisterIDs {
	return cr.TouchedRegisters()
}

func (cr *ColResSnapshot) EmittedEvents() flow.EventsList {
	return cr.EmittedEvents()
}

func (cr *ColResSnapshot) TransactionResults() flow.TransactionResults {
	return cr.TransactionResults()
}
