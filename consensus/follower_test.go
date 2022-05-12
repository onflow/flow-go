package consensus_test

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	mockmodule "github.com/onflow/flow-go/module/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
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
// As we test the mocked components separately, we assume:
//   * The mocked components work according to specification.
//   * Especially, we assume that Forks works according to specification, i.e. that the determination of
//     finalized blocks is correct and events are emitted in the desired order (both are tested separately).
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
	rootQC     *flow.QuorumCertificate
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

	// mock finalization updater
	s.verifier = &mockhotstuff.Verifier{}
	s.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// mock consumer for finalization notifications
	s.notifier = &mockhotstuff.FinalizationConsumer{}

	// root block and QC
	parentID, err := flow.HexStringToIdentifier("aa7693d498e9a087b1cadf5bfe9a1ff07829badc1915c210e482f369f9a00a70")
	require.NoError(s.T(), err)
	s.rootHeader = &flow.Header{
		ParentID:  parentID,
		Timestamp: time.Now().UTC(),
		Height:    21053,
		View:      52078,
	}
	s.rootQC = &flow.QuorumCertificate{
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
		s.T().FailNow() // stops the test
	}
	s.notifier.AssertExpectations(s.T())
	s.updater.AssertExpectations(s.T())
}

// TestInitialization verifies that the basic test setup with initialization of the Follower works as expected
func (s *HotStuffFollowerSuite) TestInitialization() {
	// we expect no additional calls to s.updater or s.notifier besides what is already specified in BeforeTest
}

// TestSubmitProposal verifies that when submitting a single valid block (child's root block),
// the Follower reacts with callbacks to s.updater.MakeValid and s.notifier.OnBlockIncorporated with this new block
func (s *HotStuffFollowerSuite) TestSubmitProposal() {
	rootBlockView := s.rootHeader.View
	nextBlock := s.mockConsensus.extendBlock(rootBlockView+1, s.rootHeader)

	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.updater.On("MakeValid", blockID(nextBlock.ID())).Return(nil).Once()
	s.submitWithTimeout(nextBlock, rootBlockView)
}

// TestFollowerFinalizedBlock verifies that when submitting 4 extra blocks
// the Follower reacts with callbacks to s.updater.MakeValid or s.notifier.OnBlockIncorporated
// for all the added blocks. Furthermore, the follower should finalize the first submitted block,
// i.e. call s.updater.MakeFinal and s.notifier.OnFinalizedBlock
func (s *HotStuffFollowerSuite) TestFollowerFinalizedBlock() {
	expectedFinalized := s.mockConsensus.extendBlock(s.rootHeader.View+1, s.rootHeader)
	s.notifier.On("OnBlockIncorporated", blockWithID(expectedFinalized.ID())).Return().Once()
	s.updater.On("MakeValid", blockID(expectedFinalized.ID())).Return(nil).Once()
	s.submitWithTimeout(expectedFinalized, s.rootHeader.View)

	// direct 1-chain on top of expectedFinalized
	nextBlock := s.mockConsensus.extendBlock(expectedFinalized.View+1, expectedFinalized)
	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.updater.On("MakeValid", blockID(nextBlock.ID())).Return(nil).Once()
	s.submitWithTimeout(nextBlock, expectedFinalized.View)

	// direct 2-chain on top of expectedFinalized
	lastBlock := nextBlock
	nextBlock = s.mockConsensus.extendBlock(lastBlock.View+1, lastBlock)
	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.updater.On("MakeValid", blockID(nextBlock.ID())).Return(nil).Once()
	s.submitWithTimeout(nextBlock, lastBlock.View)

	// indirect 3-chain on top of expectedFinalized => finalization
	lastBlock = nextBlock
	nextBlock = s.mockConsensus.extendBlock(lastBlock.View+5, lastBlock)
	s.notifier.On("OnFinalizedBlock", blockWithID(expectedFinalized.ID())).Return().Once()
	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.updater.On("MakeFinal", blockID(expectedFinalized.ID())).Return(nil).Once()
	s.updater.On("MakeValid", blockID(nextBlock.ID())).Return(nil).Once()
	s.submitWithTimeout(nextBlock, lastBlock.View)
}

