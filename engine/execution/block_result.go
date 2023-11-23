package execution

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
)

// BlockExecutionResult captures artifacts of execution of block collections
type BlockExecutionResult struct {
	*entity.ExecutableBlock

	collectionExecutionResults []CollectionExecutionResult
	ExecutionDataRoot          *flow.BlockExecutionDataRoot // full root data structure produced from block
}

// NewPopulatedBlockExecutionResult constructs a new BlockExecutionResult,
// pre-populated with `chunkCounts` number of collection results
func NewPopulatedBlockExecutionResult(eb *entity.ExecutableBlock) *BlockExecutionResult {
	chunkCounts := len(eb.CompleteCollections) + 1
	return &BlockExecutionResult{
		ExecutableBlock:            eb,
		collectionExecutionResults: make([]CollectionExecutionResult, chunkCounts),
	}
}

// Size returns the size of collection execution results
func (er *BlockExecutionResult) Size() int {
	return len(er.collectionExecutionResults)
}

func (er *BlockExecutionResult) CollectionExecutionResultAt(colIndex int) *CollectionExecutionResult {
	if colIndex < 0 && colIndex > len(er.collectionExecutionResults) {
		return nil
	}
	return &er.collectionExecutionResults[colIndex]
}

func (er *BlockExecutionResult) AllEvents() flow.EventsList {
	res := make(flow.EventsList, 0)
	for _, ce := range er.collectionExecutionResults {
		if len(ce.events) > 0 {
			res = append(res, ce.events...)
		}
	}
	return res
}

func (er *BlockExecutionResult) AllServiceEvents() flow.EventsList {
	res := make(flow.EventsList, 0)
	for _, ce := range er.collectionExecutionResults {
		if len(ce.serviceEvents) > 0 {
			res = append(res, ce.serviceEvents...)
		}
	}
	return res
}

func (er *BlockExecutionResult) TransactionResultAt(txIdx int) *flow.TransactionResult {
	allTxResults := er.AllTransactionResults() // TODO: optimize me
	if txIdx > len(allTxResults) {
		return nil
	}
	return &allTxResults[txIdx]
}

func (er *BlockExecutionResult) AllTransactionResults() flow.TransactionResults {
	res := make(flow.TransactionResults, 0)
	for _, ce := range er.collectionExecutionResults {
		if len(ce.transactionResults) > 0 {
			res = append(res, ce.transactionResults...)
		}
	}
	return res
}

func (er *BlockExecutionResult) AllExecutionSnapshots() []*snapshot.ExecutionSnapshot {
	res := make([]*snapshot.ExecutionSnapshot, 0)
	for _, ce := range er.collectionExecutionResults {
		es := ce.ExecutionSnapshot()
		res = append(res, es)
	}
	return res
}

func (er *BlockExecutionResult) AllConvertedServiceEvents() flow.ServiceEventList {
	res := make(flow.ServiceEventList, 0)
	for _, ce := range er.collectionExecutionResults {
		if len(ce.convertedServiceEvents) > 0 {
			res = append(res, ce.convertedServiceEvents...)
		}
	}
	return res
}

// AllUpdatedRegisters returns all updated unique register entries
// Note: order is not determinstic
func (er *BlockExecutionResult) AllUpdatedRegisters() []flow.RegisterEntry {
	updates := make(map[flow.RegisterID]flow.RegisterValue)
	for _, ce := range er.collectionExecutionResults {
		for regID, regVal := range ce.executionSnapshot.WriteSet {
			updates[regID] = regVal
		}
	}
	res := make([]flow.RegisterEntry, 0)
	for regID, regVal := range updates {
		res = append(res, flow.RegisterEntry{
			Key:   regID,
			Value: regVal,
		})
	}
	return res
}

// BlockAttestationResult holds collection attestation results
type BlockAttestationResult struct {
	*BlockExecutionResult

	collectionAttestationResults []CollectionAttestationResult

	// TODO(ramtin): move this to the outside, everything needed for create this
	// should be available as part of computation result and most likely trieUpdate
	// was the reason this is kept here, long term we don't need this data and should
	// act based on register deltas
	*execution_data.BlockExecutionData
}

func NewEmptyBlockAttestationResult(
	blockExecutionResult *BlockExecutionResult,
) *BlockAttestationResult {
	colSize := blockExecutionResult.Size()
	return &BlockAttestationResult{
		BlockExecutionResult:         blockExecutionResult,
		collectionAttestationResults: make([]CollectionAttestationResult, 0, colSize),
		BlockExecutionData: &execution_data.BlockExecutionData{
			BlockID: blockExecutionResult.ID(),
			ChunkExecutionDatas: make(
				[]*execution_data.ChunkExecutionData,
				0,
				colSize),
		},
	}
}

// CollectionAttestationResultAt returns CollectionAttestationResult at collection index
func (ar *BlockAttestationResult) CollectionAttestationResultAt(colIndex int) *CollectionAttestationResult {
	if colIndex < 0 && colIndex > len(ar.collectionAttestationResults) {
		return nil
	}
	return &ar.collectionAttestationResults[colIndex]
}

func (ar *BlockAttestationResult) AppendCollectionAttestationResult(
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

func (ar *BlockAttestationResult) AllChunks() []*flow.Chunk {
	chunks := make([]*flow.Chunk, len(ar.collectionAttestationResults))
	for i := 0; i < len(ar.collectionAttestationResults); i++ {
		chunks[i] = ar.ChunkAt(i) // TODO(ramtin): cache and optimize this
	}
	return chunks
}

func (ar *BlockAttestationResult) ChunkAt(index int) *flow.Chunk {
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
		execRes.executionSnapshot.TotalComputationUsed(),
	)
}

func (ar *BlockAttestationResult) AllChunkDataPacks() []*flow.ChunkDataPack {
	chunkDataPacks := make([]*flow.ChunkDataPack, len(ar.collectionAttestationResults))
	for i := 0; i < len(ar.collectionAttestationResults); i++ {
		chunkDataPacks[i] = ar.ChunkDataPackAt(i) // TODO(ramtin): cache and optimize this
	}
	return chunkDataPacks
}

func (ar *BlockAttestationResult) ChunkDataPackAt(index int) *flow.ChunkDataPack {
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
		*ar.ExecutionDataRoot,
	)
}

func (ar *BlockAttestationResult) AllEventCommitments() []flow.Identifier {
	res := make([]flow.Identifier, 0)
	for _, ca := range ar.collectionAttestationResults {
		res = append(res, ca.EventCommitment())
	}
	return res
}

// Size returns the size of collection attestation results
func (ar *BlockAttestationResult) Size() int {
	return len(ar.collectionAttestationResults)
}
