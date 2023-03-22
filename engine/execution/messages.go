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
	*BlockExecutionResults
	*BlockAttestationResults

	*flow.ExecutionReceipt
}

func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
) *ComputationResult {
	return &ComputationResult{
		BlockExecutionResults:   NewEmptyBlockExecutionResults(block),
		BlockAttestationResults: NewEmptyBlockAttestationResults(block),
	}
}

// BlockExecutionResults captures artifacts of execution of block collections
type BlockExecutionResults struct {
	CollectionExecutionResults []CollectionExecutionResult

	// TODO(patrick): switch this to execution snapshot
	ComputationIntensities meter.MeteredComputationIntensities
}

func (er *BlockExecutionResults) CollectionExecutionResult(colIndex int) *CollectionExecutionResult {
	if colIndex < 0 && colIndex > len(er.CollectionExecutionResults) {
		return nil
	}
	return &er.CollectionExecutionResults[colIndex]
}

func NewEmptyBlockExecutionResults(
	block *entity.ExecutableBlock,
) *BlockExecutionResults {
	colExecResults := make([]CollectionExecutionResult, 0, len(block.CompleteCollections)+1)
	for i := 0; i < len(block.CompleteCollections); i++ {
		compCol := block.CollectionAt(i)
		col := compCol.Collection()
		colExecResults = append(colExecResults,
			NewEmptyCollectionExecutionResult(
				block.Block.Header,
				&col,
				i,
				false,
			),
		)
	}
	colExecResults = append(colExecResults,
		NewEmptyCollectionExecutionResult(
			block.Block.Header,
			nil,
			len(block.CompleteCollections),
			true,
		),
	)
	return &BlockExecutionResults{
		CollectionExecutionResults: colExecResults,
		ComputationIntensities:     make(meter.MeteredComputationIntensities),
	}
}

func (er *BlockExecutionResults) AllServiceEvents() flow.EventsList {
	res := make(flow.EventsList, 0)
	for _, ce := range er.CollectionExecutionResults {
		if len(ce.serviceEvents) > 0 {
			res = append(res, ce.serviceEvents...)
		}
	}
	return res
}

func (er *BlockExecutionResults) AllConvertedServiceEvents() flow.ServiceEventList {
	res := make(flow.ServiceEventList, 0)
	for _, ce := range er.CollectionExecutionResults {
		if len(ce.convertedServiceEvents) > 0 {
			res = append(res, ce.convertedServiceEvents...)
		}
	}
	return res
}

// BlockAttestationResults captures results of post-processsing execution results
type BlockAttestationResults struct {
	*entity.ExecutableBlock
	CollectionAttestationResults []CollectionAttestationResult

	*execution_data.BlockExecutionData // TODO(ramtin): move this logic to outside
}

func NewEmptyBlockAttestationResults(
	block *entity.ExecutableBlock,
) *BlockAttestationResults {
	numCollections := len(block.CompleteCollections) + 1
	return &BlockAttestationResults{
		ExecutableBlock:              block,
		CollectionAttestationResults: make([]CollectionAttestationResult, 0, numCollections),
		BlockExecutionData: &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: make(
				[]*execution_data.ChunkExecutionData,
				0,
				numCollections),
		},
	}
}

func (ar *BlockAttestationResults) AppendCollectionAttestationResult(
	colExecRes *CollectionExecutionResult,
	startStateCommit flow.StateCommitment,
	endStateCommit flow.StateCommitment,
	stateProof flow.StorageProof,
	eventCommit flow.Identifier,
	chunkExecutionDatas *execution_data.ChunkExecutionData,
) {
	ar.CollectionAttestationResults = append(ar.CollectionAttestationResults,
		CollectionAttestationResult{
			CollectionExecutionResult: colExecRes,
			startStateCommit:          startStateCommit,
			endStateCommit:            endStateCommit,
			stateProof:                stateProof,
			eventCommit:               eventCommit,
		},
	)
	ar.ChunkExecutionDatas = append(ar.ChunkExecutionDatas, chunkExecutionDatas)
}

func (ar *BlockAttestationResults) CollectionAttestationResult(colIndex int) *CollectionAttestationResult {
	if colIndex < 0 && colIndex > len(ar.CollectionAttestationResults) {
		return nil
	}
	return &ar.CollectionAttestationResults[colIndex]
}

func (cr *BlockAttestationResults) InterimEndState() flow.StateCommitment {
	if len(cr.CollectionAttestationResults) == 0 {
		return *cr.StartState
	}
	return cr.CollectionAttestationResults[len(cr.CollectionAttestationResults)-1].endStateCommit
}

// TODO(ramtin): cache and optimize this
func (cr *BlockAttestationResults) AllChunks() []*flow.Chunk {
	chunks := make([]*flow.Chunk, len(cr.CollectionAttestationResults))
	for i := 0; i < len(cr.CollectionAttestationResults); i++ {
		chunks[i] = cr.ChunkAt(i)
	}
	return chunks
}

