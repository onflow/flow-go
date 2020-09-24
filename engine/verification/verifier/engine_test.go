package verifier_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/testutil/mocklocal"
	"github.com/onflow/flow-go/engine/verification"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/engine/verification/verifier"
	chmodel "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	realModule "github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	network "github.com/onflow/flow-go/network/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type VerifierEngineTestSuite struct {
	suite.Suite
	net     *mockmodule.Network
	tracer  realModule.Tracer
	state   *protocol.State
	ss      *protocol.Snapshot
	me      *mocklocal.MockLocal
	sk      crypto.PrivateKey
	hasher  hash.Hasher
	chain   flow.Chain
	con     *network.Conduit                // mocks con for submitting result approvals
	metrics *mockmodule.VerificationMetrics // mocks performance monitoring metrics
}

func TestVerifierEngine(t *testing.T) {
	suite.Run(t, new(VerifierEngineTestSuite))
}

func (suite *VerifierEngineTestSuite) SetupTest() {

	suite.state = &protocol.State{}
	suite.net = &mockmodule.Network{}
	suite.tracer = trace.NewNoopTracer()
	suite.ss = &protocol.Snapshot{}
	suite.con = &network.Conduit{}
	suite.metrics = &mockmodule.VerificationMetrics{}
	suite.chain = flow.Testnet.Chain()

	suite.net.On("Register", engine.PushApprovals, testifymock.Anything).
		Return(suite.con, nil).
		Once()

	suite.state.On("Final").Return(suite.ss)

	// Mocks the signature oracle of the engine
	//
	// generates signing and verification keys
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(suite.T(), n, crypto.KeyGenSeedMinLenBLSBLS12381)
	require.NoError(suite.T(), err)

	// creates private key of verification node
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(suite.T(), err)
	suite.sk = sk

	// tag of hasher should be the same as the tag of engine's hasher
	suite.hasher = utils.NewResultApprovalHasher()

	// defines the identity of verification node and attaches its key.
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	verIdentity.StakingPubKey = sk.PublicKey()
	suite.me = mocklocal.NewMockLocal(sk, verIdentity.NodeID, suite.T())
}

func (suite *VerifierEngineTestSuite) TestNewEngine() *verifier.Engine {
	e, err := verifier.New(zerolog.Logger{}, suite.metrics, suite.tracer, suite.net, suite.state, suite.me, ChunkVerifierMock{})
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

	completeRA := utils.LightExecutionResultFixture(1)

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
	vChunk := unittest.VerifiableChunkDataFixture(uint64(0))

	// mocking node ID using the LocalMock
	suite.me.MockNodeID(myID)
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil)

	// mocks metrics
	// reception of verifiable chunk
	suite.metrics.On("OnVerifiableChunkReceived").Return()
	// emission of result approval
	suite.metrics.On("OnResultApproval").Return()

	suite.con.
		On("Publish", testifymock.Anything, testifymock.Anything).
		Return(nil).
		Run(func(args testifymock.Arguments) {
			// check that the approval matches the input execution result
			ra, ok := args[0].(*flow.ResultApproval)
			suite.Assert().True(ok)
			suite.Assert().Equal(vChunk.Result.ID(), ra.Body.ExecutionResultID)

			// verifies the signatures
			atstID := ra.Body.Attestation.ID()
			suite.Assert().True(suite.sk.PublicKey().Verify(ra.Body.AttestationSignature, atstID[:], suite.hasher))
			bodyID := ra.Body.ID()
			suite.Assert().True(suite.sk.PublicKey().Verify(ra.VerifierSignature, bodyID[:], suite.hasher))

			// spock should be non-nil
			suite.Assert().NotNil(ra.Body.Spock)
		}).
		Once()

	err := eng.Process(myID, vChunk)
	suite.Assert().NoError(err)
	suite.ss.AssertExpectations(suite.T())
	suite.con.AssertExpectations(suite.T())

}

func (suite *VerifierEngineTestSuite) TestVerifyUnhappyPaths() {
	eng := suite.TestNewEngine()
	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))

	// mocking node ID using the LocalMock
	suite.me.MockNodeID(myID)
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil)

	// mocks metrics
	// reception of verifiable chunk
	suite.metrics.On("OnVerifiableChunkReceived").Return()

	// we shouldn't receive any result approval
	suite.con.
		On("Publish", testifymock.Anything, testifymock.Anything).
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

func (v ChunkVerifierMock) Verify(vc *verification.VerifiableChunkData) ([]byte, chmodel.ChunkFault, error) {
	if vc.IsSystemChunk {
		return nil, nil, fmt.Errorf("wrong method invoked for verifying system chunk")
	}

	switch vc.Chunk.Index {
	case 0:
		return []byte{}, nil, nil
	// return error
	case 1:
		return nil, chmodel.NewCFMissingRegisterTouch(
			[]string{"test missing register touch"},
			vc.Chunk.Index,
			vc.Result.ID()), nil

	case 2:
		return nil, chmodel.NewCFInvalidVerifiableChunk(
			"test",
			errors.New("test invalid verifiable chunk"),
			vc.Chunk.Index,
			vc.Result.ID()), nil

	case 3:
		return nil, chmodel.NewCFNonMatchingFinalState(
			unittest.StateCommitmentFixture(),
			unittest.StateCommitmentFixture(),
			vc.Chunk.Index,
			vc.Result.ID()), nil

	// TODO add cases for challenges
	// return successful by default
	default:
		return nil, nil, nil
	}

}

func (v ChunkVerifierMock) SystemChunkVerify(vc *verification.VerifiableChunkData) ([]byte, chmodel.ChunkFault, error) {
	if !vc.IsSystemChunk {
		return nil, nil, fmt.Errorf("wrong method invoked for verifying non-system chunk")
	}
	return nil, nil, nil
}
