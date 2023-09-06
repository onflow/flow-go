package chunks_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	chunksmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/chunks"
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
	vch := GetBaselineVerifiableChunk(s.T(), "", false)
	assert.NotNil(s.T(), vch)
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.Nil(s.T(), chFaults)
	assert.NotNil(s.T(), spockSecret)
}

// TestMissingRegisterTouchForUpdate tests verification given a chunkdatapack missing a register touch (update)
func (s *ChunkVerifierTestSuite) TestMissingRegisterTouchForUpdate() {
	unittest.SkipUnless(s.T(), unittest.TEST_DEPRECATED, "Check new partial ledger for missing keys")

	vch := GetBaselineVerifiableChunk(s.T(), "", false)
	assert.NotNil(s.T(), vch)
	// remove the second register touch
	// vch.ChunkDataPack.RegisterTouches = vch.ChunkDataPack.RegisterTouches[:1]
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), chFaults)
	assert.Nil(s.T(), spockSecret)
	_, ok := chFaults.(*chunksmodels.CFMissingRegisterTouch)
	assert.True(s.T(), ok)
}

// TestMissingRegisterTouchForRead tests verification given a chunkdatapack missing a register touch (read)
func (s *ChunkVerifierTestSuite) TestMissingRegisterTouchForRead() {
	unittest.SkipUnless(s.T(), unittest.TEST_DEPRECATED, "Check new partial ledger for missing keys")

	vch := GetBaselineVerifiableChunk(s.T(), "", false)
	assert.NotNil(s.T(), vch)
	// remove the second register touch
	// vch.ChunkDataPack.RegisterTouches = vch.ChunkDataPack.RegisterTouches[1:]
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), chFaults)
	assert.Nil(s.T(), spockSecret)
	_, ok := chFaults.(*chunksmodels.CFMissingRegisterTouch)
	assert.True(s.T(), ok)
}

// TestWrongEndState tests verification covering the case
// the state commitment computed after updating the partial trie
// doesn't match the one provided by the chunks
func (s *ChunkVerifierTestSuite) TestWrongEndState() {
	vch := GetBaselineVerifiableChunk(s.T(), "wrongEndState", false)
	assert.NotNil(s.T(), vch)
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), chFaults)
	assert.Nil(s.T(), spockSecret)
	_, ok := chFaults.(*chunksmodels.CFNonMatchingFinalState)
	assert.True(s.T(), ok)
}

// TestFailedTx tests verification behavior in case
// of failed transaction. if a transaction fails, it should
// still change the state commitment.
func (s *ChunkVerifierTestSuite) TestFailedTx() {
	vch := GetBaselineVerifiableChunk(s.T(), "failedTx", false)
	assert.NotNil(s.T(), vch)
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.Nil(s.T(), chFaults)
	assert.NotNil(s.T(), spockSecret)
}

// TestEventsMismatch tests verification behavior in case
// of emitted events not matching chunks
func (s *ChunkVerifierTestSuite) TestEventsMismatch() {
	vch := GetBaselineVerifiableChunk(s.T(), "eventsMismatch", false)
	assert.NotNil(s.T(), vch)
	_, chFault, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), chFault)
	assert.IsType(s.T(), &chunksmodels.CFInvalidEventsCollection{}, chFault)
}

// TestServiceEventsMismatch tests verification behavior in case
// of emitted service events not matching chunks'
func (s *ChunkVerifierTestSuite) TestServiceEventsMismatch() {
	vch := GetBaselineVerifiableChunk(s.T(), "doesn't matter", true)
	assert.NotNil(s.T(), vch)
	_, chFault, err := s.systemBadVerifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), chFault)
	assert.IsType(s.T(), &chunksmodels.CFInvalidServiceEventsEmitted{}, chFault)
}

// TestServiceEventsAreChecked ensures that service events are in fact checked
func (s *ChunkVerifierTestSuite) TestServiceEventsAreChecked() {
	vch := GetBaselineVerifiableChunk(s.T(), "doesn't matter", true)
	assert.NotNil(s.T(), vch)
	_, chFault, err := s.systemOkVerifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.Nil(s.T(), chFault)
}

// TestEmptyCollection tests verification behaviour if a
// collection doesn't have any transaction.
func (s *ChunkVerifierTestSuite) TestEmptyCollection() {
	vch := GetBaselineVerifiableChunk(s.T(), "", false)
	assert.NotNil(s.T(), vch)
	col := unittest.CollectionFixture(0)
	vch.ChunkDataPack.Collection = &col
	vch.EndState = vch.ChunkDataPack.StartState
	emptyListHash, err := flow.EventsMerkleRootHash(flow.EventsList{})
	assert.NoError(s.T(), err)
	vch.Chunk.EventCollection = emptyListHash // empty collection emits no events
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.Nil(s.T(), chFaults)
	assert.NotNil(s.T(), spockSecret)
}

// GetBaselineVerifiableChunk returns a verifiable chunk and sets the script
// of a transaction in the middle of the collection to some value to signal the
// mocked vm on what to return as tx exec outcome.
func GetBaselineVerifiableChunk(t *testing.T, script string, system bool) *verification.VerifiableChunkData {

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
	chunkEvents := make(flow.EventsList, 0)

	erServiceEvents := make([]flow.ServiceEvent, 0)

	if system {
		chunkEvents = flow.EventsList{}
		erServiceEvents = serviceEventsList
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
		ChunkID:    chunk.ID(),
		StartState: flow.StateCommitment(startState),
		Proof:      proof,
		Collection: &coll,
	}

	// ExecutionResult setup
	result := flow.ExecutionResult{
		BlockID:       blockID,
		Chunks:        flow.ChunkList{&chunk},
		ServiceEvents: erServiceEvents,
	}

	verifiableChunkData = verification.VerifiableChunkData{
		IsSystemChunk: system,
		Chunk:         &chunk,
		Header:        header,
		Result:        &result,
		ChunkDataPack: &chunkDataPack,
		EndState:      flow.StateCommitment(endState),
	}

	return &verifiableChunkData
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
