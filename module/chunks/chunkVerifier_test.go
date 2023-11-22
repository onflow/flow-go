package chunks_test

import (
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	fvmmock "github.com/onflow/flow-go/fvm/mock"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	chunksmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

var eventsList = flow.EventsList{
	{
		Type:             "event.someType",
		TransactionID:    flow.Identifier{2, 3, 2, 3},
		TransactionIndex: 1,
		EventIndex:       2,
		Payload:          []byte{7, 3, 1, 2},
	},
	{
		Type:             "event.otherType",
		TransactionID:    flow.Identifier{3, 3, 3},
		TransactionIndex: 4,
		EventIndex:       4,
		Payload:          []byte{7, 3, 1, 2},
	},
}

const computationUsed = uint64(100)

var id0 = flow.NewRegisterID("00", "")
var id5 = flow.NewRegisterID("05", "")

// the chain we use for this test suite
var testChain = flow.Emulator
var epochSetupEvent, _ = unittest.EpochSetupFixtureByChainID(testChain)
var epochCommitEvent, _ = unittest.EpochCommitFixtureByChainID(testChain)

var systemEventsList = []flow.Event{
	epochSetupEvent,
}

var executionDataCIDProvider = provider.NewExecutionDataCIDProvider(execution_data.DefaultSerializer)

var serviceTxBody *flow.TransactionBody

type ChunkVerifierTestSuite struct {
	suite.Suite

	verifier *chunks.ChunkVerifier
	ledger   *completeLedger.Ledger

	snapshots map[string]*snapshot.ExecutionSnapshot
	outputs   map[string]fvm.ProcedureOutput
}

// Make sure variables are set properly
// SetupTest is executed prior to each individual test in this test suite
func (s *ChunkVerifierTestSuite) SetupSuite() {
	vmCtx := fvm.NewContext(fvm.WithChain(testChain.Chain()))
	vmMock := fvmmock.NewVM(s.T())

	vmMock.
		On("Run",
			mock.AnythingOfType("fvm.Context"),
			mock.AnythingOfType("*fvm.TransactionProcedure"),
			mock.AnythingOfType("snapshot.SnapshotTree")).
		Return(
			func(ctx fvm.Context, proc fvm.Procedure, storage snapshot.StorageSnapshot) *snapshot.ExecutionSnapshot {
				tx, ok := proc.(*fvm.TransactionProcedure)
				if !ok {
					s.Fail("unexpected procedure type")
					return nil
				}

				if snapshot, ok := s.snapshots[string(tx.Transaction.Script)]; ok {
					return snapshot
				}
				return generateDefaultSnapshot()
			},
			func(ctx fvm.Context, proc fvm.Procedure, storage snapshot.StorageSnapshot) fvm.ProcedureOutput {
				tx, ok := proc.(*fvm.TransactionProcedure)
				if !ok {
					s.Fail("unexpected procedure type")
					return fvm.ProcedureOutput{}
				}

				if output, ok := s.outputs[string(tx.Transaction.Script)]; ok {
					return output
				}
				return generateDefaultOutput()
			},
			func(ctx fvm.Context, proc fvm.Procedure, storage snapshot.StorageSnapshot) error {
				return nil
			},
		).
		Maybe() // don't require for all tests since some never call FVM

	s.verifier = chunks.NewChunkVerifier(vmMock, vmCtx, zerolog.Nop())

	txBody, err := blueprints.SystemChunkTransaction(testChain.Chain())
	require.NoError(s.T(), err)
	serviceTxBody = txBody
}

func (s *ChunkVerifierTestSuite) SetupTest() {
	s.ledger = newLedger(s.T())

	s.snapshots = make(map[string]*snapshot.ExecutionSnapshot)
	s.outputs = make(map[string]fvm.ProcedureOutput)
}

// TestChunkVerifier invokes all the tests in this test suite
func TestChunkVerifier(t *testing.T) {
	suite.Run(t, new(ChunkVerifierTestSuite))
}

// TestHappyPath tests verification of the baseline verifiable chunk
func (s *ChunkVerifierTestSuite) TestHappyPath() {
	meta := s.GetTestSetup(s.T(), "", false)
	vch := meta.RefreshChunkData(s.T())

	spockSecret, err := s.verifier.Verify(vch)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), spockSecret)
}

