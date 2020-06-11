package verifier_test

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine"
	mocklocal "github.com/dapperlabs/flow-go/engine/testutil/mocklocal"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type VerifierEngineTestSuite struct {
	suite.Suite
	net     *mockmodule.Network
	state   *protocol.State
	ss      *protocol.Snapshot
	me      *mocklocal.MockLocal
	sk      crypto.PrivateKey
	hasher  hash.Hasher
	conduit *network.Conduit       // mocks conduit for submitting result approvals
	metrics *metrics.NoopCollector // mocks performance monitoring metrics
}

func TestVerifierEngine(t *testing.T) {
	suite.Run(t, new(VerifierEngineTestSuite))
}

func (suite *VerifierEngineTestSuite) SetupTest() {

	suite.state = &protocol.State{}
	suite.net = &mockmodule.Network{}
	suite.ss = &protocol.Snapshot{}
	suite.conduit = &network.Conduit{}
	suite.metrics = metrics.NewNoopCollector()

	suite.net.On("Register", uint8(engine.ApprovalProvider), testifymock.Anything).
		Return(suite.conduit, nil).
		Once()

	suite.state.On("Final").Return(suite.ss)

	// Mocks the signature oracle of the engine
	//
	// generates signing and verification keys
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(suite.T(), n, crypto.KeyGenSeedMinLenBLSBLS12381)
	require.NoError(suite.T(), err)
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(suite.T(), err)
	suite.sk = sk
	// tag of hasher should be the same as the tag of engine's hasher
	suite.hasher = utils.NewResultApprovalHasher()
	suite.me = mocklocal.NewMockLocal(sk, flow.Identifier{}, suite.T())
}

func (suite *VerifierEngineTestSuite) TestNewEngine() *verifier.Engine {
	e, err := verifier.New(zerolog.Logger{}, suite.metrics, suite.net, suite.state, suite.me, ChunkVerifierMock{})
	require.Nil(suite.T(), err)

	suite.net.AssertExpectations(suite.T())
	return e

}

func (suite *VerifierEngineTestSuite) TestInvalidSender() {
	eng := suite.TestNewEngine()

	myID := unittest.IdentifierFixture()
	invalidID := unittest.IdentifierFixture()

	// mocks NodeID method of the local
	suite.me.MockNodeID(myID)

	completeRA := utils.CompleteExecutionResultFixture(suite.T(), 1)

	err := eng.Process(invalidID, &completeRA)
	assert.Error(suite.T(), err)
}

func (suite *VerifierEngineTestSuite) TestIncorrectResult() {
	// TODO when ERs are verified
}

// TestVerifyHappyPath tests the verification path for a single verifiable chunk, which is
// assigned to the verifier node, and is passed by the ingest engine
// The tests evaluates that a result approval is emitted to all consensus nodes
// about the input execution receipt
func (suite *VerifierEngineTestSuite) TestVerifyHappyPath() {

	eng := suite.TestNewEngine()
	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	// creates a verifiable chunk
	vChunk := unittest.VerifiableChunkFixture(uint64(0))

	// mocking node ID using the LocalMock
	suite.me.MockNodeID(myID)
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil)

	suite.conduit.
		On("Submit", testifymock.Anything, consensusNodes[0].NodeID).
		Return(nil).
		Run(func(args testifymock.Arguments) {
			// check that the approval matches the input execution result
			ra, ok := args[0].(*flow.ResultApproval)
			suite.Assert().True(ok)
			suite.Assert().Equal(vChunk.Receipt.ExecutionResult.ID(), ra.Body.ExecutionResultID)

			// verifies the signatures
			atstID := ra.Body.Attestation.ID()
			suite.Assert().True(suite.sk.PublicKey().Verify(ra.Body.AttestationSignature, atstID[:], suite.hasher))
			bodyID := ra.Body.ID()
			suite.Assert().True(suite.sk.PublicKey().Verify(ra.VerifierSignature, bodyID[:], suite.hasher))
		}).
		Once()

	err := eng.Process(myID, vChunk)
	suite.Assert().NoError(err)
	suite.ss.AssertExpectations(suite.T())
	suite.conduit.AssertExpectations(suite.T())

}

func (suite *VerifierEngineTestSuite) TestVerifyUnhappyPaths() {
	eng := suite.TestNewEngine()
	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))

	// mocking node ID using the LocalMock
	suite.me.MockNodeID(myID)
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil)

	// we shouldn't receive any result approval
	suite.conduit.
		On("Submit", testifymock.Anything, consensusNodes[0].NodeID).
		Return(nil).
		Run(func(args testifymock.Arguments) {
			// TODO change this to check challeneges
			_, ok := args[0].(*flow.ResultApproval)
			suite.Assert().False(ok)
		})

	var tests = []struct {
		vc          *verification.VerifiableChunkData
		expectedErr error
	}{
		{unittest.VerifiableChunkDataFixture(uint64(1)), nil},
		{unittest.VerifiableChunkDataFixture(uint64(2)), nil},
		{unittest.VerifiableChunkDataFixture(uint64(3)), nil},
	}
	for _, test := range tests {
		err := eng.Process(myID, test.vc)
		suite.Assert().NoError(err)
	}
}

type ChunkVerifierMock struct {
}

func (v ChunkVerifierMock) Verify(vc *verification.VerifiableChunkData) (chmodel.ChunkFault, error) {
	switch vc.Chunk.Index {
	case 0:
		return nil, nil
	// return error
	case 1:
		return chmodel.NewCFMissingRegisterTouch(
			[]string{"test missing register touch"},
			vc.Chunk.Index,
			vc.Result.ID()), nil

	case 2:
		return chmodel.NewCFInvalidVerifiableChunk(
			"test",
			errors.New("test invalid verifiable chunk"),
			vc.Chunk.Index,
			vc.Result.ID()), nil

	case 3:
		return chmodel.NewCFNonMatchingFinalState(
			unittest.StateCommitmentFixture(),
			unittest.StateCommitmentFixture(),
			vc.Chunk.Index,
			vc.Result.ID()), nil

	// TODO add cases for challenges
	// return successful by default
	default:
		return nil, nil
	}

}