func (cr *BlockAttestationResults) ChunkAt(index int) *flow.Chunk {
	if index >= len(cr.CollectionAttestationResults) {
		return nil
	}
	res := cr.CollectionAttestationResults[index]

	return flow.NewChunk(
		cr.Block.ID(), // TODO(ramtin): cache block ID
		index,
		res.startStateCommit,
		res.collection.Len(),
		res.eventCommit,
		res.endStateCommit,
	)
}

func (cr *BlockAttestationResults) ChunkDataPackAt(index int) *flow.ChunkDataPack {
	if index >= len(cr.CollectionAttestationResults) {
		return nil
	}
	res := cr.CollectionAttestationResults[index]

	return flow.NewChunkDataPack(
		cr.ChunkAt(index).ID(), // TODO(ramtin): optimize this
		res.startStateCommit,
		res.stateProof,
		res.collection,
	)
}

type CollectionExecutionResult struct {
	blockHeader            *flow.Header
	collection             *flow.Collection
	collectionIndex        int
	isSystemCollection     bool
	emittedEvents          flow.EventsList
	serviceEvents          flow.EventsList
	convertedServiceEvents flow.ServiceEventList
	transactionResults     flow.TransactionResults
	executionSnapshot      *state.ExecutionSnapshot
}

func NewEmptyCollectionExecutionResult(
	blockHeader *flow.Header,
	collection *flow.Collection,
	collectionIndex int,
	isSystemCollection bool,
) CollectionExecutionResult {
	return CollectionExecutionResult{
		blockHeader:            blockHeader,
		collection:             collection,
		collectionIndex:        collectionIndex,
		isSystemCollection:     isSystemCollection,
		emittedEvents:          make(flow.EventsList, 0),
		serviceEvents:          make(flow.EventsList, 0),
		convertedServiceEvents: make(flow.ServiceEventList, 0),
		transactionResults:     make(flow.TransactionResults, 0, collection.Len()),
	}
}

func (c *CollectionExecutionResult) AppendTransactionResults(
	emittedEvents flow.EventsList,
	serviceEvents flow.EventsList,
	convertedServiceEvents flow.ServiceEventList,
	transactionResult flow.TransactionResult,
) {
	c.emittedEvents = append(c.emittedEvents, emittedEvents...)
	c.serviceEvents = append(c.serviceEvents, serviceEvents...)
	c.convertedServiceEvents = append(c.convertedServiceEvents, convertedServiceEvents...)
	c.transactionResults = append(c.transactionResults, transactionResult)
}

func (c *CollectionExecutionResult) UpdateExecutionSnapshot(
	executionSnapshot *state.ExecutionSnapshot,
) {
	c.executionSnapshot = executionSnapshot
}

func (c *CollectionExecutionResult) BlockHeader() *flow.Header {
	return c.blockHeader
}

func (c *CollectionExecutionResult) Collection() *flow.Collection {
	return c.collection
}

func (c *CollectionExecutionResult) CollectionIndex() int {
	return c.collectionIndex
}

func (c *CollectionExecutionResult) IsSystemCollection() bool {
	return c.isSystemCollection
}

func (c *CollectionExecutionResult) UpdatedRegisters() flow.RegisterEntries {
	return c.executionSnapshot.UpdatedRegisters()
}

func (c *CollectionExecutionResult) ReadRegisterIDs() flow.RegisterIDs {
	return c.executionSnapshot.ReadRegisterIDs()
}

func (c *CollectionExecutionResult) EmittedEvents() flow.EventsList {
	return c.emittedEvents
}

func (c *CollectionExecutionResult) ServiceEventList() flow.EventsList {
	return c.serviceEvents
}

func (c *CollectionExecutionResult) ConvertedServiceEvents() flow.ServiceEventList {
	return c.convertedServiceEvents
}

func (c *CollectionExecutionResult) TransactionResults() flow.TransactionResults {
	return c.transactionResults
}

func (c *CollectionExecutionResult) SpockData() []byte {
	return c.executionSnapshot.SpockSecret
}

func (c *CollectionExecutionResult) TotalComputationUsed() uint {
	return uint(c.executionSnapshot.TotalComputationUsed())
}

func (c *CollectionExecutionResult) ExecutionSnapshot() *state.ExecutionSnapshot {
	return c.executionSnapshot
}

type CollectionAttestationResult struct {
	*CollectionExecutionResult
	startStateCommit flow.StateCommitment
	endStateCommit   flow.StateCommitment
	stateProof       flow.StorageProof
	eventCommit      flow.Identifier
}

func (a *CollectionAttestationResult) StartStateCommitment() flow.StateCommitment {
	return a.startStateCommit
}

func (a *CollectionAttestationResult) EndStateCommitment() flow.StateCommitment {
	return a.endStateCommit
}

func (a *CollectionAttestationResult) StateProof() flow.StorageProof {
	return a.stateProof
}

func (a *CollectionAttestationResult) EventListCommitment() flow.Identifier {
	return a.eventCommit
}
