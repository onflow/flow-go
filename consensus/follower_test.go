package consensus_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/onflow/flow-go/module/signature"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
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
//   - The Follower is running in a single go-routine, pulling one event at a time from an
//     _unbuffered_ channel. The test will send blocks to the Follower's input channel and block there
//     until the Follower receives the block from the channel. Hence, when all sends have completed, the
//     Follower has processed all blocks but the last one. Furthermore, the last block has already been
//     received.
//   - Therefore, the Follower will only pick up a shutdown signal _after_ it processed the last block.
//     Hence, waiting for the Follower's `Done()` channel guarantees that it complete processing any
//     blocks that are in the event loop.
//
// For this test, most of the Follower's injected components are mocked out.
// As we test the mocked components separately, we assume:
//   - The mocked components work according to specification.
//   - Especially, we assume that Forks works according to specification, i.e. that the determination of
//     finalized blocks is correct and events are emitted in the desired order (both are tested separately).
func TestHotStuffFollower(t *testing.T) {
	suite.Run(t, new(HotStuffFollowerSuite))
}

type HotStuffFollowerSuite struct {
	suite.Suite

	committee     *mockhotstuff.DynamicCommittee
	headers       *mockstorage.Headers
	finalizer     *mockmodule.Finalizer
	verifier      *mockhotstuff.Verifier
	notifier      *mockhotstuff.FinalizationConsumer
	rootHeader    *flow.Header
	rootQC        *flow.QuorumCertificate
	finalized     *flow.Header
	pending       []*flow.Header
	follower      *hotstuff.FollowerLoop
	mockConsensus *MockConsensus

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error
}

// SetupTest initializes all the components needed for the Follower.
// The follower itself is instantiated in method BeforeTest
func (s *HotStuffFollowerSuite) SetupTest() {
	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	s.mockConsensus = &MockConsensus{identities: identities}

	// mock consensus committee
	s.committee = &mockhotstuff.DynamicCommittee{}
	s.committee.On("IdentitiesByEpoch", mock.Anything).Return(
		func(_ uint64) flow.IdentityList {
			return identities
		},
		nil,
	)
	for _, identity := range identities {
		s.committee.On("IdentityByEpoch", mock.Anything, identity.NodeID).Return(identity, nil)
		s.committee.On("IdentityByBlock", mock.Anything, identity.NodeID).Return(identity, nil)
	}
	s.committee.On("LeaderForView", mock.Anything).Return(
		func(view uint64) flow.Identifier { return identities[int(view)%len(identities)].NodeID },
		nil,
	)
	s.committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(identities.TotalWeight()), nil)

	// mock storage headers
	s.headers = &mockstorage.Headers{}

	// mock finalization finalizer
	s.finalizer = mockmodule.NewFinalizer(s.T())

	// mock finalization finalizer
	s.verifier = mockhotstuff.NewVerifier(s.T())
	s.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	s.verifier.On("VerifyTC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// mock consumer for finalization notifications
	s.notifier = mockhotstuff.NewFinalizationConsumer(s.T())

	// root block and QC
	parentID, err := flow.HexStringToIdentifier("aa7693d498e9a087b1cadf5bfe9a1ff07829badc1915c210e482f369f9a00a70")
	require.NoError(s.T(), err)
	s.rootHeader = &flow.Header{
		ParentID:   parentID,
		Timestamp:  time.Now().UTC(),
		Height:     21053,
		View:       52078,
		ParentView: 52077,
	}

	signerIndices, err := signature.EncodeSignersToIndices(identities.NodeIDs(), identities.NodeIDs()[:3])
	require.NoError(s.T(), err)
	s.rootQC = &flow.QuorumCertificate{
		View:          s.rootHeader.View,
		BlockID:       s.rootHeader.ID(),
		SignerIndices: signerIndices,
	}

	// we start with the latest finalized block being the root block
	s.finalized = s.rootHeader
	// and no pending (unfinalized) block
	s.pending = []*flow.Header{}
}

// BeforeTest instantiates and starts Follower
func (s *HotStuffFollowerSuite) BeforeTest(suiteName, testName string) {
	var err error
	s.follower, err = consensus.NewFollower(
		zerolog.New(os.Stderr),
		s.committee,
		s.headers,
		s.finalizer,
		s.verifier,
		s.notifier,
		s.rootHeader,
		s.rootQC,
		s.finalized,
		s.pending,
	)
	require.NoError(s.T(), err)

	s.ctx, s.cancel, s.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	s.follower.Start(s.ctx)
	unittest.RequireCloseBefore(s.T(), s.follower.Ready(), time.Second, "follower failed to start")
}

