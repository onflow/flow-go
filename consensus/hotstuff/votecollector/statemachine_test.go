package votecollector

import (
	"errors"
	"fmt"
	"github.com/gammazero/workerpool"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestStateMachine(t *testing.T) {
	suite.Run(t, new(StateMachineTestSuite))
}

var factoryError = errors.New("factory error")

type StateMachineTestSuite struct {
	suite.Suite

	view             uint64
	notifier         *mocks.Consumer
	workerPool       *workerpool.WorkerPool
	factoryMethod    VerifyingVoteProcessorFactory
	mockedProcessors map[flow.Identifier]*mocks.VerifyingVoteProcessor
	collector        *VoteCollector
}

func (s *StateMachineTestSuite) TearDownTest() {
	// Without this line we are risking running into weird situations where one test has finished but there are active workers
	// that are executing some work on the shared pool. Need to ensure that all pending work has been executed before
	// starting next test.
	s.workerPool.StopWait()
}

func (s *StateMachineTestSuite) SetupTest() {
	s.view = 1000
	s.mockedProcessors = make(map[flow.Identifier]*mocks.VerifyingVoteProcessor)

	s.factoryMethod = func(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
		if processor, found := s.mockedProcessors[block.BlockID]; found {
			return processor, nil
		}
		return nil, fmt.Errorf("mocked processor %v not found: %w", block.BlockID, factoryError)
	}

	s.workerPool = workerpool.New(4)
	s.collector = NewStateMachine(s.view, unittest.Logger(), s.workerPool, s.notifier, s.factoryMethod)
}

// prepareMockedProcessor prepares a mocked processor and stores it in map, later it will be used
// to mock behavior of verifying vote processor.
func (s *StateMachineTestSuite) prepareMockedProcessor(block *model.Block) *mocks.VerifyingVoteProcessor {
	processor := &mocks.VerifyingVoteProcessor{}
	processor.On("Block").Return(func() *model.Block {
		return block
	}).Maybe()
	processor.On("Status").Return(hotstuff.VoteCollectorStatusVerifying)
	s.mockedProcessors[block.BlockID] = processor
	return processor
}

func (s *StateMachineTestSuite) TestStatus_StateTransitions() {
	block := helper.MakeBlock(s.T(), helper.WithBlockView(s.view))
	proposal := helper.MakeProposal(s.T(), helper.WithBlock(block))
	s.prepareMockedProcessor(block)

	// by default, we should create in caching status
	require.Equal(s.T(), hotstuff.VoteCollectorStatusCaching, s.collector.Status())

	// after processing block we should get into verifying status
	err := s.collector.ProcessBlock(proposal)
	require.NoError(s.T(), err)
	require.Equal(s.T(), hotstuff.VoteCollectorStatusVerifying, s.collector.Status())

	// after submitting double proposal we should transfer into invalid state
	err = s.collector.ProcessBlock(helper.MakeProposal(s.T(),
		helper.WithBlock(
			helper.MakeBlock(s.T(), helper.WithBlockView(s.view)))))
	require.NoError(s.T(), err)
	require.Equal(s.T(), hotstuff.VoteCollectorStatusInvalid, s.collector.Status())
}

// TestAddVote_VerifyingState tests that AddVote correctly process valid and invalid votes as well
// as repeated, invalid and double votes in verifying state
func (s *StateMachineTestSuite) TestAddVote_VerifyingState() {
	block := helper.MakeBlock(s.T(), helper.WithBlockView(s.view))
	proposal := helper.MakeProposal(s.T(), helper.WithBlock(block))
	processor := s.prepareMockedProcessor(block)
	err := s.collector.ProcessBlock(proposal)
	require.NoError(s.T(), err)
	s.T().Run("add-valid-vote", func(t *testing.T) {
		vote := unittest.VoteForBlockFixture(block)
		processor.On("Process", vote).Return(nil).Once()
		err := s.collector.AddVote(vote)
		require.NoError(t, err)
		processor.AssertCalled(t, "Process", vote)
	})
	s.T().Run("add-double-vote", func(t *testing.T) {
		//vote := unittest.VoteForBlockFixture(block)
		//processor.On("Process", vote).Return(nil)
		//err := s.collector.AddVote(vote)
		//require.NoError(t, err)
		//processor.AssertCalled(t, "Process", vote)
	})
	s.T().Run("add-repeated-vote", func(t *testing.T) {
		vote := unittest.VoteForBlockFixture(block)
		processor.On("Process", vote).Return(nil).Once()
		err := s.collector.AddVote(vote)
		require.NoError(t, err)

		// calling with same vote should exit early without error and don't do any extra processing
		err = s.collector.AddVote(vote)
		require.NoError(t, err)

		processor.AssertCalled(t, "Process", vote)
	})
	s.T().Run("add-incompatible-view-vote", func(t *testing.T) {
		vote := unittest.VoteForBlockFixture(block, unittest.WithVoteView(s.view+1))
		err := s.collector.AddVote(vote)
		require.ErrorIs(t, err, VoteForIncompatibleViewError)
	})
	s.T().Run("add-invalid-vote", func(t *testing.T) {

	})
}
