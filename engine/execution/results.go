package execution

import (
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// BlockExecutionResults captures artifacts of execution of block collections
type BlockExecutionResults struct {
	*entity.ExecutableBlock

	collectionExecutionResults []CollectionExecutionResult

	// TODO(patrick): switch this to execution snapshot
	ComputationIntensities meter.MeteredComputationIntensities
}

// NewPopulatedBlockExecutionResults constructs a new BlockExecutionResults,
// pre-populated with `chunkCounts` number of collection results
func NewPopulatedBlockExecutionResults(eb *entity.ExecutableBlock) *BlockExecutionResults {
	chunkCounts := len(eb.CompleteCollections) + 1
	return &BlockExecutionResults{
		ExecutableBlock:            eb,
		collectionExecutionResults: make([]CollectionExecutionResult, chunkCounts),
		ComputationIntensities:     make(meter.MeteredComputationIntensities),
	}
}

// Size returns the size of collection execution results
func (er *BlockExecutionResults) Size() int {
	return len(er.collectionExecutionResults)
}

func (er *BlockExecutionResults) CollectionExecutionResultAt(colIndex int) *CollectionExecutionResult {
	if colIndex < 0 && colIndex > len(er.collectionExecutionResults) {
		return nil
	}
	return &er.collectionExecutionResults[colIndex]
}

func (er *BlockExecutionResults) AllEvents() flow.EventsList {
	res := make(flow.EventsList, 0)
	for _, ce := range er.collectionExecutionResults {
		if len(ce.events) > 0 {
			res = append(res, ce.events...)
		}
	}
	return res
}

func (er *BlockExecutionResults) AllServiceEvents() flow.EventsList {
	res := make(flow.EventsList, 0)
	for _, ce := range er.collectionExecutionResults {
		if len(ce.serviceEvents) > 0 {
			res = append(res, ce.serviceEvents...)
		}
	}
	return res
}

func (er *BlockExecutionResults) TransactionResultAt(txIdx int) *flow.TransactionResult {
	allTxResults := er.AllTransactionResults() // TODO: optimize me
	if txIdx > len(allTxResults) {
		return nil
	}
	return &allTxResults[txIdx]
}

func (er *BlockExecutionResults) AllTransactionResults() flow.TransactionResults {
	res := make(flow.TransactionResults, 0)
	for _, ce := range er.collectionExecutionResults {
		if len(ce.transactionResults) > 0 {
			res = append(res, ce.transactionResults...)
		}
	}
	return res
}

func (er *BlockExecutionResults) AllExecutionSnapshots() []*state.ExecutionSnapshot {
	res := make([]*state.ExecutionSnapshot, 0)
	for _, ce := range er.collectionExecutionResults {
		es := ce.ExecutionSnapshot()
		res = append(res, es)
	}
	return res
}

func (er *BlockExecutionResults) AllConvertedServiceEvents() flow.ServiceEventList {
	res := make(flow.ServiceEventList, 0)
	for _, ce := range er.collectionExecutionResults {
		if len(ce.convertedServiceEvents) > 0 {
			res = append(res, ce.convertedServiceEvents...)
		}
	}
	return res
}

// BlockAttestationResults holds collection attestation results
type BlockAttestationResults struct {
	*BlockExecutionResults

	collectionAttestationResults []CollectionAttestationResult

	// TODO(ramtin): move this to the outside, everything needed for create this
	// should be available as part of computation result and most likely trieUpdate
	// was the reason this is kept here, long term we don't need this data and should
	// act based on register deltas
	*execution_data.BlockExecutionData
}

func NewEmptyBlockAttestationResults(
	blockExecutionResults *BlockExecutionResults,
) *BlockAttestationResults {
	colSize := blockExecutionResults.Size()
	return &BlockAttestationResults{
		BlockExecutionResults:        blockExecutionResults,
		collectionAttestationResults: make([]CollectionAttestationResult, 0, colSize),
		BlockExecutionData: &execution_data.BlockExecutionData{
			BlockID: blockExecutionResults.ID(),
			ChunkExecutionDatas: make(
				[]*execution_data.ChunkExecutionData,
				0,
				colSize),
		},
	}
}

// CollectionAttestationResultAt returns CollectionAttestationResult at collection index
func (ar *BlockAttestationResults) CollectionAttestationResultAt(colIndex int) *CollectionAttestationResult {
	if colIndex < 0 && colIndex > len(ar.collectionAttestationResults) {
		return nil
	}
	return &ar.collectionAttestationResults[colIndex]
}

func (ar *BlockAttestationResults) AppendCollectionAttestationResult(
	startStateCommit flow.StateCommitment,
	endStateCommit flow.StateCommitment,
	stateProof flow.StorageProof,
	eventCommit flow.Identifier,
	chunkExecutionDatas *execution_data.ChunkExecutionData,
) {
	ar.collectionAttestationResults = append(ar.collectionAttestationResults,
		CollectionAttestationResult{
			startStateCommit: startStateCommit,
			endStateCommit:   endStateCommit,
			stateProof:       stateProof,
			eventCommit:      eventCommit,
		},
	)
	ar.ChunkExecutionDatas = append(ar.ChunkExecutionDatas, chunkExecutionDatas)
}

func (ar *BlockAttestationResults) AllChunks() []*flow.Chunk {
	chunks := make([]*flow.Chunk, len(ar.collectionAttestationResults))
	for i := 0; i < len(ar.collectionAttestationResults); i++ {
		chunks[i] = ar.ChunkAt(i) // TODO(ramtin): cache and optimize this
	}
	return chunks
}

func (ar *BlockAttestationResults) ChunkAt(index int) *flow.Chunk {
	if index < 0 || index >= len(ar.collectionAttestationResults) {
		return nil
	}

	execRes := ar.collectionExecutionResults[index]
	attestRes := ar.collectionAttestationResults[index]

	return flow.NewChunk(
		ar.Block.ID(),
		index,
		attestRes.startStateCommit,
		len(execRes.TransactionResults()),
		attestRes.eventCommit,
		attestRes.endStateCommit,
	)
}

func (ar *BlockAttestationResults) AllChunkDataPacks() []*flow.ChunkDataPack {
	chunkDataPacks := make([]*flow.ChunkDataPack, len(ar.collectionAttestationResults))
	for i := 0; i < len(ar.collectionAttestationResults); i++ {
		chunkDataPacks[i] = ar.ChunkDataPackAt(i) // TODO(ramtin): cache and optimize this
	}
	return chunkDataPacks
}

func (ar *BlockAttestationResults) ChunkDataPackAt(index int) *flow.ChunkDataPack {
	if index < 0 || index >= len(ar.collectionAttestationResults) {
		return nil
	}

	// Note: There's some inconsistency in how chunk execution data and
	// chunk data pack populate their collection fields when the collection
	// is the system collection.
	// collectionAt would return nil if the collection is system collection
	collection := ar.CollectionAt(index)

	attestRes := ar.collectionAttestationResults[index]

	return flow.NewChunkDataPack(
		ar.ChunkAt(index).ID(), // TODO(ramtin): optimize this
		attestRes.startStateCommit,
		attestRes.stateProof,
		collection,
	)
}

func (ar *BlockAttestationResults) AllEventCommitments() []flow.Identifier {
	res := make([]flow.Identifier, 0)
	for _, ca := range ar.collectionAttestationResults {
		res = append(res, ca.EventCommitment())
	}
	return res
}

// Size returns the size of collection attestation results
func (ar *BlockAttestationResults) Size() int {
	return len(ar.collectionAttestationResults)
}
