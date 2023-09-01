package chunks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/ledger/partial"
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

// the chain we use for this test suite
var testChain = flow.Emulator
var epochSetupEvent, _ = unittest.EpochSetupFixtureByChainID(testChain)
var epochCommitEvent, _ = unittest.EpochCommitFixtureByChainID(testChain)

var epochSetupServiceEvent, _ = convert.ServiceEvent(testChain, epochSetupEvent)
var epochCommitServiceEvent, _ = convert.ServiceEvent(testChain, epochCommitEvent)

var serviceEventsList = []flow.ServiceEvent{
	*epochSetupServiceEvent,
}

var executionDataCIDProvider = provider.NewExecutionDataCIDProvider(execution_data.DefaultSerializer)

type ChunkVerifierTestSuite struct {
	suite.Suite
	verifier          *chunks.ChunkVerifier
	systemOkVerifier  *chunks.ChunkVerifier
	systemBadVerifier *chunks.ChunkVerifier
}

// Make sure variables are set properly
// SetupTest is executed prior to each individual test in this test suite
func (s *ChunkVerifierTestSuite) SetupSuite() {
	vm := new(vmMock)
	systemOkVm := new(vmSystemOkMock)
	systemBadVm := new(vmSystemBadMock)
	vmCtx := fvm.NewContext(fvm.WithChain(testChain.Chain()))

	// system chunk runs predefined system transaction, hence we can't distinguish
	// based on its content and we need separate VMs
	s.verifier = chunks.NewChunkVerifier(vm, vmCtx, zerolog.Nop())
	s.systemOkVerifier = chunks.NewChunkVerifier(systemOkVm, vmCtx, zerolog.Nop())
	s.systemBadVerifier = chunks.NewChunkVerifier(systemBadVm, vmCtx, zerolog.Nop())
}

// TestChunkVerifier invokes all the tests in this test suite
func TestChunkVerifier(t *testing.T) {
	suite.Run(t, new(ChunkVerifierTestSuite))
}

// TestHappyPath tests verification of the baseline verifiable chunk
func (s *ChunkVerifierTestSuite) TestHappyPath() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "", false)
	require.NotNil(s.T(), vch)

	spockSecret, err := s.verifier.Verify(vch)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), spockSecret)
}

// TestMissingRegisterTouchForUpdate tests verification given a chunkdatapack missing a register touch (update)
func (s *ChunkVerifierTestSuite) TestMissingRegisterTouchForUpdate() {
	unittest.SkipUnless(s.T(), unittest.TEST_DEPRECATED, "Check new partial ledger for missing keys")

	vch, _ := GetBaselineVerifiableChunk(s.T(), "", false)
	require.NotNil(s.T(), vch)

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

	vch, _ := GetBaselineVerifiableChunk(s.T(), "", false)
	require.NotNil(s.T(), vch)

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
	vch, chunkEvents := GetBaselineVerifiableChunk(s.T(), "wrongEndState", false)
	require.NotNil(s.T(), vch)

	id1 := flow.NewRegisterID("00", "")
	value1 := []byte{'F'}

	id2 := flow.NewRegisterID("05", "")
	value2 := []byte{'B'}

	ids := make([]flow.RegisterID, 0)
	values := make([]flow.RegisterValue, 0)
	ids = append(ids, id1, id2)
	values = append(values, value1, value2)

	require.Equal(s.T(), len(ids), len(values))

	entries := make(flow.RegisterEntries, len(ids))
	for i := range ids {
		entries = append(entries, flow.RegisterEntry{
			Key:   ids[i],
			Value: values[i],
		})
	}

	keys, regValues := executionState.RegisterEntriesToKeysValues(entries)

	update, err := ledger.NewUpdate(
		ledger.State(vch.ChunkDataPack.StartState),
		keys,
		regValues,
	)
	assert.NoError(s.T(), err)

	updateExecutionData(s.T(), vch, vch.ChunkDataPack.Collection, chunkEvents, update, vch.Result.BlockID)

	spockSecret, err := s.verifier.Verify(vch)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFNonMatchingFinalState{}, err)
	assert.Nil(s.T(), spockSecret)
}

