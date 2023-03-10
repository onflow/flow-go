package follower

import (
	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	realstorage "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestFollowerCore(t *testing.T) {
	suite.Run(t, new(CoreSuite))
}

type CoreSuite struct {
	suite.Suite

	con       *mocknetwork.Conduit
	me        *module.Local
	cleaner   *storage.Cleaner
	headers   *storage.Headers
	payloads  *storage.Payloads
	state     *protocol.FollowerState
	snapshot  *protocol.Snapshot
	cache     *module.PendingBlockBuffer
	follower  *module.HotStuffFollower
	sync      *module.BlockRequester
	validator *hotstuff.Validator

	core *Core
}

func (s *CoreSuite) SetupTest() {
	s.con = mocknetwork.NewConduit(s.T())
	s.me = module.NewLocal(s.T())
	s.cleaner = storage.NewCleaner(s.T())
	s.headers = storage.NewHeaders(s.T())
	s.payloads = storage.NewPayloads(s.T())
	s.state = protocol.NewFollowerState(s.T())
	s.snapshot = protocol.NewSnapshot(s.T())
	s.cache = module.NewPendingBlockBuffer(s.T())
	s.follower = module.NewHotStuffFollower(s.T())
	s.validator = hotstuff.NewValidator(s.T())
	s.sync = module.NewBlockRequester(s.T())

	nodeID := unittest.IdentifierFixture()
	s.me.On("NodeID").Return(nodeID).Maybe()

	s.cleaner.On("RunGC").Return().Maybe()
	s.state.On("Final").Return(s.snapshot).Maybe()
	s.cache.On("PruneByView", mock.Anything).Return().Maybe()
	s.cache.On("Size", mock.Anything).Return(uint(0)).Maybe()

	metrics := metrics.NewNoopCollector()
	s.core = NewCore(
		unittest.Logger(),
		metrics,
		s.cleaner,
		s.headers,
		s.payloads,
		s.state,
		s.cache,
		s.follower,
		s.validator,
		s.sync,
		trace.NewNoopTracer())
}

func (s *CoreSuite) TestHandlePendingBlock() {

	originID := unittest.IdentifierFixture()
	head := unittest.BlockFixture()
	block := unittest.BlockFixture()

	head.Header.Height = 10
	block.Header.Height = 12

	// not in cache
	s.cache.On("ByID", block.ID()).Return(flow.Slashable[*flow.Block]{}, false).Once()
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	// don't return the parent when requested
	s.snapshot.On("Head").Return(head.Header, nil)
	s.cache.On("ByID", block.Header.ParentID).Return(flow.Slashable[*flow.Block]{}, false).Once()
	s.headers.On("ByBlockID", block.Header.ParentID).Return(nil, realstorage.ErrNotFound).Once()

	s.cache.On("Add", mock.Anything, mock.Anything).Return(true).Once()
	s.sync.On("RequestBlock", block.Header.ParentID, block.Header.Height-1).Return().Once()

	// submit the block
	proposal := unittest.ProposalFromBlock(&block)
	err := s.core.OnBlockProposal(originID, proposal)
	require.NoError(s.T(), err)

	s.follower.AssertNotCalled(s.T(), "SubmitProposal", mock.Anything)
}

