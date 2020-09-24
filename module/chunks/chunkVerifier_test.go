package chunks_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/verification"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	chunksmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/ledger"
	"github.com/onflow/flow-go/utils/unittest"
)

type ChunkVerifierTestSuite struct {
	suite.Suite
	verifier *chunks.ChunkVerifier
}

// Make sure variables are set properly
// SetupTest is executed prior to each individual test in this test suite
func (s *ChunkVerifierTestSuite) SetupTest() {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	vm := new(vmMock)
	vmCtx := fvm.NewContext()

	s.verifier = chunks.NewChunkVerifier(vm, vmCtx)
}

// TestChunkVerifier invokes all the tests in this test suite
func TestChunkVerifier(t *testing.T) {
	suite.Run(t, new(ChunkVerifierTestSuite))
}

// TestHappyPath tests verification of the baseline verifiable chunk
func (s *ChunkVerifierTestSuite) TestHappyPath() {
	vch := GetBaselineVerifiableChunk(s.T(), []byte{})
	assert.NotNil(s.T(), vch)
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.Nil(s.T(), chFaults)
	assert.NotNil(s.T(), spockSecret)
}

// TestMissingRegisterTouchForUpdate tests verification given a chunkdatapack missing a register touch (update)
func (s *ChunkVerifierTestSuite) TestMissingRegisterTouchForUpdate() {
	vch := GetBaselineVerifiableChunk(s.T(), []byte(""))
	assert.NotNil(s.T(), vch)
	// remove the second register touch
	vch.ChunkDataPack.RegisterTouches = vch.ChunkDataPack.RegisterTouches[:1]
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), chFaults)
	assert.Nil(s.T(), spockSecret)
	_, ok := chFaults.(*chunksmodels.CFMissingRegisterTouch)
	assert.True(s.T(), ok)
}

// TestMissingRegisterTouchForRead tests verification given a chunkdatapack missing a register touch (read)
func (s *ChunkVerifierTestSuite) TestMissingRegisterTouchForRead() {
	vch := GetBaselineVerifiableChunk(s.T(), []byte(""))
	assert.NotNil(s.T(), vch)
	// remove the second register touch
	vch.ChunkDataPack.RegisterTouches = vch.ChunkDataPack.RegisterTouches[1:]
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
	vch := GetBaselineVerifiableChunk(s.T(), []byte("wrongEndState"))
	assert.NotNil(s.T(), vch)
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.NotNil(s.T(), chFaults)
	assert.Nil(s.T(), spockSecret)
	_, ok := chFaults.(*chunksmodels.CFNonMatchingFinalState)
	assert.True(s.T(), ok)
}

// TestFailedTx tests verification behaviour in case
// of failed transaction. if a transaction fails, it shouldn't
// change the state commitment.
func (s *ChunkVerifierTestSuite) TestFailedTx() {
	vch := GetBaselineVerifiableChunk(s.T(), []byte("failedTx"))
	assert.NotNil(s.T(), vch)
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.Nil(s.T(), chFaults)
	assert.NotNil(s.T(), spockSecret)
}

// TestVerifyWrongChunkType evaluates that following invocations return an error:
// - verifying a system chunk with Verify method.
// - verifying a non-system chunk with SystemChunkVerify method.
func (s *ChunkVerifierTestSuite) TestVerifyWrongChunkType() {
	// defines verifiable chunk for a system chunk
	svc := &verification.VerifiableChunkData{
		IsSystemChunk: true,
	}
	// invoking Verify method with system chunk should return an error
	_, _, err := s.verifier.Verify(svc)
	require.Error(s.T(), err)

	// defines verifiable chunk for a non-system chunk
	vc := &verification.VerifiableChunkData{
		IsSystemChunk: false,
	}
	// invoking SystemChunkVerify method with a non-system chunk should return an error
	_, _, err = s.verifier.SystemChunkVerify(vc)
	require.Error(s.T(), err)
}

