package approvals_test

import (
	"sync"
	"testing"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/engine/consensus/approvals/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// AssignmentCollectorStateMachineTestSuite is a test suite for testing AssignmentCollectorStateMachine. Contains a minimal set of
// helper mocks to test the behavior.
type AssignmentCollectorStateMachineTestSuite struct {
	testutil.BaseAssignmentCollectorTestSuite
	collector *approvals.AssignmentCollectorStateMachine
}

func TestAssignmentCollectorStateMachine(t *testing.T) {
	suite.Run(t, new(AssignmentCollectorStateMachineTestSuite))
}

func (s *AssignmentCollectorStateMachineTestSuite) SetupTest() {
	s.BaseAssignmentCollectorTestSuite.SetupTest()

	ac, err := approvals.NewAssignmentCollectorBase(
		zerolog.Nop(),
		workerpool.New(4),
		s.IncorporatedResult.Result,
		s.State,
		s.Headers,
		s.Assigner,
		s.SealsPL,
		s.SigHasher,
		s.Conduit,
		s.RequestTracker,
		5,
	)
	require.NoError(s.T(), err)

	s.collector = approvals.NewAssignmentCollectorStateMachine(ac)
}

// TestChangeProcessingStatus_CachingToVerifying tests that state machine correctly performs transition from CachingApprovals to
// VerifyingApprovals state. After transition all caches approvals and results need to be applied to new state.
func (s *AssignmentCollectorStateMachineTestSuite) TestChangeProcessingStatus_CachingToVerifying() {
	require.Equal(s.T(), approvals.CachingApprovals, s.collector.ProcessingStatus())
	results := make([]*flow.IncorporatedResult, 3)

	s.PublicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for i := range results {
		block := unittest.BlockHeaderWithParentFixture(s.Block)
		s.Blocks[block.ID()] = block
		result := unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()),
			unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result),
		)
		results[i] = result
	}

	approvs := make([]*flow.ResultApproval, s.Chunks.Len())

	for i := range approvs {
		approval := unittest.ResultApprovalFixture(
			unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()),
			unittest.WithChunk(uint64(i)),
			unittest.WithApproverID(s.VerID),
			unittest.WithBlockID(s.Block.ID()),
		)
		approvs[i] = approval
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		for _, result := range results {
			require.NoError(s.T(), s.collector.ProcessIncorporatedResult(result))
		}
	})
	wg.Go(func() {
		for _, approval := range approvs {
			require.NoError(s.T(), s.collector.ProcessApproval(approval))
		}
	})

	err := s.collector.ChangeProcessingStatus(approvals.CachingApprovals, approvals.VerifyingApprovals)
	require.NoError(s.T(), err)
	require.Equal(s.T(), approvals.VerifyingApprovals, s.collector.ProcessingStatus())

	wg.Wait()

	// give some time to process on worker pool
	time.Sleep(1 * time.Second)
	// need to check if collector has processed cached items
	verifyingCollector, ok := s.collector.GetCollectorState().(*approvals.VerifyingAssignmentCollector)
	require.True(s.T(), ok)

	for _, ir := range results {
		require.True(s.T(), verifyingCollector.HasIncorporatedResult(ir.IncorporatedBlockID))

		for _, approval := range approvs {
			signed := verifyingCollector.HasApprovalBeenProcessed(ir.IncorporatedBlockID, approval.Body.ChunkIndex, approval.Body.ApproverID)
			require.True(s.T(), signed)
		}
	}
}

// TestChangeProcessingStatus_InvalidTransition tries to perform transition from caching to verifying status
// but with underlying orphan status. This should result in sentinel error ErrInvalidCollectorStateTransition.
func (s *AssignmentCollectorStateMachineTestSuite) TestChangeProcessingStatus_InvalidTransition() {
	// first change status to orphan
	err := s.collector.ChangeProcessingStatus(approvals.CachingApprovals, approvals.Orphaned)
	require.NoError(s.T(), err)
	require.Equal(s.T(), approvals.Orphaned, s.collector.ProcessingStatus())
	// then try to perform transition from caching to verifying
	err = s.collector.ChangeProcessingStatus(approvals.CachingApprovals, approvals.VerifyingApprovals)
	require.ErrorIs(s.T(), err, approvals.ErrDifferentCollectorState)
}
