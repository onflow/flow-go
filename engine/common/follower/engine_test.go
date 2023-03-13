package follower

import (
	"context"
	commonmock "github.com/onflow/flow-go/engine/common/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

func TestFollowerEngine(t *testing.T) {
	suite.Run(t, new(EngineSuite))
}

type EngineSuite struct {
	suite.Suite

	net     *mocknetwork.Network
	con     *mocknetwork.Conduit
	me      *module.Local
	headers *storage.Headers
	core    *commonmock.FollowerCore

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error
	engine *Engine
}

func (s *EngineSuite) SetupTest() {

	s.net = mocknetwork.NewNetwork(s.T())
	s.con = mocknetwork.NewConduit(s.T())
	s.me = module.NewLocal(s.T())
	s.headers = storage.NewHeaders(s.T())
	s.core = commonmock.NewFollowerCore(s.T())

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

	proposals := []*messages.BlockProposal{messages.NewBlockProposal(&parent),
		messages.NewBlockProposal(&block)}

	originID := unittest.IdentifierFixture()

	var done sync.WaitGroup
	done.Add(len(proposals))
	for _, proposal := range proposals {
		s.core.On("OnBlockProposal", originID, proposal).Run(func(_ mock.Arguments) {
			done.Done()
		}).Return(nil).Once()
	}

	s.engine.OnSyncedBlocks(flow.Slashable[[]*messages.BlockProposal]{
		OriginID: originID,
		Message:  proposals,
	})
	unittest.AssertReturnsBefore(s.T(), done.Wait, time.Second)
}