// TestFailedTx tests verification behavior in case
// of failed transaction. if a transaction fails, it should
// still change the state commitment.
func (s *ChunkVerifierTestSuite) TestFailedTx() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "failedTx", false)
	require.NotNil(s.T(), vch)

	spockSecret, err := s.verifier.Verify(vch)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), spockSecret)
}

// TestEventsMismatch tests verification behavior in case
// of emitted events not matching chunks
func (s *ChunkVerifierTestSuite) TestEventsMismatch() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "eventsMismatch", false)
	require.NotNil(s.T(), vch)

	_, err := s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFInvalidEventsCollection{}, err)
}

// TestServiceEventsMismatch tests verification behavior in case
// of emitted service events not matching chunks'
func (s *ChunkVerifierTestSuite) TestServiceEventsMismatch() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "doesn't matter", true)
	require.NotNil(s.T(), vch)

	_, err := s.systemBadVerifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFInvalidServiceEventsEmitted{}, err)
}

// TestServiceEventsAreChecked ensures that service events are in fact checked
func (s *ChunkVerifierTestSuite) TestServiceEventsAreChecked() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "doesn't matter", true)
	require.NotNil(s.T(), vch)

	_, err := s.systemOkVerifier.Verify(vch)
	assert.NoError(s.T(), err)
}

// TestSystemChunkWithCollectionFails ensures verification fails for system chunks with collections
func (s *ChunkVerifierTestSuite) TestSystemChunkWithCollectionFails() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "doesn't matter", true)
	require.NotNil(s.T(), vch)

	collection := unittest.CollectionFixture(1)
	vch.ChunkDataPack.Collection = &collection

	_, err := s.systemBadVerifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFSystemChunkIncludedCollection{}, err)
}

// TestEmptyCollection tests verification behaviour if a
// collection doesn't have any transaction.
func (s *ChunkVerifierTestSuite) TestEmptyCollection() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "", false)
	require.NotNil(s.T(), vch)

	col := unittest.CollectionFixture(0)
	vch.ChunkDataPack.Collection = &col
	vch.EndState = vch.ChunkDataPack.StartState

	emptyListHash, err := flow.EventsMerkleRootHash(flow.EventsList{})
	require.NoError(s.T(), err)
	vch.Chunk.EventCollection = emptyListHash // empty collection emits no events

	update, err := ledger.NewEmptyUpdate(ledger.State(vch.ChunkDataPack.StartState))
	require.NoError(s.T(), err)
	updateExecutionData(s.T(), vch, &col, nil, update, vch.Result.BlockID)

	spockSecret, err := s.verifier.Verify(vch)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), spockSecret)
}

func (s *ChunkVerifierTestSuite) TestExecutionDataBlockMismatch() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "", false)
	require.NotNil(s.T(), vch)

	col := unittest.CollectionFixture(0)
	vch.ChunkDataPack.Collection = &col
	vch.EndState = vch.ChunkDataPack.StartState

	emptyListHash, err := flow.EventsMerkleRootHash(flow.EventsList{})
	require.NoError(s.T(), err)
	vch.Chunk.EventCollection = emptyListHash //empty collection emits no events

	update, err := ledger.NewEmptyUpdate(ledger.State(vch.ChunkDataPack.StartState))
	require.NoError(s.T(), err)

	wrongBlock := vch.Result.BlockID

	wrongBlock[5]++ // it wraps

	updateExecutionData(s.T(), vch, &col, flow.EventsList{}, update, wrongBlock)

	_, err = s.verifier.Verify(vch)
	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFExecutionDataBlockIDMismatch{}, err)
}