// TestOutOfOrderBlocks verifies that when submitting a variety of blocks with view numbers
// OUT OF ORDER, the Follower reacts with callbacks to s.updater.MakeValid or s.notifier.OnBlockIncorporated
// for all the added blocks. Furthermore, we construct the test such that the follower should finalize
// eventually a bunch of blocks in one go.
// The following illustrates the tree of submitted blocks, with notation
//   * [a, b] is a block at view "b" with a QC with view "a",
//     e.g., [1, 2] means a block at view "2" with an included  QC for view "1"
//
//                                                       [52078+15, 52078+20] (should finalize this fork)
//                                                                          |
//                                                                          |
//                                                       [52078+14, 52078+15]
//                                                                          |
//                                                                          |
//                                                       [52078+13, 52078+14]
//                                                                          |
//                                                                          |
//   [52078+11, 52078+12]   [52078+11, 52078+17]         [52078+ 9, 52078+13]   [52078+ 9, 52078+10]
//                        \ |                                               |  /
//                         \|                                               | /
//   [52078+ 7, 52078+ 8]   [52078+ 7, 52078+11]         [52078+ 5, 52078+ 9]   [52078+ 5, 52078+ 6]
//                        \ |                                               |  /
//                         \|                                               | /
//   [52078+ 3, 52078+ 4]   [52078+ 3, 52078+ 7]         [52078+ 1, 52078+ 5]   [52078+ 1, 52078+ 2]
//                        \ |                                               |  /
//                         \|                                               | /
//                          [52078+ 0, 52078+ 3]         [52078+ 0, 52078+ 1]
//                                             \         /
//                                              \       /
//                                            [52078+ 0, x] (root block; no qc to parent)
func (s *HotStuffFollowerSuite) TestOutOfOrderBlocks() {
	// in the following, we reference the block's by their view minus the view of the
	// root block (52078). E.g. block [52078+ 9, 52078+10] would be referenced as `block10`
	rootView := s.rootHeader.View

	// constructing blocks bottom up, line by line, left to right
	block03 := s.mockConsensus.extendBlock(rootView+3, s.rootHeader)
	block01 := s.mockConsensus.extendBlock(rootView+1, s.rootHeader)

	block04 := s.mockConsensus.extendBlock(rootView+4, block03)
	block07 := s.mockConsensus.extendBlock(rootView+7, block03)
	block05 := s.mockConsensus.extendBlock(rootView+5, block01)
	block02 := s.mockConsensus.extendBlock(rootView+2, block01)

	block08 := s.mockConsensus.extendBlock(rootView+8, block07)
	block11 := s.mockConsensus.extendBlock(rootView+11, block07)
	block09 := s.mockConsensus.extendBlock(rootView+9, block05)
	block06 := s.mockConsensus.extendBlock(rootView+6, block05)

	block12 := s.mockConsensus.extendBlock(rootView+12, block11)
	block17 := s.mockConsensus.extendBlock(rootView+17, block11)
	block13 := s.mockConsensus.extendBlock(rootView+13, block09)
	block10 := s.mockConsensus.extendBlock(rootView+10, block09)

	block14 := s.mockConsensus.extendBlock(rootView+14, block13)
	block15 := s.mockConsensus.extendBlock(rootView+15, block14)
	block20 := s.mockConsensus.extendBlock(rootView+20, block15)

	for _, b := range []*flow.Header{block01, block02, block03, block04, block05, block06, block07, block08, block09, block10, block11, block12, block13, block14, block15, block17, block20} {
		s.notifier.On("OnBlockIncorporated", blockWithID(b.ID())).Return().Once()
		s.updater.On("MakeValid", blockID(b.ID())).Return(nil).Once()
	}

	// now we feed the blocks in some wild view order into the Follower
	// (Caution: we still have to make sure the parent is known, before we give its child to the Follower)
	s.submitWithTimeout(block03, rootView)
	s.submitWithTimeout(block07, rootView+3)
	s.submitWithTimeout(block11, rootView+7)
	s.submitWithTimeout(block01, rootView)
	s.submitWithTimeout(block12, rootView+11)
	s.submitWithTimeout(block05, rootView+1)
	s.submitWithTimeout(block17, rootView+11)
	s.submitWithTimeout(block09, rootView+5)
	s.submitWithTimeout(block06, rootView+5)
	s.submitWithTimeout(block10, rootView+9)
	s.submitWithTimeout(block04, rootView+3)
	s.submitWithTimeout(block13, rootView+9)
	s.submitWithTimeout(block14, rootView+13)
	s.submitWithTimeout(block08, rootView+7)
	s.submitWithTimeout(block15, rootView+14)
	s.submitWithTimeout(block02, rootView+1)

	// Block 20 should now finalize the fork up to and including block13
	s.notifier.On("OnFinalizedBlock", blockWithID(block01.ID())).Return().Once()
	s.updater.On("MakeFinal", blockID(block01.ID())).Return(nil).Once()
	s.notifier.On("OnFinalizedBlock", blockWithID(block05.ID())).Return().Once()
	s.updater.On("MakeFinal", blockID(block05.ID())).Return(nil).Once()
	s.notifier.On("OnFinalizedBlock", blockWithID(block09.ID())).Return().Once()
	s.updater.On("MakeFinal", blockID(block09.ID())).Return(nil).Once()
	s.notifier.On("OnFinalizedBlock", blockWithID(block13.ID())).Return().Once()
	s.updater.On("MakeFinal", blockID(block13.ID())).Return(nil).Once()
	s.submitWithTimeout(block20, rootView+15)
}

// blockWithID returns a testify `argumentMatcher` that only accepts blocks with the given ID
func blockWithID(expectedBlockID flow.Identifier) interface{} {
	return mock.MatchedBy(func(block *model.Block) bool { return expectedBlockID == block.BlockID })
}

// blockID returns a testify `argumentMatcher` that only accepts the given ID
func blockID(expectedBlockID flow.Identifier) interface{} {
	return mock.MatchedBy(func(blockID flow.Identifier) bool { return expectedBlockID == blockID })
}

// submitWithTimeout submits the given (proposal, parentView) pair to the Follower. As the follower
// might block on this call, we add a timeout that fails the test, in case of a dead-lock.
func (s *HotStuffFollowerSuite) submitWithTimeout(proposal *flow.Header, parentView uint64) {
	sent := make(chan struct{})
	go func() {
		s.follower.SubmitProposal(proposal, parentView)
		close(sent)
	}()
	select {
	case <-sent:
	case <-time.After(time.Second):
		s.T().Error("timeout on waiting for expected Follower shutdown")
		s.T().FailNow() // stops the test
	}

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
