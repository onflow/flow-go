package consensus_test

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	mockhotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	mockstorage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestHotStuffFollower is a test suite for the HotStuff Follower.
// The main focus of this test suite is to test that the follower generates the expected callbacks to
// module.Finalizer and hotstuff.FinalizationConsumer. In this context, note that the Follower internally
// has its own processing thread. Therefore, the test must be concurrency safe and ensure that the Follower
// has asynchronously processed the submitted blocks _before_ we assert whether all callbacks were run.
// We use the following knowledge about the Follower's _internal_ processing:
//   * The Follower is running in a single go-routine, pulling one event at a time from an
//     _unbuffered_ channel. The test will send blocks to the Follower's input channel and block there
//     until the Follower receives the block from the channel. Hence, when all sends have completed, the
//     Follower has processed all blocks but the last one. Furthermore, the last block has already been
//     received.
//   * Therefore, the Follower will only pick up a shutdown signal _after_ it processed the last block.
//     Hence, waiting for the Follower's `Done()` channel guarantees that it complete processing any
//     blocks that are in the event loop.
// For this test, most of the Follower's injected components are mocked out.
// As we test the mocked components separately, we assume here that they work according to specification.
// Furthermore, we also assume that Forks works according to specification, i.e. that the determination of
// finalized blocks is correct (which is also tested separately)
func TestHotStuffFollower(t *testing.T) {
	suite.Run(t, new(HotStuffFollowerSuite))
}

type HotStuffFollowerSuite struct {
	suite.Suite

	committee  *mockhotstuff.Committee
	headers    *mockstorage.Headers
	updater    *mockmodule.Finalizer
	verifier   *mockhotstuff.Verifier
	notifier   *mockhotstuff.FinalizationConsumer
	rootHeader *flow.Header
	rootQC     *model.QuorumCertificate
	finalized  *flow.Header
	pending    []*flow.Header
	follower   *hotstuff.FollowerLoop

	mockConsensus *MockConsensus
}

// SetupTest initializes all the components needed for the Follower.
// The follower itself is instantiated in method BeforeTest
func (s *HotStuffFollowerSuite) SetupTest() {
	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	s.mockConsensus = &MockConsensus{identities: identities}

	// mock consensus committee
	s.committee = &mockhotstuff.Committee{}
	s.committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)
	for _, identity := range identities {
		s.committee.On("Identity", mock.Anything, identity.NodeID).Return(identity, nil)
	}
	s.committee.On("LeaderForView", mock.Anything).Return(
		func(view uint64) flow.Identifier { return identities[int(view)%len(identities)].NodeID },
		nil,
	)

	// mock storage headers
	s.headers = &mockstorage.Headers{}

	// mock finalization updater
	s.updater = &mockmodule.Finalizer{}
	//s.updater.On("MakePending", mock.Anything).Return(nil).Run(
	//	func(args mock.Arguments) {
	//		require.True(s.T(), s.expectedUpdaterRecord.HasNextExpectedRecord(), "no expected record for this call")
	//		expectedMethodName, expectedBlockID := s.expectedUpdaterRecord.NextExpectedRecord()
	//		require.Equal(s.T(), expectedMethodName, "MakePending", "unexpected method call")
	//		blockID := args.Get(0).(flow.Identifier)
	//		require.Equal(s.T(), expectedBlockID, blockID, "unexpected block ID")
	//	},
	//)
	//s.updater.On("MakeFinal", mock.Anything).Return(nil).Run(
	//	func(args mock.Arguments) {
	//		require.True(s.T(), s.expectedUpdaterRecord.HasNextExpectedRecord(), "no expected record for this call")
	//		expectedMethodName, expectedBlockID := s.expectedUpdaterRecord.NextExpectedRecord()
	//		require.Equal(s.T(), expectedMethodName, "MakeFinal", "unexpected method call")
	//		blockID := args.Get(0).(flow.Identifier)
	//		require.Equal(s.T(), expectedBlockID, blockID, "unexpected block ID")
	//	},
	//)

	// mock finalization updater
	s.verifier = &mockhotstuff.Verifier{}
	s.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// mock consumer for finalization notifications
	s.notifier = &mockhotstuff.FinalizationConsumer{}
	//s.notifier.On("OnBlockIncorporated", mock.Anything).Return().Run(
	//	func(args mock.Arguments) {
	//		require.True(s.T(), s.expectedNotifierRecord.HasNextExpectedRecord(), "no expected record for this call")
	//		expectedMethodName, expectedBlockID := s.expectedNotifierRecord.NextExpectedRecord()
	//		require.Equal(s.T(), expectedMethodName, "OnBlockIncorporated", "unexpected method call")
	//		block := args.Get(0).(*model.Block)
	//		require.Equal(s.T(), expectedBlockID, block.BlockID, "unexpected block ID")
	//	},
	//)
	//s.notifier.On("OnFinalizedBlock", mock.Anything).Return().Run(
	//	func(args mock.Arguments) {
	//		require.True(s.T(), s.expectedNotifierRecord.HasNextExpectedRecord(), "no expected record for this call")
	//		expectedMethodName, expectedBlockID := s.expectedNotifierRecord.NextExpectedRecord()
	//		require.Equal(s.T(), expectedMethodName, "OnFinalizedBlock", "unexpected method call")
	//		block := args.Get(0).(*model.Block)
	//		require.Equal(s.T(), expectedBlockID, block.BlockID, "unexpected block ID")
	//	},
	//)

	// root block and QC
	parentID, err := flow.HexStringToIdentifier("aa7693d498e9a087b1cadf5bfe9a1ff07829badc1915c210e482f369f9a00a70")
	require.NoError(s.T(), err)
	s.rootHeader = &flow.Header{
		ParentID:  parentID,
		Timestamp: time.Now().UTC(),
		Height:    21053,
		View:      52078,
	}
	s.rootQC = &model.QuorumCertificate{
		View:      s.rootHeader.View,
		BlockID:   s.rootHeader.ID(),
		SignerIDs: identities.NodeIDs()[:3],
	}

	// we start with the latest finalized block being the root block
	s.finalized = s.rootHeader
	// and no pending (unfinalized) block
	s.pending = []*flow.Header{}
}

