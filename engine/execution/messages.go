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
