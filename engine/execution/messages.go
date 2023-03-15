package execution

import (
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// TODO(patrick): rm unaccessed fields
type ComputationResult struct {
	*entity.ExecutableBlock
	StateSnapshots         []*state.ExecutionSnapshot
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
		StateSnapshots:         make([]*state.ExecutionSnapshot, 0, numCollections),
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
	return cr.TouchedRegistersFunc()
}

func (cr *ColResSnapshot) EmittedEvents() flow.EventsList {
	return cr.EmittedEventsFunc()
}

func (cr *ColResSnapshot) TransactionResults() flow.TransactionResults {
	return cr.TransactionResultsFunc()
}
