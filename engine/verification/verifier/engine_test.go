package verifier_test

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/mock"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"

	"github.com/onflow/flow-go/engine/testutil/mocklocal"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/engine/verification/verifier"
	chmodel "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	realModule "github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type VerifierEngineTestSuite struct {
	suite.Suite
	net           *mocknetwork.Network
	tracer        realModule.Tracer
	state         *protocol.State
	ss            *protocol.Snapshot
	me            *mocklocal.MockLocal
	sk            crypto.PrivateKey
	hasher        hash.Hasher
	chain         flow.Chain
	pushCon       *mocknetwork.Conduit // mocks con for submitting result approvals
	pullCon       *mocknetwork.Conduit
	metrics       *mockmodule.VerificationMetrics // mocks performance monitoring metrics
	approvals     *mockstorage.ResultApprovals
	chunkVerifier *mockmodule.ChunkVerifier
}

func TestVerifierEngine(t *testing.T) {
	suite.Run(t, new(VerifierEngineTestSuite))
}

func (suite *VerifierEngineTestSuite) SetupTest() {

	suite.state = new(protocol.State)
	suite.net = mocknetwork.NewNetwork(suite.T())
	suite.tracer = trace.NewNoopTracer()
	suite.ss = new(protocol.Snapshot)
	suite.pushCon = mocknetwork.NewConduit(suite.T())
	suite.pullCon = mocknetwork.NewConduit(suite.T())
	suite.metrics = mockmodule.NewVerificationMetrics(suite.T())
	suite.chain = flow.Testnet.Chain()
	suite.approvals = mockstorage.NewResultApprovals(suite.T())
	suite.chunkVerifier = mockmodule.NewChunkVerifier(suite.T())

	suite.approvals.On("Store", mock.Anything).Return(nil)
	suite.approvals.On("Index", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	suite.net.On("Register", channels.PushApprovals, testifymock.Anything).
		Return(suite.pushCon, nil).
		Once()

	suite.net.On("Register", channels.ProvideApprovalsByChunk, testifymock.Anything).
		Return(suite.pullCon, nil).
		Once()

	suite.state.On("Final").Return(suite.ss)

	// Mocks the signature oracle of the engine
	//
	// generates signing and verification keys
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	n, err := rand.Read(seed)
	require.Equal(suite.T(), n, crypto.KeyGenSeedMinLen)
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

func (suite *VerifierEngineTestSuite) getTestNewEngine() *verifier.Engine {
	e, err := verifier.New(
		unittest.Logger(),
		suite.metrics,
		suite.tracer,
		suite.net,
		suite.state,
		suite.me,
		suite.chunkVerifier,
		suite.approvals)
	require.Nil(suite.T(), err)

	suite.net.AssertExpectations(suite.T())
	return e

}

// TestVerifyHappyPath tests the verification path for a single verifiable chunk, which is
// assigned to the verifier node, and is passed by the ingest engine
// The tests evaluates that a result approval is emitted to all consensus nodes
// about the input execution receipt
func (suite *VerifierEngineTestSuite) TestVerifyHappyPath() {

	eng := suite.getTestNewEngine()
	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))
	// creates a verifiable chunk
	vChunk := unittest.VerifiableChunkDataFixture(uint64(0))

	suite.chunkVerifier.On("Verify", vChunk).Return(nil, nil)

	// mocking node ID using the LocalMock
	suite.me.MockNodeID(myID)
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil)

	// mocks metrics
	// reception of verifiable chunk
	suite.metrics.On("OnVerifiableChunkReceivedAtVerifierEngine").Return()
	// emission of result approval
	suite.metrics.On("OnResultApprovalDispatchedInNetworkByVerifier").Return()

	suite.pushCon.
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

	err := eng.ProcessLocal(vChunk)
	suite.Assert().NoError(err)
	suite.ss.AssertExpectations(suite.T())
	suite.pushCon.AssertExpectations(suite.T())

}

func (suite *VerifierEngineTestSuite) TestVerifyUnhappyPaths() {
	eng := suite.getTestNewEngine()
	myID := unittest.IdentifierFixture()
	consensusNodes := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleConsensus))

	// mocking node ID using the LocalMock
	suite.me.MockNodeID(myID)
	suite.ss.On("Identities", testifymock.Anything).Return(consensusNodes, nil)

	// mocks metrics
	// reception of verifiable chunk
	suite.metrics.On("OnVerifiableChunkReceivedAtVerifierEngine").Return()

	// we shouldn't receive any result approval
	suite.pushCon.
		On("Publish", testifymock.Anything, testifymock.Anything).
		Return(nil).
		Run(func(args testifymock.Arguments) {
			// TODO change this to check challenges
			_, ok := args[0].(*flow.ResultApproval)
			// TODO change this to false when missing register is rolled back
			suite.Assert().True(ok)
		})

	// emission of result approval
	suite.metrics.On("OnResultApprovalDispatchedInNetworkByVerifier").Return()

	var tests = []struct {
		errFn func(vc *verification.VerifiableChunkData) error
	}{
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFMissingRegisterTouch(
					[]string{"test missing register touch"},
					vc.Chunk.Index,
					vc.Result.ID(),
					unittest.TransactionFixture().ID())
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFInvalidVerifiableChunk(
					"test",
					errors.New("test invalid verifiable chunk"),
					vc.Chunk.Index,
					vc.Result.ID())
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFNonMatchingFinalState(
					unittest.StateCommitmentFixture(),
					unittest.StateCommitmentFixture(),
					vc.Chunk.Index,
					vc.Result.ID())
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFInvalidEventsCollection(
					unittest.IdentifierFixture(),
					unittest.IdentifierFixture(),
					vc.Chunk.Index,
					vc.Result.ID(),
					flow.EventsList{})
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFSystemChunkIncludedCollection(vc.Chunk.Index, vc.Result.ID())
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFExecutionDataBlockIDMismatch(
					unittest.IdentifierFixture(),
					unittest.IdentifierFixture(),
					vc.Chunk.Index,
					vc.Result.ID())
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFExecutionDataChunksLengthMismatch(
					0,
					0,
					vc.Chunk.Index,
					vc.Result.ID())
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFExecutionDataInvalidChunkCID(
					cid.Cid{},
					cid.Cid{},
					vc.Chunk.Index,
					vc.Result.ID())
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return chmodel.NewCFInvalidExecutionDataID(
					unittest.IdentifierFixture(),
					unittest.IdentifierFixture(),
					vc.Chunk.Index,
					vc.Result.ID())
			},
		},
		{
			errFn: func(vc *verification.VerifiableChunkData) error {
				return errors.New("test error")
			},
		},
	}

	for i, test := range tests {
		vc := unittest.VerifiableChunkDataFixture(uint64(i))
		expectedErr := test.errFn(vc)

		suite.chunkVerifier.On("Verify", vc).Return(nil, expectedErr).Once()

		err := eng.ProcessLocal(vc)

		// no error returned from the engine
		suite.Assert().NoError(err)
	}
}
