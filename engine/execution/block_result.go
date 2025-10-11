package execution

import (
	"fmt"
	"math"

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
	if colIndex < 0 || colIndex > len(er.collectionExecutionResults) {
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

// ServiceEventCountForChunk returns the number of service events emitted in the given chunk.
func (er *BlockExecutionResult) ServiceEventCountForChunk(chunkIndex int) uint16 {
	serviceEventCount := len(er.collectionExecutionResults[chunkIndex].serviceEvents)
	if serviceEventCount > math.MaxUint16 {
		// The current protocol demands that the ServiceEventCount does not exceed 65535.
		// For defensive programming, we explicitly enforce this limit as 65k could be produced by a bug.
		// Execution nodes would be first to realize that this bound is violated, and crash (fail early).
		panic(fmt.Sprintf("service event count (%d) exceeds maximum value of 65535", serviceEventCount))
	}
	return uint16(serviceEventCount)
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
	res := make([]flow.RegisterEntry, 0, len(updates))
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
			BlockID: blockExecutionResult.BlockID(),
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

func (ar *BlockAttestationResult) AllChunks() ([]*flow.Chunk, error) {
	chunks := make([]*flow.Chunk, len(ar.collectionAttestationResults))
	for i := 0; i < len(ar.collectionAttestationResults); i++ {
		chunk, err := ar.ChunkAt(i)
		if err != nil {
			return nil, fmt.Errorf("could not find chunk: %w", err)
		}
		chunks[i] = chunk // TODO(ramtin): cache and optimize this
	}
	return chunks, nil
}

// ChunkAt returns the Chunk for the collection at the given index.
// Receiver BlockAttestationResult is expected to be well-formed; callers must use an index that exists.
// No errors are expected during normal operation.
func (ar *BlockAttestationResult) ChunkAt(index int) (*flow.Chunk, error) {
	if index < 0 || index >= len(ar.collectionAttestationResults) {
		return nil, fmt.Errorf("chunk collection index is not valid: %v", index)
	}

	execRes := ar.collectionExecutionResults[index]
	attestRes := ar.collectionAttestationResults[index]

	if execRes.executionSnapshot == nil {
		// This should never happen
		// In case it does, attach additional information to the error message
		panic(fmt.Sprintf("execution snapshot is nil. Block ID: %s, EndState: %s", ar.Block.ID(), attestRes.endStateCommit))
	}

	chunk, err := flow.NewChunk(flow.UntrustedChunk{
		ChunkBody: flow.ChunkBody{
			BlockID:              ar.Block.ID(),
			CollectionIndex:      uint(index),
			StartState:           attestRes.startStateCommit,
			EventCollection:      attestRes.eventCommit,
			ServiceEventCount:    ar.ServiceEventCountForChunk(index),
			TotalComputationUsed: execRes.executionSnapshot.TotalComputationUsed(),
			NumberOfTransactions: uint64(len(execRes.TransactionResults())),
		},
		Index:    uint64(index),
		EndState: attestRes.endStateCommit,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build chunk: %w", err)
	}

	return chunk, nil

}

func (ar *BlockAttestationResult) AllChunkDataPacks() ([]*flow.ChunkDataPack, error) {
	chunkDataPacks := make([]*flow.ChunkDataPack, len(ar.collectionAttestationResults))
	for i := 0; i < len(ar.collectionAttestationResults); i++ {
		chunkDataPack, err := ar.ChunkDataPackAt(i)
		if err != nil {
			return nil, fmt.Errorf("could not find chunk data pack: %w", err)
		}
		chunkDataPacks[i] = chunkDataPack // TODO(ramtin): cache and optimize this
	}
	return chunkDataPacks, nil
}

// ChunkDataPackAt returns the ChunkDataPack for the collection at the given index.
// Receiver BlockAttestationResult is expected to be well-formed; callers must use an index that exists.
// No errors are expected during normal operation.
func (ar *BlockAttestationResult) ChunkDataPackAt(index int) (*flow.ChunkDataPack, error) {
	if index < 0 || index >= len(ar.collectionAttestationResults) {
		return nil, fmt.Errorf("chunk collection index is not valid: %v", index)
	}

	// Note: There's some inconsistency in how chunk execution data and
	// chunk data pack populate their collection fields when the collection
	// is the system collection.
	// collectionAt would return nil if the collection is system collection
	collection := ar.CollectionAt(index)

	attestRes := ar.collectionAttestationResults[index]

	chunk, err := ar.ChunkAt(index)
	if err != nil {
		return nil, fmt.Errorf("could not build chunk: %w", err)
	}

	chunkDataPack, err := flow.FromUntrustedChunkDataPack(flow.UntrustedChunkDataPack{
		ChunkID:           chunk.ID(), // TODO(ramtin): optimize this
		StartState:        attestRes.startStateCommit,
		Proof:             attestRes.stateProof,
		Collection:        collection,
		ExecutionDataRoot: *ar.ExecutionDataRoot,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build chunk data pack: %w", err)
	}

	return chunkDataPack, nil
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