// TestMissingRegisterTouchForUpdate tests verification given a chunkdatapack missing a register touch (update)
func (s *ChunkVerifierTestSuite) TestMissingRegisterTouchForUpdate() {
	unittest.SkipUnless(s.T(), unittest.TEST_DEPRECATED, "Check new partial ledger for missing keys")

	meta := s.GetTestSetup(s.T(), "", false)
	vch := meta.RefreshChunkData(s.T())

	// remove the second register touch
	// vch.ChunkDataPack.RegisterTouches = vch.ChunkDataPack.RegisterTouches[:1]
	spockSecret, err := s.verifier.Verify(vch)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFMissingRegisterTouch{}, err)
	assert.Nil(s.T(), spockSecret)
}

// TestMissingRegisterTouchForRead tests verification given a chunkdatapack missing a register touch (read)
func (s *ChunkVerifierTestSuite) TestMissingRegisterTouchForRead() {
	unittest.SkipUnless(s.T(), unittest.TEST_DEPRECATED, "Check new partial ledger for missing keys")

	meta := s.GetTestSetup(s.T(), "", false)
	vch := meta.RefreshChunkData(s.T())

	// remove the second register touch
	// vch.ChunkDataPack.RegisterTouches = vch.ChunkDataPack.RegisterTouches[1:]
	spockSecret, err := s.verifier.Verify(vch)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFMissingRegisterTouch{}, err)
	assert.Nil(s.T(), spockSecret)
}

// TestWrongEndState tests verification covering the case
// the state commitment computed after updating the partial trie
// doesn't match the one provided by the chunks
func (s *ChunkVerifierTestSuite) TestWrongEndState() {
	meta := s.GetTestSetup(s.T(), "wrongEndState", false)
	vch := meta.RefreshChunkData(s.T())

	// modify calculated end state, which is different from the one provided by the vch
	s.snapshots["wrongEndState"] = &snapshot.ExecutionSnapshot{
		WriteSet: map[flow.RegisterID]flow.RegisterValue{
			id0: []byte{'F'},
		},
	}

	spockSecret, err := s.verifier.Verify(vch)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFNonMatchingFinalState{}, err)
	assert.Nil(s.T(), spockSecret)
}

// TestFailedTx tests verification behavior in case
// of failed transaction. if a transaction fails, it should
// still change the state commitment.
func (s *ChunkVerifierTestSuite) TestFailedTx() {
	meta := s.GetTestSetup(s.T(), "failedTx", false)
	vch := meta.RefreshChunkData(s.T())

	// modify the FVM output to include a failing tx. the input already has a failing tx, but we need to
	s.snapshots["failedTx"] = &snapshot.ExecutionSnapshot{
		WriteSet: map[flow.RegisterID]flow.RegisterValue{
			id5: []byte{'B'},
		},
	}
	s.outputs["failedTx"] = fvm.ProcedureOutput{
		ComputationUsed: computationUsed,
		Err:             fvmErrors.NewCadenceRuntimeError(runtime.Error{}), // inside the runtime (e.g. div by zero, access account)
	}

	spockSecret, err := s.verifier.Verify(vch)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), spockSecret)
}

// TestEventsMismatch tests verification behavior in case
// of emitted events not matching chunks
func (s *ChunkVerifierTestSuite) TestEventsMismatch() {
	meta := s.GetTestSetup(s.T(), "eventsMismatch", false)
	vch := meta.RefreshChunkData(s.T())

	// add an additional event to the list of events produced by FVM
	output := generateDefaultOutput()
	output.Events = append(eventsList, flow.Event{
		Type:             "event.Extra",
		TransactionID:    flow.Identifier{2, 3},
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          []byte{88},
	})
	s.outputs["eventsMismatch"] = output

	_, err := s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFInvalidEventsCollection{}, err)
}

// TestServiceEventsMismatch tests verification behavior in case
// of emitted service events not matching chunks'
func (s *ChunkVerifierTestSuite) TestServiceEventsMismatch() {
	meta := s.GetTestSetup(s.T(), "doesn't matter", true)
	vch := meta.RefreshChunkData(s.T())

	// modify the list of service events produced by FVM
	// EpochSetup event is expected, but we emit EpochCommit here resulting in a chunk fault
	epochCommitServiceEvent, err := convert.ServiceEvent(testChain, epochCommitEvent)
	require.NoError(s.T(), err)

	s.snapshots[string(serviceTxBody.Script)] = &snapshot.ExecutionSnapshot{}
	s.outputs[string(serviceTxBody.Script)] = fvm.ProcedureOutput{
		ComputationUsed:        computationUsed,
		ConvertedServiceEvents: flow.ServiceEventList{*epochCommitServiceEvent},
		Events:                 meta.ChunkEvents,
	}

	_, err = s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFInvalidServiceEventsEmitted{}, err)
}

