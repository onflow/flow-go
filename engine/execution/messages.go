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
	*ExecutionResults
	*AttestationResults

	*flow.ExecutionReceipt
}

func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
) *ComputationResult {
	return &ComputationResult{
		ExecutableBlock:    block,
		ExecutionResults:   NewEmptyExecutionResults(block),
		AttestationResults: NewEmptyAttestationResults(block),
	}
}

func (cr *ComputationResult) CollectionResult(colIndex int) *ColResSnapshot {
	if colIndex < 0 && colIndex > cr.ExecutedColCounter {
		return nil
	}
	return &ColResSnapshot{
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

// ExecutionResults captures artifacts of execution of block collections
type ExecutionResults struct {
	ExecutedColCounter     int
	StateSnapshots         []*state.ExecutionSnapshot
	Events                 []flow.EventsList
	ServiceEvents          []flow.EventsList
	ConvertedServiceEvents []flow.ServiceEventList
	TransactionResults     []flow.TransactionResult
	TransactionResultIndex []int

	// TODO(patrick): switch this to execution snapshot
	ComputationIntensities meter.MeteredComputationIntensities
}

func NewEmptyExecutionResults(
	block *entity.ExecutableBlock,
) *ExecutionResults {
	numCollections := len(block.CompleteCollections) + 1
	return &ExecutionResults{
		ExecutedColCounter:     0,
		StateSnapshots:         make([]*state.ExecutionSnapshot, 0, numCollections),
		Events:                 make([]flow.EventsList, numCollections),
		ServiceEvents:          make([]flow.EventsList, numCollections),
		ConvertedServiceEvents: make([]flow.ServiceEventList, numCollections),
		TransactionResults:     make([]flow.TransactionResult, 0),
		TransactionResultIndex: make([]int, 0),
		ComputationIntensities: make(meter.MeteredComputationIntensities),
	}
}

func (er *ExecutionResults) transactionResultsByCollectionIndex(colIndex int) []flow.TransactionResult {
	var startTxnIndex int
	if colIndex > 0 {
		startTxnIndex = er.TransactionResultIndex[colIndex-1]
	}
	endTxnIndex := er.TransactionResultIndex[colIndex]
	return er.TransactionResults[startTxnIndex:endTxnIndex]
}

func (er *ExecutionResults) AllServiceEvents() flow.EventsList {
	res := make(flow.EventsList, 0)
	for _, sv := range er.ServiceEvents {
		if len(sv) > 0 {
			res = append(res, sv...)
		}
	}
	return res
}

func (er *ExecutionResults) AllConvertedServiceEvents() flow.ServiceEventList {
	res := make(flow.ServiceEventList, 0)
	for _, sv := range er.ConvertedServiceEvents {
		if len(sv) > 0 {
			res = append(res, sv...)
		}
	}
	return res
}

// AttestationResults captures results of post-processsing execution results
type AttestationResults struct {
	AttestedColCounter int
	EventsHashes       []flow.Identifier
	Chunks             []*flow.Chunk
	ChunkDataPacks     []*flow.ChunkDataPack
	EndState           flow.StateCommitment
	*execution_data.BlockExecutionData
}

func NewEmptyAttestationResults(
	block *entity.ExecutableBlock,
) *AttestationResults {
	numCollections := len(block.CompleteCollections) + 1
	return &AttestationResults{
		AttestedColCounter: 0,
		EventsHashes:       make([]flow.Identifier, 0, numCollections),
		Chunks:             make([]*flow.Chunk, 0, numCollections),
		ChunkDataPacks:     make([]*flow.ChunkDataPack, 0, numCollections),
		BlockExecutionData: &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: make(
				[]*execution_data.ChunkExecutionData,
				0,
				numCollections),
		},
	}
}

func (cr *AttestationResults) AttestedCollection(colIndex int) *AttestedColSnapshot {
	if colIndex < 0 && colIndex > cr.AttestedColCounter {
		return nil
	}
	return &AttestedColSnapshot{
		startStateCommit: cr.Chunks[colIndex].StartState,
		endStateCommit:   cr.Chunks[colIndex].EndState,
		stateProof:       cr.ChunkDataPacks[colIndex].Proof,
		eventCommit:      cr.EventsHashes[colIndex],
	}
}

type ColResSnapshot struct {
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

func (c *ColResSnapshot) BlockHeader() *flow.Header {
	return c.blockHeader
}

func (c *ColResSnapshot) Collection() *flow.Collection {
	return c.collection
}

func (c *ColResSnapshot) CollectionIndex() int {
	return c.collectionIndex
}

func (c *ColResSnapshot) IsSystemCollection() bool {
	return c.isSystemCollection
}

func (c *ColResSnapshot) UpdatedRegisters() flow.RegisterEntries {
	return c.updatedRegisters
}

func (c *ColResSnapshot) ReadRegisterIDs() flow.RegisterIDs {
	return c.readRegisterIDs
}

func (c *ColResSnapshot) EmittedEvents() flow.EventsList {
	return c.emittedEvents
}

func (c *ColResSnapshot) ServiceEventList() flow.EventsList {
	return c.serviceEvents
}

func (c *ColResSnapshot) ConvertedServiceEvents() flow.ServiceEventList {
	return c.convertedServiceEvents
}

func (c *ColResSnapshot) TransactionResults() flow.TransactionResults {
	return c.transactionResults
}

func (c *ColResSnapshot) SpockData() []byte {
	return c.executionSnapshot.SpockSecret
}

func (c *ColResSnapshot) TotalComputationUsed() uint {
	return uint(c.executionSnapshot.TotalComputationUsed())
}

func (c *ColResSnapshot) ExecutionSnapshot() *state.ExecutionSnapshot {
	return c.executionSnapshot
}

type AttestedColSnapshot struct {
	ColResSnapshot
	startStateCommit flow.StateCommitment
	endStateCommit   flow.StateCommitment
	stateProof       flow.StorageProof
	eventCommit      flow.Identifier
}

func (a *AttestedColSnapshot) StartStateCommitment() flow.StateCommitment {
	return a.startStateCommit
}

func (a *AttestedColSnapshot) EndStateCommitment() flow.StateCommitment {
	return a.endStateCommit
}

func (a *AttestedColSnapshot) StateProof() flow.StorageProof {
	return a.stateProof
}

func (a *AttestedColSnapshot) EventListCommitment() flow.Identifier {
	return a.eventCommit
}