// AfterTest stops follower and asserts that the Follower executed the expected callbacks.
func (s *HotStuffFollowerSuite) AfterTest(suiteName, testName string) {
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.follower.Done(), time.Second, "follower failed to stop")
	select {
	case err := <-s.errs:
		require.NoError(s.T(), err)
	default:
	}
}

// TestInitialization verifies that the basic test setup with initialization of the Follower works as expected
func (s *HotStuffFollowerSuite) TestInitialization() {
	// we expect no additional calls to s.finalizer or s.notifier besides what is already specified in BeforeTest
}

// TestSubmitProposal verifies that when submitting a single valid block (child's root block),
// the Follower reacts with callbacks to s.notifier.OnBlockIncorporated with this new block
func (s *HotStuffFollowerSuite) TestSubmitProposal() {
	rootBlockView := s.rootHeader.View
	nextBlock := s.mockConsensus.extendBlock(rootBlockView+1, s.rootHeader)

	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.submitProposal(nextBlock)
}

// TestFollowerFinalizedBlock verifies that when submitting 2 extra blocks
// the Follower reacts with callbacks to s.notifier.OnBlockIncorporated
// for all the added blocks. Furthermore, the follower should finalize the first submitted block,
// i.e. call s.finalizer.MakeFinal and s.notifier.OnFinalizedBlock
func (s *HotStuffFollowerSuite) TestFollowerFinalizedBlock() {
	expectedFinalized := s.mockConsensus.extendBlock(s.rootHeader.View+1, s.rootHeader)
	s.notifier.On("OnBlockIncorporated", blockWithID(expectedFinalized.ID())).Return().Once()
	s.submitProposal(expectedFinalized)

	// direct 1-chain on top of expectedFinalized
	nextBlock := s.mockConsensus.extendBlock(expectedFinalized.View+1, expectedFinalized)
	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.submitProposal(nextBlock)

	done := make(chan struct{})

	// indirect 2-chain on top of expectedFinalized
	lastBlock := nextBlock
	nextBlock = s.mockConsensus.extendBlock(lastBlock.View+5, lastBlock)
	s.notifier.On("OnBlockIncorporated", blockWithID(nextBlock.ID())).Return().Once()
	s.notifier.On("OnFinalizedBlock", blockWithID(expectedFinalized.ID())).Return().Once()
	s.finalizer.On("MakeFinal", blockID(expectedFinalized.ID())).Run(func(_ mock.Arguments) {
		close(done)
	}).Return(nil).Once()
	s.submitProposal(nextBlock)
	unittest.RequireCloseBefore(s.T(), done, time.Second, "expect to close before timeout")
}