// TestServiceEventsAreChecked ensures that service events are in fact checked
func (s *ChunkVerifierTestSuite) TestServiceEventsAreChecked() {
	meta := s.GetTestSetup(s.T(), "doesn't matter", true)
	vch := meta.RefreshChunkData(s.T())

	// setup the verifier output to include the correct data for the service events
	output := generateDefaultOutput()
	output.ConvertedServiceEvents = meta.ServiceEvents
	output.Events = meta.ChunkEvents
	s.outputs[string(serviceTxBody.Script)] = output

	_, err := s.verifier.Verify(vch)
	assert.NoError(s.T(), err)
}

// TestSystemChunkWithCollectionFails ensures verification fails for system chunks with collections
func (s *ChunkVerifierTestSuite) TestSystemChunkWithCollectionFails() {
	meta := s.GetTestSetup(s.T(), "doesn't matter", true)

	// add a collection to the system chunk
	col := unittest.CollectionFixture(1)
	meta.Collection = &col

	vch := meta.RefreshChunkData(s.T())

	_, err := s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFSystemChunkIncludedCollection{}, err)
}

// TestEmptyCollection tests verification behaviour if a
// collection doesn't have any transaction.
func (s *ChunkVerifierTestSuite) TestEmptyCollection() {
	meta := s.GetTestSetup(s.T(), "", false)

	// reset test to use an empty collection
	collection := unittest.CollectionFixture(0)
	meta.Collection = &collection
	meta.ChunkEvents = nil
	meta.TxResults = nil

	// update the Update to not change the state
	update, err := ledger.NewEmptyUpdate(meta.StartState)
	require.NoError(s.T(), err)

	meta.Update = update

	vch := meta.RefreshChunkData(s.T())

	spockSecret, err := s.verifier.Verify(vch)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), spockSecret)
}

func (s *ChunkVerifierTestSuite) TestExecutionDataBlockMismatch() {
	meta := s.GetTestSetup(s.T(), "", false)

	// modify Block in the ExecutionDataRoot
	meta.ExecDataBlockID = unittest.IdentifierFixture()

	vch := meta.RefreshChunkData(s.T())

	_, err := s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFExecutionDataBlockIDMismatch{}, err)
}

func (s *ChunkVerifierTestSuite) TestExecutionDataChunkIdsLengthDiffers() {
	meta := s.GetTestSetup(s.T(), "", false)
	vch := meta.RefreshChunkData(s.T())

	// add an additional ChunkExecutionDataID into the ExecutionDataRoot passed into Verify
	vch.ChunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs = append(vch.ChunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs, cid.Undef)

	_, err := s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFExecutionDataChunksLengthMismatch{}, err)
}

func (s *ChunkVerifierTestSuite) TestExecutionDataChunkIdMismatch() {
	meta := s.GetTestSetup(s.T(), "", false)
	vch := meta.RefreshChunkData(s.T())

	// modify one of the ChunkExecutionDataIDs passed into Verify
	vch.ChunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs[0] = cid.Undef // substitute invalid CID

	_, err := s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFExecutionDataInvalidChunkCID{}, err)
}

func (s *ChunkVerifierTestSuite) TestExecutionDataIdMismatch() {
	meta := s.GetTestSetup(s.T(), "", false)
	vch := meta.RefreshChunkData(s.T())

	// modify ExecutionDataID passed into Verify
	vch.Result.ExecutionDataID[5]++

	_, err := s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFInvalidExecutionDataID{}, err)
}

func newLedger(t *testing.T) *completeLedger.Ledger {
	f, err := completeLedger.NewLedger(&fixtures.NoopWAL{}, 1000, metrics.NewNoopCollector(), zerolog.Nop(), completeLedger.DefaultPathFinderVersion)
	require.NoError(t, err)

	compactor := fixtures.NewNoopCompactor(f)
	<-compactor.Ready()

	t.Cleanup(func() {
		<-f.Done()
		<-compactor.Done()
	})

	return f
}

