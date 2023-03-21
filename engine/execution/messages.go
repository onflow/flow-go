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
	StateSnapshots []*state.ExecutionSnapshot
	Events         []flow.EventsList

	TransactionResults     []flow.TransactionResult
	TransactionResultIndex []int

	// TODO(patrick): switch this to execution snapshot
	ComputationIntensities meter.MeteredComputationIntensities
}

func NewEmptyComputationResult(
	block *entity.ExecutableBlock,
) *ComputationResult {
	numCollections := len(block.CompleteCollections) + 1
	return &ComputationResult{
		StateSnapshots:         make([]*state.ExecutionSnapshot, 0, numCollections),
		ExecutableBlock:        block,
		Events:                 make([]flow.EventsList, numCollections),
		TransactionResults:     make([]flow.TransactionResult, 0),
		TransactionResultIndex: make([]int, 0),
		ComputationIntensities: make(meter.MeteredComputationIntensities),
	}
}

func (cr ComputationResult) TxResByColIndex(colIndex int) []flow.TransactionResult {
	var startTxnIndex int
	if colIndex > 0 {
		startTxnIndex = cr.TransactionResultIndex[colIndex-1]
	}
	endTxnIndex := cr.TransactionResultIndex[colIndex]
	return cr.TransactionResults[startTxnIndex:endTxnIndex]
}

func (cr *ComputationResult) CollectionResult(colIndex int) *ColResSnapshot {
	if colIndex < 0 && colIndex > len(cr.CompleteCollections) {
		return nil
	}
	return &ColResSnapshot{
		blockHeader: cr.Block.Header,
		collection: &flow.Collection{
			Transactions: cr.CollectionAt(colIndex).Transactions,
		},
		collectionIndex:    colIndex,
		isSystemCollection: colIndex == len(cr.CompleteCollections),
		updatedRegisters:   cr.StateSnapshots[colIndex].UpdatedRegisters(),
		readRegisterIDs:    cr.StateSnapshots[colIndex].ReadRegisterIDs(),
		spockData:          cr.StateSnapshots[colIndex].SpockSecret,
		emittedEvents:      cr.Events[colIndex],
		transactionResults: cr.TxResByColIndex(colIndex),
		executionSnapshot:  cr.StateSnapshots[colIndex],
	}
}

type ColResSnapshot struct {
	*entity.ExecutableBlock
	blockHeader          *flow.Header
	collection           *flow.Collection
	updatedRegisters     flow.RegisterEntries
	readRegisterIDs      flow.RegisterIDs
	emittedEvents        flow.EventsList
	transactionResults   flow.TransactionResults
	spockData            []byte
	totalComputationUsed uint
	isSystemCollection   bool
	collectionIndex      int
	executionSnapshot    *state.ExecutionSnapshot
}

func (c *ColResSnapshot) BlockHeader() *flow.Header {
	return c.blockHeader
}

func (c *ColResSnapshot) SpockData() []byte {
	return c.spockData
}

func (c *ColResSnapshot) TotalComputationUsed() uint {
	return c.totalComputationUsed
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

func (c *ColResSnapshot) TransactionResults() flow.TransactionResults {
	return c.transactionResults
}

func (c *ColResSnapshot) ExecutionSnapshot() *state.ExecutionSnapshot {
	return c.executionSnapshot
}

type AttestationResults struct {
	ColResSnapshots []ColResSnapshot
	EventsHashes    []flow.Identifier
	Chunks          []*flow.Chunk
	ChunkDataPacks  []*flow.ChunkDataPack
	EndState        flow.StateCommitment
	*execution_data.BlockExecutionData
}

func NewEmptyAttestationResult(
	block *entity.ExecutableBlock,
) *AttestationResults {
	numCollections := len(block.CompleteCollections) + 1
	return &AttestationResults{
		EventsHashes:   make([]flow.Identifier, 0, numCollections),
		Chunks:         make([]*flow.Chunk, 0, numCollections),
		ChunkDataPacks: make([]*flow.ChunkDataPack, 0, numCollections),
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
	if colIndex < 0 && colIndex > len(cr.ColResSnapshots) {
		return nil
	}
	return &AttestedColSnapshot{
		colResSnapshot:   &cr.ColResSnapshots[colIndex],
		startStateCommit: cr.Chunks[colIndex].StartState,
		endStateCommit:   cr.Chunks[colIndex].EndState,
		stateProof:       cr.ChunkDataPacks[colIndex].Proof,
		eventCommit:      cr.EventsHashes[colIndex],
	}
}

type AttestedColSnapshot struct {
	colResSnapshot   *ColResSnapshot
	startStateCommit flow.StateCommitment
	endStateCommit   flow.StateCommitment
	stateProof       flow.StorageProof
	eventCommit      flow.Identifier
}

func (a *AttestedColSnapshot) BlockHeader() *flow.Header {
	return a.colResSnapshot.blockHeader
}

func (a *AttestedColSnapshot) SpockData() []byte {
	return a.colResSnapshot.spockData
}

func (a *AttestedColSnapshot) TotalComputationUsed() uint {
	return a.colResSnapshot.totalComputationUsed
}

func (a *AttestedColSnapshot) Collection() *flow.Collection {
	return a.colResSnapshot.collection
}

func (a *AttestedColSnapshot) CollectionIndex() int {
	return a.colResSnapshot.collectionIndex
}

func (a *AttestedColSnapshot) IsSystemCollection() bool {
	return a.colResSnapshot.isSystemCollection
}

func (a *AttestedColSnapshot) UpdatedRegisters() flow.RegisterEntries {
	return a.colResSnapshot.updatedRegisters
}

func (a *AttestedColSnapshot) ReadRegisterIDs() flow.RegisterIDs {
	return a.colResSnapshot.readRegisterIDs
}

func (a *AttestedColSnapshot) EmittedEvents() flow.EventsList {
	return a.colResSnapshot.emittedEvents
}

func (a *AttestedColSnapshot) TransactionResults() flow.TransactionResults {
	return a.colResSnapshot.transactionResults
}

func (c *AttestedColSnapshot) ExecutionSnapshot() *state.ExecutionSnapshot {
	return c.colResSnapshot.executionSnapshot
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
