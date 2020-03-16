package chunkVerifier

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type ChunkVerifierTestSuite struct {
	suite.Suite
	verifier ChunkVerifier
}

// Make sure variables are set properly
func (s *ChunkVerifierTestSuite) SetupTest() {
	s.verifier = NewFlowChunkVerifier()
}

// TestVerification invokes all the tests in this test suite
func TestVerification(t *testing.T) {
	suite.Run(t, new(ChunkVerifierTestSuite))
}

// TestHappyPath tests verification of the baseline verifiable chunk
func (s *ChunkVerifierTestSuite) TestHappyPath() {
	vch := GetBaselineVerifiableChunk()
	err := s.verifier.Verify(vch)
	assert.NotNil(s.T(), err)
}

// TestMissingRegisterTouch tests verification of the a chunkdatapack missing a register touch
func (s *ChunkVerifierTestSuite) TestMissingRegisterTouch() {
	vch := GetBaselineVerifiableChunk()
	// remove the first register touch
	vch.ChunkDataPack.RegisterTouches = vch.ChunkDataPack.RegisterTouches[1:]
	err := s.verifier.Verify(vch)
	// TODO RAMTIN
	assert.NotNil(s.T(), err)
}

func GetBaselineVerifiableChunk() *verification.VerifiableChunk {
	// Collection setup
	coll := unittest.CollectionFixture(3)
	guarantee := coll.Guarantee()

	// Block setup
	payload := flow.Payload{
		Identities: unittest.IdentityListFixture(32),
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}
	header := unittest.BlockHeaderFixture()
	header.PayloadHash = payload.Hash()
	block := flow.Block{
		Header:  header,
		Payload: payload,
	}

	// registerTouch and State setup
	id1 := make([]byte, 32)
	value1 := []byte{'a'}

	id2 := make([]byte, 32)
	value2 := []byte{'b'}
	UpdatedValue2 := []byte{'B'}
	SetBit(id2, 5)

	ids := make([][]byte, 0)
	values := make([][]byte, 0)
	ids = append(ids, id1, id2)
	values = append(values, value1, value2)

	db, _ := TempLevelDB()
	f, _ := ledger.NewTrieStorage(db)
	startState, _ := f.UpdateRegisters(ids, values)
	regTs, _ := f.GetRegisterTouches(ids, startState)

	ids = [][]byte{id1}
	values = [][]byte{UpdatedValue2}
	endState, _ := f.UpdateRegisters(ids, values)

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

	receipt := flow.ExecutionReceipt{
		ExecutionResult: result,
	}

	return &verification.VerifiableChunk{
		ChunkIndex:    chunk.Index,
		EndState:      endState,
		Block:         &block,
		Receipt:       &receipt,
		Collection:    &coll,
		ChunkDataPack: &chunkDataPack,
	}

}

// ChunkMissingRegTouch *verification.VerifiableChunk
// ChunkWrongEndState   *verification.VerifiableChunk
// TooManyInputs

func SetBit(b []byte, i int) {
	b[i/8] |= 1 << int(7-i%8)
}

func TempLevelDB() (*leveldb.LevelDB, error) {
	dir, err := ioutil.TempDir("", "flow-test-db")
	if err != nil {
		return nil, err
	}
	kvdbPath := filepath.Join(dir, "kvdb")
	tdbPath := filepath.Join(dir, "tdb")
	db, err := leveldb.NewLevelDB(kvdbPath, tdbPath)
	return db, err
}