func (s *ChunkVerifierTestSuite) TestExecutionDataChunkIdsLengthDiffers() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "", false)
	require.NotNil(s.T(), vch)

	vch.ChunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs = append(vch.ChunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs, cid.Undef)

	_, err := s.verifier.Verify(vch)

	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFExecutionDataChunksLengthMismatch{}, err)
}

func (s *ChunkVerifierTestSuite) TestExecutionDataChunkIdMismatch() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "", false)
	require.NotNil(s.T(), vch)

	vch.ChunkDataPack.ExecutionDataRoot.ChunkExecutionDataIDs[0] = cid.Undef // substitute invalid CID

	_, err := s.verifier.Verify(vch)

	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFExecutionDataInvalidChunkCID{}, err)
}

func (s *ChunkVerifierTestSuite) TestExecutionDataIdMismatch() {
	vch, _ := GetBaselineVerifiableChunk(s.T(), "", false)
	require.NotNil(s.T(), vch)

	vch.Result.ExecutionDataID[5]++ //wraparounds

	_, err := s.verifier.Verify(vch)

	assert.Error(s.T(), err)
	assert.True(s.T(), chunksmodels.IsChunkFaultError(err))
	assert.IsType(s.T(), &chunksmodels.CFInvalidExecutionDataID{}, err)
}

// updateExecutionData the VerifiableChunkData with a new execution data containing the execution
// data for the given collection and events
func updateExecutionData(t *testing.T, vch *verification.VerifiableChunkData, collection *flow.Collection, chunkEvents flow.EventsList, update *ledger.Update, blockID flow.Identifier) {
	var trieUpdate *ledger.TrieUpdate

	if update.Size() > 0 {
		var err error
		trieUpdate, err = pathfinder.UpdateToTrieUpdate(update, partial.DefaultPathFinderVersion)
		require.NoError(t, err)
	}

	ced := execution_data.ChunkExecutionData{
		Collection: collection,
		Events:     chunkEvents,
		TrieUpdate: trieUpdate,
	}

	cedCID, err := executionDataCIDProvider.CalculateChunkExecutionDataID(context.Background(), ced)
	require.NoError(t, err)

	bedr := flow.BlockExecutionDataRoot{
		BlockID:               blockID,
		ChunkExecutionDataIDs: []cid.Cid{cedCID},
	}
	vch.ChunkDataPack.ExecutionDataRoot = bedr

	vch.Result.ExecutionDataID, err = executionDataCIDProvider.CalculateExecutionDataRootID(context.Background(), bedr)
	require.NoError(t, err)
}

