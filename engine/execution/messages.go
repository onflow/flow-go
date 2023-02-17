package execution

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/meter"
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
	StateSnapshots         []*delta.SpockSnapshot
	StateCommitments       []flow.StateCommitment
	Proofs                 [][]byte
	Events                 []flow.EventsList
	EventsHashes           []flow.Identifier
	ServiceEvents          flow.EventsList
	ConvertedServiceEvents flow.ServiceEventList
	TransactionResults     []flow.TransactionResult
	TransactionResultIndex []int
	ComputationIntensities meter.MeteredComputationIntensities
	ExecutionDataID        flow.Identifier
	SpockSignatures        []crypto.Signature
	Chunks                 []*flow.Chunk
	ChunkDataPacks         []*flow.ChunkDataPack
	EndState               flow.StateCommitment

	*execution_data.BlockExecutionData
}

func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
) *ComputationResult {
	numCollections := len(block.CompleteCollections) + 1
	return &ComputationResult{
		ExecutableBlock:        block,
		StateSnapshots:         make([]*delta.SpockSnapshot, 0, numCollections),
		StateCommitments:       make([]flow.StateCommitment, 0, numCollections),
		Proofs:                 make([][]byte, 0, numCollections),
		Events:                 make([]flow.EventsList, numCollections),
		EventsHashes:           make([]flow.Identifier, 0, numCollections),
		ServiceEvents:          make(flow.EventsList, 0),
		ConvertedServiceEvents: make(flow.ServiceEventList, 0),
		TransactionResults:     make([]flow.TransactionResult, 0),
		TransactionResultIndex: make([]int, 0),
		ComputationIntensities: make(meter.MeteredComputationIntensities),
		SpockSignatures:        make([]crypto.Signature, 0, numCollections),
		Chunks:                 make([]*flow.Chunk, 0, numCollections),
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

func (cr *ComputationResult) AddTransactionResult(
	collectionIndex int,
	txn *fvm.TransactionProcedure,
) {
	cr.Events[collectionIndex] = append(
		cr.Events[collectionIndex],
		txn.Events...)
	cr.ServiceEvents = append(cr.ServiceEvents, txn.ServiceEvents...)
	cr.ConvertedServiceEvents = append(
		cr.ConvertedServiceEvents,
		txn.ConvertedServiceEvents...)

	txnResult := flow.TransactionResult{
		TransactionID:   txn.ID,
		ComputationUsed: txn.ComputationUsed,
		MemoryUsed:      txn.MemoryEstimate,
	}
	if txn.Err != nil {
		txnResult.ErrorMessage = txn.Err.Error()
	}

	cr.TransactionResults = append(cr.TransactionResults, txnResult)

	for computationKind, intensity := range txn.ComputationIntensities {
		cr.ComputationIntensities[computationKind] += intensity
	}
}

func (cr *ComputationResult) AddCollection(snapshot *delta.SpockSnapshot) {
	cr.TransactionResultIndex = append(
		cr.TransactionResultIndex,
		len(cr.TransactionResults))
	cr.StateSnapshots = append(cr.StateSnapshots, snapshot)
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
	return module.ExecutionResultStats{
		ComputationUsed:                 computationUsed,
		MemoryUsed:                      memoryUsed,
		EventCounts:                     len(events),
		EventSize:                       events.ByteSize(),
		NumberOfRegistersTouched:        snapshot.NumberOfRegistersTouched,
		NumberOfBytesWrittenToRegisters: snapshot.NumberOfBytesWrittenToRegisters,
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