// TestEmptyCollection tests verification behaviour if a
// collection doesn't have any transaction.
func (s *ChunkVerifierTestSuite) TestEmptyCollection() {
	vch := GetBaselineVerifiableChunk(s.T(), []byte{})
	assert.NotNil(s.T(), vch)
	col := unittest.CollectionFixture(0)
	vch.Collection = &col
	vch.EndState = vch.ChunkDataPack.StartState
	spockSecret, chFaults, err := s.verifier.Verify(vch)
	assert.Nil(s.T(), err)
	assert.Nil(s.T(), chFaults)
	assert.NotNil(s.T(), spockSecret)
}

// GetBaselineVerifiableChunk returns a verifiable chunk and sets the script
// of a transaction in the middle of the collection to some value to signal the
// mocked vm on what to return as tx exec outcome.
func GetBaselineVerifiableChunk(t *testing.T, script []byte) *verification.VerifiableChunkData {
	// Collection setup

	coll := unittest.CollectionFixture(5)
	coll.Transactions[3] = &flow.TransactionBody{Script: script}

	guarantee := coll.Guarantee()

	// Block setup
	payload := flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}
	header := unittest.BlockHeaderFixture()
	header.PayloadHash = payload.Hash()
	block := flow.Block{
		Header:  &header,
		Payload: &payload,
	}

	// registerTouch and State setup
	id1 := state.RegisterID(string(make([]byte, 32)), "", "")
	value1 := []byte{'a'}

	id2Bytes := make([]byte, 32)
	id2Bytes[0] = byte(5)
	id2 := state.RegisterID(string(id2Bytes), "", "")
	value2 := []byte{'b'}
	UpdatedValue2 := []byte{'B'}

	ids := make([][]byte, 0)
	values := make([][]byte, 0)
	ids = append(ids, id1, id2)
	values = append(values, value1, value2)

	var verifiableChunkData verification.VerifiableChunkData

	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dbDir string) {
		f, _ := ledger.NewMTrieStorage(dbDir, 1000, metricsCollector, nil)
		startState, _ := f.UpdateRegisters(ids, values, f.EmptyStateCommitment())
		regTs, _ := f.GetRegisterTouches(ids, startState)

		ids = [][]byte{id2}
		values = [][]byte{UpdatedValue2}
		endState, _ := f.UpdateRegisters(ids, values, startState)

		// Chunk setup
		chunk := flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: 0,
				StartState:      startState,
			},
			Index: 0,
		}

		chunkDataPack := flow.ChunkDataPack{
			ChunkID:         chunk.ID(),
			StartState:      startState,
			RegisterTouches: regTs,
		}

		// ExecutionResult setup
		result := flow.ExecutionResult{
			ExecutionResultBody: flow.ExecutionResultBody{
				BlockID: block.ID(),
				Chunks:  flow.ChunkList{&chunk},
			},
		}

		verifiableChunkData = verification.VerifiableChunkData{
			IsSystemChunk: false,
			Chunk:         &chunk,
			Header:        &header,
			Result:        &result,
			Collection:    &coll,
			ChunkDataPack: &chunkDataPack,
			EndState:      endState,
		}
	})

	return &verifiableChunkData
}

type vmMock struct{}

func (vm *vmMock) Run(ctx fvm.Context, proc fvm.Procedure, ledger state.Ledger) error {
	tx, ok := proc.(*fvm.TransactionProcedure)
	if !ok {
		return fmt.Errorf("invokable is not a transaction")
	}

	switch string(tx.Transaction.Script) {
	case "wrongEndState":
		id1 := string(make([]byte, 32))
		UpdatedValue1 := []byte{'F'}
		// add updates to the ledger
		ledger.Set(id1, "", "", UpdatedValue1)

		tx.Logs = []string{"log1", "log2"}
	case "failedTx":
		id1 := string(make([]byte, 32))
		UpdatedValue1 := []byte{'F'}
		// add updates to the ledger
		ledger.Set(id1, "", "", UpdatedValue1)

		tx.Err = &fvm.MissingPayerError{} // inside the runtime (e.g. div by zero, access account)
	default:
		id1 := string(make([]byte, 32))
		id2 := make([]byte, 32)
		id2[0] = byte(5)
		UpdatedValue2 := []byte{'B'}
		_, _ = ledger.Get(id1, "", "")
		ledger.Set(string(id2), "", "", UpdatedValue2)

		tx.Logs = []string{"log1", "log2"}
	}

	return nil
}