// GetBaselineVerifiableChunk returns a verifiable chunk and sets the script
// of a transaction in the middle of the collection to some value to signal the
// mocked vm on what to return as tx exec outcome.
func GetBaselineVerifiableChunk(t *testing.T, script string, system bool) (*verification.VerifiableChunkData, flow.EventsList) {

	// Collection setup

	collectionSize := 5
	magicTxIndex := 3
	coll := unittest.CollectionFixture(collectionSize)
	coll.Transactions[magicTxIndex] = &flow.TransactionBody{Script: []byte(script)}

	guarantee := coll.Guarantee()

	// Block setup
	payload := flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}
	header := unittest.BlockHeaderFixture()
	header.PayloadHash = payload.Hash()
	block := flow.Block{
		Header:  header,
		Payload: &payload,
	}
	blockID := block.ID()

	// registerTouch and State setup
	id1 := flow.NewRegisterID("00", "")
	value1 := []byte{'a'}

	id2Bytes := make([]byte, 32)
	id2Bytes[0] = byte(5)
	id2 := flow.NewRegisterID("05", "")
	value2 := []byte{'b'}
	UpdatedValue2 := []byte{'B'}

	entries := flow.RegisterEntries{
		{
			Key:   id1,
			Value: value1,
		},
		{
			Key:   id2,
			Value: value2,
		},
	}

	var verifiableChunkData verification.VerifiableChunkData

	metricsCollector := &metrics.NoopCollector{}

	f, _ := completeLedger.NewLedger(&fixtures.NoopWAL{}, 1000, metricsCollector, zerolog.Nop(), completeLedger.DefaultPathFinderVersion)

	compactor := fixtures.NewNoopCompactor(f)
	<-compactor.Ready()

	defer func() {
		<-f.Done()
		<-compactor.Done()
	}()

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
			Value: UpdatedValue2,
		},
	}

	keys, values = executionState.RegisterEntriesToKeysValues(entries)
	update, err = ledger.NewUpdate(startState, keys, values)
	require.NoError(t, err)

	endState, _, err := f.Set(update)
	require.NoError(t, err)

	// events
	var chunkEvents flow.EventsList

	erServiceEvents := make([]flow.ServiceEvent, 0)

	if system {
		chunkEvents = nil
		erServiceEvents = serviceEventsList

		// the system chunk's data pack does not include the collection, but the execution data does.
		// we must include the correct collection in the execution data, otherwise verification will fail.
		txBody, err := blueprints.SystemChunkTransaction(testChain.Chain())
		require.NoError(t, err)

		coll = flow.Collection{
			Transactions: []*flow.TransactionBody{txBody},
		}
	} else {
		for i := 0; i < collectionSize; i++ {
			if i == magicTxIndex {
				switch script {
				case "failedTx":
					continue
				}
			}
			chunkEvents = append(chunkEvents, eventsList...)
		}
	}

	EventsMerkleRootHash, err := flow.EventsMerkleRootHash(chunkEvents)
	require.NoError(t, err)

	trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, partial.DefaultPathFinderVersion)
	require.NoError(t, err)

	chunkExecutionData := execution_data.ChunkExecutionData{
		Collection: &coll,
		Events:     chunkEvents,
		TrieUpdate: trieUpdate,
	}
	chunkCid, err := executionDataCIDProvider.CalculateChunkExecutionDataID(context.Background(), chunkExecutionData)
	require.NoError(t, err)

	executionDataRoot := flow.BlockExecutionDataRoot{
		BlockID:               blockID,
		ChunkExecutionDataIDs: []cid.Cid{chunkCid},
	}

	executionDataID, err := executionDataCIDProvider.CalculateExecutionDataRootID(context.Background(), executionDataRoot)
	require.NoError(t, err)

	// Chunk setup
	chunk := flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: 0,
			StartState:      flow.StateCommitment(startState),
			BlockID:         blockID,
			EventCollection: EventsMerkleRootHash,
		},
		Index: 0,
	}

	chunkDataPack := flow.ChunkDataPack{
		ChunkID:           chunk.ID(),
		StartState:        flow.StateCommitment(startState),
		Proof:             proof,
		Collection:        &coll,
		ExecutionDataRoot: executionDataRoot,
	}

	// ENs don't include the system collection in the data pack.
	if system {
		chunkDataPack.Collection = nil
	}

	// ExecutionResult setup
	result := flow.ExecutionResult{
		BlockID:         blockID,
		Chunks:          flow.ChunkList{&chunk},
		ServiceEvents:   erServiceEvents,
		ExecutionDataID: executionDataID,
	}

	verifiableChunkData = verification.VerifiableChunkData{
		IsSystemChunk: system,
		Chunk:         &chunk,
		Header:        header,
		Result:        &result,
		ChunkDataPack: &chunkDataPack,
		EndState:      flow.StateCommitment(endState),
	}

	return &verifiableChunkData, chunkEvents
}

type vmMock struct{}

func (vm *vmMock) NewExecutor(
	ctx fvm.Context,
	proc fvm.Procedure,
	txn storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	panic("not implemented")
}

