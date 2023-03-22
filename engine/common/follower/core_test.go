package follower

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFollowerCore(t *testing.T) {
	suite.Run(t, new(CoreSuite))
}

type CoreSuite struct {
	suite.Suite

	originID       flow.Identifier
	finalizedBlock *flow.Header
	state          *protocol.FollowerState
	follower       *module.HotStuffFollower
	sync           *module.BlockRequester
	validator      *hotstuff.Validator

	core *Core
}

func (s *CoreSuite) SetupTest() {
	s.state = protocol.NewFollowerState(s.T())
	s.follower = module.NewHotStuffFollower(s.T())
	s.validator = hotstuff.NewValidator(s.T())
	s.sync = module.NewBlockRequester(s.T())

	s.originID = unittest.IdentifierFixture()
	s.finalizedBlock = unittest.BlockHeaderFixture()

	metrics := metrics.NewNoopCollector()
	s.core = NewCore(
		unittest.Logger(),
		metrics,
		s.state,
		s.follower,
		s.validator,
		s.sync,
		trace.NewNoopTracer())

	s.core.OnFinalizedBlock(s.finalizedBlock)
}

// TestProcessingSingleBlock tests processing a range with length 1, it must result in block being validated and added to cache.
func (s *CoreSuite) TestProcessingSingleBlock() {
	block := unittest.BlockWithParentFixture(s.finalizedBlock)

	// incoming block has to be validated
	s.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header)).Return(nil).Once()

	err := s.core.OnBlockRange(s.originID, []*flow.Block{block})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), s.core.pendingCache.Peek(block.ID()))
}

// TestAddFinalizedBlock tests that adding block below finalized height results in processing it, but since cache was pruned
// to finalized view, it must be rejected by it.
func (s *CoreSuite) TestAddFinalizedBlock() {
	block := unittest.BlockFixture()
	block.Header.View = s.finalizedBlock.View - 1 // block is below finalized view

	// incoming block has to be validated
	s.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header)).Return(nil).Once()

	err := s.core.OnBlockRange(s.originID, []*flow.Block{&block})
	require.NoError(s.T(), err)
	require.Nil(s.T(), s.core.pendingCache.Peek(block.ID()))
}