// BeforeTest instantiates and starts Follower
func (s *HotStuffFollowerSuite) BeforeTest(suiteName, testName string) {
	s.notifier.On("OnBlockIncorporated", blockWithID(s.rootHeader.ID())).Return().Once()

	var err error
	s.follower, err = consensus.NewFollower(
		zerolog.New(os.Stderr),
		s.committee,
		s.headers,
		s.updater,
		s.verifier,
		s.notifier,
		s.rootHeader,
		s.rootQC,
		s.finalized,
		s.pending,
	)
	require.NoError(s.T(), err)

	select {
	case <-s.follower.Ready():
	case <-time.After(time.Second):
		s.T().Error("timeout on waiting for follower start")
	}
}

// AfterTest stops follower and asserts that the Follower executed the expected callbacks
// to s.updater.MakeValid or s.notifier.OnBlockIncorporated
func (s *HotStuffFollowerSuite) AfterTest(suiteName, testName string) {
	select {
	case <-s.follower.Done():
	case <-time.After(time.Second):
		s.T().Error("timeout on waiting for expected Follower shutdown")
	}
	s.notifier.AssertExpectations(s.T())
	s.updater.AssertExpectations(s.T())
}

// TestFollowerInitialization verifies that the basic test setup with initialization of the Follower works as expected
func (s *HotStuffFollowerSuite) TestInitialization() {
	// we expect no additional calls to s.updater or s.notifier
}

// TestFollowerProcessBlock verifies that when submitting a single valid block (child root block),
// the Follower reacts with callbacks to s.updater.MakeValid or s.notifier.OnBlockIncorporated with this new block
func (s *HotStuffFollowerSuite) TestSubmitProposal() {
	rootBlockView := s.rootHeader.View
	nextBlock := s.mockConsensus.extendBlock(rootBlockView+1, s.rootHeader)

	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.updater.On("MakeValid", blockID(nextBlock.ID())).Return(nil).Once()
	s.follower.SubmitProposal(nextBlock, rootBlockView)
}

// TestFollowerProcessBlock verifies that when submitting a single valid block (child root block),
// the Follower reacts with callbacks to s.updater.MakeValid or s.notifier.OnBlockIncorporated with this new block
func (s *HotStuffFollowerSuite) TestFollowerFinalizedBlock() {
	expectedFinalized := s.mockConsensus.extendBlock(s.rootHeader.View+1, s.rootHeader)
	s.notifier.On("OnBlockIncorporated", blockWithID(expectedFinalized.ID())).Return().Once()
	s.updater.On("MakeValid", blockID(expectedFinalized.ID())).Return(nil).Once()
	s.follower.SubmitProposal(expectedFinalized, s.rootHeader.View)

	// direct 1-chain on top of expectedFinalized
	nextBlock := s.mockConsensus.extendBlock(expectedFinalized.View+1, expectedFinalized)
	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.updater.On("MakeValid", blockID(nextBlock.ID())).Return(nil).Once()
	s.follower.SubmitProposal(nextBlock, expectedFinalized.View)

	// direct 2-chain on top of expectedFinalized
	lastBlock := nextBlock
	nextBlock = s.mockConsensus.extendBlock(lastBlock.View+1, lastBlock)
	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.updater.On("MakeValid", blockID(nextBlock.ID())).Return(nil).Once()
	s.follower.SubmitProposal(nextBlock, lastBlock.View)

	// indirect 3-chain on top of expectedFinalized => finalization
	lastBlock = nextBlock
	nextBlock = s.mockConsensus.extendBlock(lastBlock.View+5, lastBlock)
	s.notifier.On("OnFinalizedBlock", blockWithID(expectedFinalized.ID())).Return().Once()
	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.updater.On("MakeFinal", blockID(expectedFinalized.ID())).Return(nil).Once()
	s.updater.On("MakeValid", blockID(nextBlock.ID())).Return(nil).Once()
	s.follower.SubmitProposal(nextBlock, lastBlock.View)
}

// MockConsensus is used to generate Blocks for a mocked consensus committee
type MockConsensus struct {
	identities flow.IdentityList
}

func (mc *MockConsensus) extendBlock(blockView uint64, parent *flow.Header) *flow.Header {
	nextBlock := unittest.BlockHeaderWithParentFixture(parent)
	nextBlock.View = blockView
	nextBlock.ProposerID = mc.identities[int(blockView)%len(mc.identities)].NodeID
	nextBlock.ParentVoterIDs = mc.identities.NodeIDs()
	return &nextBlock
}

func blockWithID(expectedBlockID flow.Identifier) interface{} {
	return mock.MatchedBy(func(block *model.Block) bool { return expectedBlockID == block.BlockID })
}

func blockID(expectedBlockID flow.Identifier) interface{} {
	return mock.MatchedBy(func(blockID flow.Identifier) bool { return expectedBlockID == blockID })
}