// TestOutOfOrderBlocks verifies that when submitting a variety of blocks with view numbers
// OUT OF ORDER, the Follower reacts with callbacks to s.notifier.OnBlockIncorporated
// for all the added blocks. Furthermore, we construct the test such that the follower should finalize
// eventually a bunch of blocks in one go.
// The following illustrates the tree of submitted blocks, with notation
//
//	                                                    [52078+14, 52078+20] (should finalize this fork)
//	                                                                       |
//	                                                                       |
//	                                                    [52078+13, 52078+14]
//	                                                                       |
//	                                                                       |
//	                       [52078+11, 52078+17]         [52078+ 9, 52078+13]   [52078+ 9, 52078+10]
//	                       |                                               |  /
//	                       |                                               | /
//	[52078+ 7, 52078+ 8]   [52078+ 7, 52078+11]         [52078+ 5, 52078+ 9]   [52078+ 5, 52078+ 6]
//	                     \ |                                               |  /
//	                      \|                                               | /
//	[52078+ 3, 52078+ 4]   [52078+ 3, 52078+ 7]         [52078+ 1, 52078+ 5]   [52078+ 1, 52078+ 2]
//	                     \ |                                               |  /
//	                      \|                                               | /
//	                       [52078+ 0, 52078+ 3]         [52078+ 0, 52078+ 1]
//	                                          \         /
//	                                           \       /
//	                                         [52078+ 0, x] (root block; no qc to parent)
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

	block17 := s.mockConsensus.extendBlock(rootView+17, block11)
	block13 := s.mockConsensus.extendBlock(rootView+13, block09)
	block10 := s.mockConsensus.extendBlock(rootView+10, block09)

	block14 := s.mockConsensus.extendBlock(rootView+14, block13)
	block20 := s.mockConsensus.extendBlock(rootView+20, block14)

	for _, b := range []*flow.Header{block01, block02, block03, block04, block05, block06, block07, block08, block09, block10, block11, block13, block14, block17, block20} {
		s.notifier.On("OnBlockIncorporated", blockWithID(b.ID())).Return().Once()
	}

	// now we feed the blocks in some wild view order into the Follower
	// (Caution: we still have to make sure the parent is known, before we give its child to the Follower)
	s.submitProposal(block03)
	s.submitProposal(block07)
	s.submitProposal(block11)
	s.submitProposal(block01)
	s.submitProposal(block05)
	s.submitProposal(block17)
	s.submitProposal(block09)
	s.submitProposal(block06)
	s.submitProposal(block10)
	s.submitProposal(block04)
	s.submitProposal(block13)
	s.submitProposal(block14)
	s.submitProposal(block08)
	s.submitProposal(block02)

	done := make(chan struct{})

	// Block 20 should now finalize the fork up to and including block13
	s.notifier.On("OnFinalizedBlock", blockWithID(block01.ID())).Return().Once()
	s.finalizer.On("MakeFinal", blockID(block01.ID())).Return(nil).Once()
	s.notifier.On("OnFinalizedBlock", blockWithID(block05.ID())).Return().Once()
	s.finalizer.On("MakeFinal", blockID(block05.ID())).Return(nil).Once()
	s.notifier.On("OnFinalizedBlock", blockWithID(block09.ID())).Return().Once()
	s.finalizer.On("MakeFinal", blockID(block09.ID())).Return(nil).Once()
	s.notifier.On("OnFinalizedBlock", blockWithID(block13.ID())).Return().Once()
	s.finalizer.On("MakeFinal", blockID(block13.ID())).Run(func(_ mock.Arguments) {
		close(done)
	}).Return(nil).Once()
	s.submitProposal(block20)
	unittest.RequireCloseBefore(s.T(), done, time.Second, "expect to close before timeout")
}

// blockWithID returns a testify `argumentMatcher` that only accepts blocks with the given ID
func blockWithID(expectedBlockID flow.Identifier) interface{} {
	return mock.MatchedBy(func(block *model.Block) bool { return expectedBlockID == block.BlockID })
}

// blockID returns a testify `argumentMatcher` that only accepts the given ID
func blockID(expectedBlockID flow.Identifier) interface{} {
	return mock.MatchedBy(func(blockID flow.Identifier) bool { return expectedBlockID == blockID })
}

// submitProposal submits the given (proposal, parentView) pair to the Follower.
func (s *HotStuffFollowerSuite) submitProposal(proposal *flow.Header) {
	s.follower.SubmitProposal(model.ProposalFromFlow(proposal))
}

// MockConsensus is used to generate Blocks for a mocked consensus committee
type MockConsensus struct {
	identities flow.IdentityList
}

func (mc *MockConsensus) extendBlock(blockView uint64, parent *flow.Header) *flow.Header {
	nextBlock := unittest.BlockHeaderWithParentFixture(parent)
	nextBlock.View = blockView
	nextBlock.ProposerID = mc.identities[int(blockView)%len(mc.identities)].NodeID
	signerIndices, _ := signature.EncodeSignersToIndices(mc.identities.NodeIDs(), mc.identities.NodeIDs())
	nextBlock.ParentVoterIndices = signerIndices
	if nextBlock.View == parent.View+1 {
		nextBlock.LastViewTC = nil
	} else {
		newestQC := unittest.QuorumCertificateFixture(func(qc *flow.QuorumCertificate) {
			qc.View = parent.View
			qc.SignerIndices = signerIndices
		})
		nextBlock.LastViewTC = &flow.TimeoutCertificate{
			View:          blockView - 1,
			NewestQCViews: []uint64{newestQC.View},
			NewestQC:      newestQC,
			SignerIndices: signerIndices,
			SigData:       unittest.SignatureFixture(),
		}
	}
	return nextBlock
}
