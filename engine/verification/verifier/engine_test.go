package verifier_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TODO add challeneges
// if ChunkIndex == 0 : ok
// if ChunkIndex == 1 : error
// if ChunkIndex == 2 : challenge
type ChunkVerifierMock struct {
}

func (v ChunkVerifierMock) Verify(ch *verification.VerifiableChunk) error {
	return nil
}

type VerifierEngineTestSuite struct {
	suite.Suite
	net     *mockmodule.Network
	state   *protocol.State
	ss      *protocol.Snapshot
	me      *MockLocal
	sk      crypto.PrivateKey
	hasher  crypto.Hasher
	conduit *network.Conduit // mocks conduit for submitting result approvals
}

func TestVerifierEngine(t *testing.T) {
	suite.Run(t, new(VerifierEngineTestSuite))
}

func (suite *VerifierEngineTestSuite) SetupTest() {
	suite.state = &protocol.State{}
	suite.net = &mockmodule.Network{}
	suite.ss = &protocol.Snapshot{}
	suite.conduit = &network.Conduit{}

	suite.net.On("Register", uint8(engine.ApprovalProvider), testifymock.Anything).
		Return(suite.conduit, nil).
		Once()

	suite.state.On("Final").Return(suite.ss)

	// Mocks the signature oracle of the engine
	//
	// generates signing and verification keys
	seed := []byte{1, 2, 3, 4}
	sk, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, seed)
	require.NoError(suite.T(), err)
	suite.sk = sk
	// tag of hasher should be the same as the tag of engine's hasher
	suite.hasher = utils.NewResultApprovalHasher()
	suite.me = NewMockLocal(sk, flow.Identifier{}, suite.T())
}

func (suite *VerifierEngineTestSuite) TestNewEngine() *verifier.Engine {
	e, err := verifier.New(zerolog.Logger{}, suite.net, suite.state, suite.me, ChunkVerifierMock{})
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

	completeRA := unittest.CompleteExecutionResultFixture(1)

	err := eng.Process(invalidID, &completeRA)
	assert.Error(suite.T(), err)
}

func (suite *VerifierEngineTestSuite) TestIncorrectResult() {
	// TODO when ERs are verified
}

// TestVerify tests the verification path for a single verifiable chunk, which is
// assigned to the verifier node, and is passed by the ingest engine
// The tests evaluates that a result approval is emitted to all consensus nodes
// about the input execution receipt
func (suite *VerifierEngineTestSuite) TestVerify() {

	eng := suite.TestNewEngine()
	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	// creates a verifiable chunk
	vChunk := unittest.VerifiableChunkFixture()

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
			suite.Assert().Equal(vChunk.Receipt.ExecutionResult.ID(), ra.ResultApprovalBody.ExecutionResultID)

			// verifies the signature over the result approval
			batst, err := encoding.DefaultEncoder.Encode(ra.ResultApprovalBody.Attestation())
			suite.Assert().NoError(err)
			suite.Assert().True(suite.sk.PublicKey().Verify(ra.VerifierSignature, batst, suite.hasher))
		}).
		Once()

	err := eng.Process(myID, vChunk)
	suite.Assert().Nil(err)

	suite.ss.AssertExpectations(suite.T())
	suite.conduit.AssertExpectations(suite.T())
}

// MockLocal represents a mock of Local
// We needed to develop a separate mock for Local as we could not mock
// a method with return values
type MockLocal struct {
	sk crypto.PrivateKey
	t  testifymock.TestingT
	id flow.Identifier
}

func NewMockLocal(sk crypto.PrivateKey, id flow.Identifier, t testifymock.TestingT) *MockLocal {
	return &MockLocal{
		sk: sk,
		t:  t,
		id: id,
	}
}

func (m *MockLocal) NodeID() flow.Identifier {
	return m.id
}

func (m *MockLocal) Address() string {
	require.Fail(m.t, "should not call MockLocal Address")
	return ""
}

func (m *MockLocal) Sign(msg []byte, hasher crypto.Hasher) (crypto.Signature, error) {
	return m.sk.Sign(msg, hasher)
}

func (m *MockLocal) MockNodeID(id flow.Identifier) {
	m.id = id
}

func (m *MockLocal) NotMeFilter() flow.IdentityFilter {
	return filter.Not(filter.HasNodeID(m.id))
}
