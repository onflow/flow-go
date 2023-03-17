package follower

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	realstorage "github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFollowerEngine(t *testing.T) {
	suite.Run(t, new(EngineSuite))
}

type EngineSuite struct {
	CoreSuite

	net *mocknetwork.Network
	con *mocknetwork.Conduit
	me  *module.Local

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error
	engine *Engine
}

func (s *EngineSuite) SetupTest() {
	s.CoreSuite.SetupTest()

	s.net = mocknetwork.NewNetwork(s.T())
	s.con = mocknetwork.NewConduit(s.T())
	s.me = module.NewLocal(s.T())

	nodeID := unittest.IdentifierFixture()
	s.me.On("NodeID").Return(nodeID).Maybe()

	s.net.On("Register", mock.Anything, mock.Anything).Return(s.con, nil)

	metrics := metrics.NewNoopCollector()
	eng, err := New(
		unittest.Logger(),
		s.net,
		s.me,
		metrics,
		s.core)
	require.Nil(s.T(), err)

	s.engine = eng

	s.ctx, s.cancel, s.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	s.engine.Start(s.ctx)
	unittest.RequireCloseBefore(s.T(), s.engine.Ready(), time.Second, "engine failed to start")
}

// TearDownTest stops the engine and checks there are no errors thrown to the SignallerContext.
func (s *EngineSuite) TearDownTest() {
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.engine.Done(), time.Second, "engine failed to stop")
	select {
	case err := <-s.errs:
		assert.NoError(s.T(), err)
	default:
	}
}

// TestProcessSyncedBlock checks if processing synced block using unsafe API results in error.
// All blocks from sync engine should be sent through dedicated compliance API.
func (s *EngineSuite) TestProcessSyncedBlock() {
	parent := unittest.BlockFixture()
	block := unittest.BlockFixture()

	parent.Header.Height = 10
	block.Header.Height = 11
	block.Header.ParentID = parent.ID()

	// not in cache
	s.cache.On("ByID", block.ID()).Return(flow.Slashable[*flow.Block]{}, false).Once()
	s.cache.On("ByID", block.Header.ParentID).Return(flow.Slashable[*flow.Block]{}, false).Once()
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	done := make(chan struct{})
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
	s.follower.On("SubmitProposal", hotstuffProposal).Run(func(_ mock.Arguments) {
		close(done)
	}).Once()

	s.engine.OnSyncedBlocks(flow.Slashable[[]*messages.BlockProposal]{
		OriginID: unittest.IdentifierFixture(),
		Message:  []*messages.BlockProposal{messages.NewBlockProposal(&block)},
	})
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}