func (s *CoreSuite) TestHandleProposal() {

	originID := unittest.IdentifierFixture()
	parent := unittest.BlockFixture()
	block := unittest.BlockFixture()

	parent.Header.Height = 10
	block.Header.Height = 11
	block.Header.ParentID = parent.ID()

	// not in cache
	s.cache.On("ByID", block.ID()).Return(flow.Slashable[*flow.Block]{}, false).Once()
	s.cache.On("ByID", block.Header.ParentID).Return(flow.Slashable[*flow.Block]{}, false).Once()
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	hotstuffProposal := model.ProposalFromFlow(block.Header)

	// the parent is the last finalized state
	s.snapshot.On("Head").Return(parent.Header, nil)
	// the block passes hotstuff validation
	s.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	// we should be able to extend the state with the block
	s.state.On("ExtendCertified", mock.Anything, &block, (*flow.QuorumCertificate)(nil)).Return(nil).Once()
	// we should be able to get the parent header by its ID
	s.headers.On("ByBlockID", block.Header.ParentID).Return(parent.Header, nil).Once()
	// we do not have any children cached
	s.cache.On("ByParentID", block.ID()).Return(nil, false)
	// the proposal should be forwarded to the follower
	s.follower.On("SubmitProposal", hotstuffProposal).Once()

	// submit the block
	proposal := unittest.ProposalFromBlock(&block)
	err := s.core.OnBlockProposal(originID, proposal)
	require.NoError(s.T(), err)
}

func (s *CoreSuite) TestHandleProposalSkipProposalThreshold() {

	// mock latest finalized state
	final := unittest.BlockHeaderFixture()
	s.snapshot.On("Head").Return(final, nil)

	originID := unittest.IdentifierFixture()
	block := unittest.BlockFixture()

	block.Header.Height = final.Height + compliance.DefaultConfig().SkipNewProposalsThreshold + 1

	// not in cache or storage
	s.cache.On("ByID", block.ID()).Return(flow.Slashable[*flow.Block]{}, false).Once()
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	// submit the block
	proposal := unittest.ProposalFromBlock(&block)
	err := s.core.OnBlockProposal(originID, proposal)
	require.NoError(s.T(), err)

	// block should be dropped - not added to state or cache
	s.state.AssertNotCalled(s.T(), "Extend", mock.Anything)
	s.cache.AssertNotCalled(s.T(), "Add", originID, mock.Anything)
}

// TestHandleProposalWithPendingChildren tests processing a block which has a pending
// child cached.
//   - the block should be processed
//   - the cached child block should also be processed
func (s *CoreSuite) TestHandleProposalWithPendingChildren() {

	originID := unittest.IdentifierFixture()
	parent := unittest.BlockFixture()                       // already processed and incorporated block
	block := unittest.BlockWithParentFixture(parent.Header) // block which is passed as input to the engine
	child := unittest.BlockWithParentFixture(block.Header)  // block which is already cached

	hotstuffProposal := model.ProposalFromFlow(block.Header)
	childHotstuffProposal := model.ProposalFromFlow(child.Header)

	// the parent is the last finalized state
	s.snapshot.On("Head").Return(parent.Header, nil)

	s.cache.On("ByID", mock.Anything).Return(flow.Slashable[*flow.Block]{}, false)
	// first time calling, assume it's not there
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()
	// both blocks pass HotStuff validation
	s.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	s.validator.On("ValidateProposal", childHotstuffProposal).Return(nil)
	// should extend state with the input block, and the child
	s.state.On("ExtendCertified", mock.Anything, block, (*flow.QuorumCertificate)(nil)).Return(nil).Once()
	s.state.On("ExtendCertified", mock.Anything, child, (*flow.QuorumCertificate)(nil)).Return(nil).Once()
	// we have already received and stored the parent
	s.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil).Once()
	// should submit to follower
	s.follower.On("SubmitProposal", hotstuffProposal).Once()
	s.follower.On("SubmitProposal", childHotstuffProposal).Once()

	// we have one pending child cached
	pending := []flow.Slashable[*flow.Block]{
		{
			OriginID: originID,
			Message:  child,
		},
	}
	s.cache.On("ByParentID", block.ID()).Return(pending, true).Once()
	s.cache.On("ByParentID", child.ID()).Return(nil, false).Once()
	s.cache.On("DropForParent", block.ID()).Once()

	// submit the block proposal
	proposal := unittest.ProposalFromBlock(block)
	err := s.core.OnBlockProposal(originID, proposal)
	require.NoError(s.T(), err)
}