func blockFixture(collection *flow.Collection) *flow.Block {
	guarantee := collection.Guarantee()
	block := &flow.Block{
		Header: unittest.BlockHeaderFixture(),
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}
	block.Header.PayloadHash = block.Payload.Hash()
	return block
}

func generateStateUpdates(t *testing.T, f *completeLedger.Ledger) (ledger.State, ledger.Proof, *ledger.Update) {
	id1 := flow.NewRegisterID("00", "")
	id2 := flow.NewRegisterID("05", "")

	entries := flow.RegisterEntries{
		{
			Key:   id1,
			Value: []byte{'a'},
		},
		{
			Key:   id2,
			Value: []byte{'b'},
		},
	}

	keys, values := executionState.RegisterEntriesToKeysValues(entries)
	update, err := ledger.NewUpdate(f.InitialState(), keys, values)
	require.NoError(t, err)

	startState, _, err := f.Set(update)
	require.NoError(t, err)

	query, err := ledger.NewQuery(startState, keys)
	require.NoError(t, err)

	proof, err := f.Prove(query)
	require.NoError(t, err)

	entries = flow.RegisterEntries{
		{
			Key:   id2,
			Value: []byte{'B'},
		},
	}

	keys, values = executionState.RegisterEntriesToKeysValues(entries)
	update, err = ledger.NewUpdate(startState, keys, values)
	require.NoError(t, err)

	return startState, proof, update
}

func generateExecutionData(t *testing.T, blockID flow.Identifier, ced *execution_data.ChunkExecutionData) (flow.Identifier, flow.BlockExecutionDataRoot) {
	chunkCid, err := executionDataCIDProvider.CalculateChunkExecutionDataID(*ced)
	require.NoError(t, err)

	executionDataRoot := flow.BlockExecutionDataRoot{
		BlockID:               blockID,
		ChunkExecutionDataIDs: []cid.Cid{chunkCid},
	}

	executionDataID, err := executionDataCIDProvider.CalculateExecutionDataRootID(executionDataRoot)
	require.NoError(t, err)

	return executionDataID, executionDataRoot
}

func generateEvents(t *testing.T, isSystemChunk bool, collection *flow.Collection) (flow.EventsList, []flow.ServiceEvent) {
	var chunkEvents flow.EventsList
	serviceEvents := make([]flow.ServiceEvent, 0)

	// service events are also included as regular events
	if isSystemChunk {
		for _, e := range systemEventsList {
			e := e
			event, err := convert.ServiceEvent(testChain, e)
			require.NoError(t, err)

			serviceEvents = append(serviceEvents, *event)
			chunkEvents = append(chunkEvents, e)
		}
	}

	for _, coll := range collection.Transactions {
		switch string(coll.Script) {
		case "failedTx":
			continue
		}
		chunkEvents = append(chunkEvents, eventsList...)
	}

	return chunkEvents, serviceEvents
}

func generateTransactionResults(t *testing.T, collection *flow.Collection) []flow.LightTransactionResult {
	txResults := make([]flow.LightTransactionResult, len(collection.Transactions))
	for i, tx := range collection.Transactions {
		txResults[i] = flow.LightTransactionResult{
			TransactionID:   tx.ID(),
			ComputationUsed: computationUsed,
			Failed:          false,
		}

		if string(tx.Script) == "failedTx" {
			txResults[i].Failed = true
		}
	}

	return txResults
}

func generateCollection(t *testing.T, isSystemChunk bool, script string) *flow.Collection {
	if isSystemChunk {
		// the system chunk's data pack does not include the collection, but the execution data does.
		// we must include the correct collection in the execution data, otherwise verification will fail.
		return &flow.Collection{
			Transactions: []*flow.TransactionBody{serviceTxBody},
		}
	}

	collectionSize := 5
	magicTxIndex := 3

	coll := unittest.CollectionFixture(collectionSize)
	if script != "" {
		coll.Transactions[magicTxIndex] = &flow.TransactionBody{Script: []byte(script)}
	}

	return &coll
}