func (vm *vmMock) Run(
	ctx fvm.Context,
	proc fvm.Procedure,
	storage snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	tx, ok := proc.(*fvm.TransactionProcedure)
	if !ok {
		return nil, fvm.ProcedureOutput{}, fmt.Errorf(
			"invokable is not a transaction")
	}

	snapshot := &snapshot.ExecutionSnapshot{}
	output := fvm.ProcedureOutput{}

	id0 := flow.NewRegisterID("00", "")
	id5 := flow.NewRegisterID("05", "")

	switch string(tx.Transaction.Script) {
	case "wrongEndState":
		snapshot.WriteSet = map[flow.RegisterID]flow.RegisterValue{
			id0: []byte{'F'},
		}
		output.Logs = []string{"log1", "log2"}
		output.Events = eventsList
	case "failedTx":
		snapshot.WriteSet = map[flow.RegisterID]flow.RegisterValue{
			id5: []byte{'B'},
		}
		output.Err = fvmErrors.NewCadenceRuntimeError(runtime.Error{}) // inside the runtime (e.g. div by zero, access account)
	case "eventsMismatch":
		output.Events = append(eventsList, flow.Event{
			Type:             "event.Extra",
			TransactionID:    flow.Identifier{2, 3},
			TransactionIndex: 0,
			EventIndex:       0,
			Payload:          []byte{88},
		})
	default:
		snapshot.ReadSet = map[flow.RegisterID]struct{}{
			id0: struct{}{},
			id5: struct{}{},
		}
		snapshot.WriteSet = map[flow.RegisterID]flow.RegisterValue{
			id5: []byte{'B'},
		}
		output.Logs = []string{"log1", "log2"}
		output.Events = eventsList
	}

	return snapshot, output, nil
}

func (vmMock) GetAccount(
	_ fvm.Context,
	_ flow.Address,
	_ snapshot.StorageSnapshot,
) (
	*flow.Account,
	error) {
	panic("not expected")
}

type vmSystemOkMock struct{}

func (vm *vmSystemOkMock) NewExecutor(
	ctx fvm.Context,
	proc fvm.Procedure,
	txn storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	panic("not implemented")
}

func (vm *vmSystemOkMock) Run(
	ctx fvm.Context,
	proc fvm.Procedure,
	storage snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	_, ok := proc.(*fvm.TransactionProcedure)
	if !ok {
		return nil, fvm.ProcedureOutput{}, fmt.Errorf(
			"invokable is not a transaction")
	}

	id0 := flow.NewRegisterID("00", "")
	id5 := flow.NewRegisterID("05", "")

	// add "default" interaction expected in tests
	snapshot := &snapshot.ExecutionSnapshot{
		ReadSet: map[flow.RegisterID]struct{}{
			id0: struct{}{},
			id5: struct{}{},
		},
		WriteSet: map[flow.RegisterID]flow.RegisterValue{
			id5: []byte{'B'},
		},
	}
	output := fvm.ProcedureOutput{
		ConvertedServiceEvents: flow.ServiceEventList{*epochSetupServiceEvent},
		Logs:                   []string{"log1", "log2"},
	}

	return snapshot, output, nil
}

func (vmSystemOkMock) GetAccount(
	_ fvm.Context,
	_ flow.Address,
	_ snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	panic("not expected")
}

type vmSystemBadMock struct{}

func (vm *vmSystemBadMock) NewExecutor(
	ctx fvm.Context,
	proc fvm.Procedure,
	txn storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	panic("not implemented")
}

func (vm *vmSystemBadMock) Run(
	ctx fvm.Context,
	proc fvm.Procedure,
	storage snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	_, ok := proc.(*fvm.TransactionProcedure)
	if !ok {
		return nil, fvm.ProcedureOutput{}, fmt.Errorf(
			"invokable is not a transaction")
	}

	// EpochSetup event is expected, but we emit EpochCommit here resulting in
	// a chunk fault
	output := fvm.ProcedureOutput{
		ConvertedServiceEvents: flow.ServiceEventList{*epochCommitServiceEvent},
	}

	return &snapshot.ExecutionSnapshot{}, output, nil
}

func (vmSystemBadMock) GetAccount(
	_ fvm.Context,
	_ flow.Address,
	_ snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	panic("not expected")
}
