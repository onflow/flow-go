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
	*BlockExecutionResults
	*BlockAttestationResults

	*flow.ExecutionReceipt
}

func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
) *ComputationResult {
	return &ComputationResult{
		ExecutableBlock:         block,
		BlockExecutionResults:   NewEmptyBlockExecutionResults(block),
		BlockAttestationResults: NewEmptyBlockAttestationResults(block),
	}
}

func (cr *ComputationResult) CollectionExecutionResult(colIndex int) *CollectionExecutionResult {
	if colIndex < 0 && colIndex > len(cr.StateSnapshots) {
		return nil
	}
	return &CollectionExecutionResult{
		blockHeader: cr.Block.Header,
		collection: &flow.Collection{
			Transactions: cr.CollectionAt(colIndex).Transactions,
		},
		collectionIndex:        colIndex,
		isSystemCollection:     colIndex == len(cr.CompleteCollections),
		updatedRegisters:       cr.StateSnapshots[colIndex].UpdatedRegisters(),
		readRegisterIDs:        cr.StateSnapshots[colIndex].ReadRegisterIDs(),
		emittedEvents:          cr.Events[colIndex],
		transactionResults:     cr.transactionResultsByCollectionIndex(colIndex),
		executionSnapshot:      cr.StateSnapshots[colIndex],
		serviceEvents:          cr.ServiceEvents[colIndex],
		convertedServiceEvents: cr.ConvertedServiceEvents[colIndex],
	}
}

// BlockExecutionResults captures artifacts of execution of block collections
type BlockExecutionResults struct {
	StateSnapshots         []*state.ExecutionSnapshot
	Events                 []flow.EventsList
	ServiceEvents          []flow.EventsList
	ConvertedServiceEvents []flow.ServiceEventList
	TransactionResults     []flow.TransactionResult
	TransactionResultIndex []int

	// TODO(patrick): switch this to execution snapshot
	ComputationIntensities meter.MeteredComputationIntensities
}

func NewEmptyBlockExecutionResults(
	block *entity.ExecutableBlock,
) *BlockExecutionResults {
	numCollections := len(block.CompleteCollections) + 1
	return &BlockExecutionResults{
		StateSnapshots:         make([]*state.ExecutionSnapshot, 0, numCollections),
		Events:                 make([]flow.EventsList, numCollections),
		ServiceEvents:          make([]flow.EventsList, numCollections),
		ConvertedServiceEvents: make([]flow.ServiceEventList, numCollections),
		TransactionResults:     make([]flow.TransactionResult, 0),
		TransactionResultIndex: make([]int, 0),
		ComputationIntensities: make(meter.MeteredComputationIntensities),
	}
}

func (er *BlockExecutionResults) transactionResultsByCollectionIndex(colIndex int) []flow.TransactionResult {
	var startTxnIndex int
	if colIndex > 0 {
		startTxnIndex = er.TransactionResultIndex[colIndex-1]
	}
	endTxnIndex := er.TransactionResultIndex[colIndex]
	return er.TransactionResults[startTxnIndex:endTxnIndex]
}

func (er *BlockExecutionResults) AllServiceEvents() flow.EventsList {
	res := make(flow.EventsList, 0)
	for _, sv := range er.ServiceEvents {
		if len(sv) > 0 {
			res = append(res, sv...)
		}
	}
	return res
}

func (er *BlockExecutionResults) AllConvertedServiceEvents() flow.ServiceEventList {
	res := make(flow.ServiceEventList, 0)
	for _, sv := range er.ConvertedServiceEvents {
		if len(sv) > 0 {
			res = append(res, sv...)
		}
	}
	return res
}

// BlockAttestationResults captures results of post-processsing execution results
type BlockAttestationResults struct {
	EventsHashes   []flow.Identifier
	Chunks         []*flow.Chunk
	ChunkDataPacks []*flow.ChunkDataPack
	EndState       flow.StateCommitment
	*execution_data.BlockExecutionData
}

func NewEmptyBlockAttestationResults(
	block *entity.ExecutableBlock,
) *BlockAttestationResults {
	numCollections := len(block.CompleteCollections) + 1
	return &BlockAttestationResults{
		EventsHashes:   make([]flow.Identifier, 0, numCollections),
		Chunks:         make([]*flow.Chunk, 0, numCollections),
		ChunkDataPacks: make([]*flow.ChunkDataPack, 0, numCollections),
		EndState:       *block.StartState,
		BlockExecutionData: &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: make(
				[]*execution_data.ChunkExecutionData,
				0,
				numCollections),
		},
	}
}

func (cr *BlockAttestationResults) AttestedCollection(colIndex int) *CollectionAttestationResult {
	if colIndex < 0 && colIndex > len(cr.Chunks) {
		return nil
	}
	return &CollectionAttestationResult{
		startStateCommit: cr.Chunks[colIndex].StartState,
		endStateCommit:   cr.Chunks[colIndex].EndState,
		stateProof:       cr.ChunkDataPacks[colIndex].Proof,
		eventCommit:      cr.EventsHashes[colIndex],
	}
}

type CollectionExecutionResult struct {
	blockHeader            *flow.Header
	collection             *flow.Collection
	collectionIndex        int
	isSystemCollection     bool
	updatedRegisters       flow.RegisterEntries
	readRegisterIDs        flow.RegisterIDs
	emittedEvents          flow.EventsList
	serviceEvents          flow.EventsList
	convertedServiceEvents flow.ServiceEventList
	transactionResults     flow.TransactionResults
	executionSnapshot      *state.ExecutionSnapshot
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
	return c.updatedRegisters
}

func (c *CollectionExecutionResult) ReadRegisterIDs() flow.RegisterIDs {
	return c.readRegisterIDs
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
	CollectionExecutionResult
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