func generateDefaultSnapshot() *snapshot.ExecutionSnapshot {
	return &snapshot.ExecutionSnapshot{
		ReadSet: map[flow.RegisterID]struct{}{
			id0: {},
			id5: {},
		},
		WriteSet: map[flow.RegisterID]flow.RegisterValue{
			id5: []byte{'B'},
		},
	}
}

func generateDefaultOutput() fvm.ProcedureOutput {
	return fvm.ProcedureOutput{
		ComputationUsed: computationUsed,
		Logs:            []string{"log1", "log2"},
		Events:          eventsList,
	}
}

func (s *ChunkVerifierTestSuite) GetTestSetup(t *testing.T, script string, system bool) *testMetadata {
	collection := generateCollection(t, system, script)
	block := blockFixture(collection)

	// transaction results
	txResults := generateTransactionResults(t, collection)
	// make sure this includes results even for the service tx
	if system {
		require.Len(t, txResults, 1)
	} else {
		require.Len(t, txResults, len(collection.Transactions))
	}

	// events
	chunkEvents, serviceEvents := generateEvents(t, system, collection)
	// make sure this includes events even for the service tx
	require.NotEmpty(t, chunkEvents)
	if system {
		require.Len(t, serviceEvents, 1)
	} else {
		require.Empty(t, serviceEvents)
	}

	// registerTouch and State setup
	startState, proof, update := generateStateUpdates(t, s.ledger)

	if system {
		collection = nil
	}

	meta := &testMetadata{
		IsSystemChunk: system,
		Header:        block.Header,
		Collection:    collection,
		TxResults:     txResults,
		ChunkEvents:   chunkEvents,
		ServiceEvents: serviceEvents,
		StartState:    startState,
		Update:        update,
		Proof:         proof,

		ExecDataBlockID: block.Header.ID(),

		ledger: s.ledger,
	}

	return meta
}

type testMetadata struct {
	IsSystemChunk bool
	Header        *flow.Header
	Collection    *flow.Collection
	TxResults     []flow.LightTransactionResult
	ChunkEvents   flow.EventsList
	ServiceEvents []flow.ServiceEvent
	StartState    ledger.State
	Update        *ledger.Update
	Proof         ledger.Proof

	// separated to allow overriding
	ExecDataBlockID flow.Identifier

	ledger *completeLedger.Ledger
}

func (m *testMetadata) RefreshChunkData(t *testing.T) *verification.VerifiableChunkData {
	cedCollection := m.Collection

	if m.IsSystemChunk {
		// the system chunk's data pack does not include the collection, but the execution data does.
		// we must include the correct collection in the execution data, otherwise verification will fail.
		cedCollection = &flow.Collection{
			Transactions: []*flow.TransactionBody{serviceTxBody},
		}
	}

	endState, trieUpdate, err := m.ledger.Set(m.Update)
	require.NoError(t, err)

	eventsMerkleRootHash, err := flow.EventsMerkleRootHash(m.ChunkEvents)
	require.NoError(t, err)

	chunkExecutionData := &execution_data.ChunkExecutionData{
		Collection:         cedCollection,
		Events:             m.ChunkEvents,
		TrieUpdate:         trieUpdate,
		TransactionResults: m.TxResults,
	}

	executionDataID, executionDataRoot := generateExecutionData(t, m.ExecDataBlockID, chunkExecutionData)

	// Chunk setup
	chunk := &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: 0,
			StartState:      flow.StateCommitment(m.StartState),
			BlockID:         m.Header.ID(),
			EventCollection: eventsMerkleRootHash,
		},
		Index: 0,
	}

	chunkDataPack := &flow.ChunkDataPack{
		ChunkID:           chunk.ID(),
		StartState:        flow.StateCommitment(m.StartState),
		Proof:             m.Proof,
		Collection:        m.Collection,
		ExecutionDataRoot: executionDataRoot,
	}

	// ExecutionResult setup
	result := &flow.ExecutionResult{
		BlockID:         m.Header.ID(),
		Chunks:          flow.ChunkList{chunk},
		ServiceEvents:   m.ServiceEvents,
		ExecutionDataID: executionDataID,
	}

	return &verification.VerifiableChunkData{
		IsSystemChunk: m.IsSystemChunk,
		Header:        m.Header,
		Chunk:         chunk,
		Result:        result,
		ChunkDataPack: chunkDataPack,
		EndState:      flow.StateCommitment(endState),
	}
}
