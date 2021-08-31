package approvals

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// AssignmentCollectorStateMachineTestSuite is a test suite for testing AssignmentCollectorStateMachine. Contains a minimal set of
// helper mocks to test the behavior.
type AssignmentCollectorStateMachineTestSuite struct {
	BaseAssignmentCollectorTestSuite
	collector *AssignmentCollectorStateMachine
}

func TestAssignmentCollectorStateMachine(t *testing.T) {
	suite.Run(t, new(AssignmentCollectorStateMachineTestSuite))
}

func (s *AssignmentCollectorStateMachineTestSuite) SetupTest() {
	s.BaseAssignmentCollectorTestSuite.SetupTest()

	s.collector = NewAssignmentCollectorStateMachine(AssignmentCollectorBase{
		workerPool:                           workerpool.New(4),
		assigner:                             s.Assigner,
		state:                                s.State,
		headers:                              s.Headers,
		verifier:                             s.SigVerifier,
		seals:                                s.SealsPL,
		approvalConduit:                      s.Conduit,
		requestTracker:                       s.RequestTracker,
		requiredApprovalsForSealConstruction: 5,
		executedBlock:                        &s.Block,
		result:                               s.IncorporatedResult.Result,
		resultID:                             s.IncorporatedResult.Result.ID(),
	})
}

// TestChangeProcessingStatus_CachingToVerifying tests that state machine correctly performs transition from CachingApprovals to
// VerifyingApprovals state. After transition all caches approvals and results need to be applied to new state.
func (s *AssignmentCollectorStateMachineTestSuite) TestChangeProcessingStatus_CachingToVerifying() {
	require.Equal(s.T(), CachingApprovals, s.collector.ProcessingStatus())
	results := make([]*flow.IncorporatedResult, 3)

	s.SigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for i := range results {
		block := unittest.BlockHeaderWithParentFixture(&s.Block)
		s.Blocks[block.ID()] = &block
		result := unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()),
			unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result),
		)
		results[i] = result
	}

	approvals := make([]*flow.ResultApproval, s.Chunks.Len())

	for i := range approvals {
		approval := unittest.ResultApprovalFixture(
			unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()),
			unittest.WithChunk(uint64(i)),
			unittest.WithApproverID(s.VerID),
			unittest.WithBlockID(s.Block.ID()),
		)
		approvals[i] = approval
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, result := range results {
			require.NoError(s.T(), s.collector.ProcessIncorporatedResult(result))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, approval := range approvals {
			require.NoError(s.T(), s.collector.ProcessApproval(approval))
		}
	}()

	err := s.collector.ChangeProcessingStatus(CachingApprovals, VerifyingApprovals)
	require.NoError(s.T(), err)
	require.Equal(s.T(), VerifyingApprovals, s.collector.ProcessingStatus())

	wg.Wait()

	// give some time to process on worker pool
	time.Sleep(1 * time.Second)
	// need to check if collector has processed cached items
	verifyingCollector, ok := s.collector.atomicLoadCollector().(*VerifyingAssignmentCollector)
	require.True(s.T(), ok)

	for _, ir := range results {
		collector, ok := verifyingCollector.collectors[ir.IncorporatedBlockID]
		require.True(s.T(), ok)

		for _, approval := range approvals {
			chunkCollector := collector.chunkCollectors[approval.Body.ChunkIndex]
			signed := chunkCollector.chunkApprovals.HasSigned(approval.Body.ApproverID)
			require.True(s.T(), signed)
		}
	}
}

// TestChangeProcessingStatus_InvalidTransition tries to perform transition from caching to verifying status
// but with underlying orphan status. This should result in sentinel error ErrInvalidCollectorStateTransition.
func (s *AssignmentCollectorStateMachineTestSuite) TestChangeProcessingStatus_InvalidTransition() {
	// first change status to orphan
	err := s.collector.ChangeProcessingStatus(CachingApprovals, Orphaned)
	require.NoError(s.T(), err)
	require.Equal(s.T(), Orphaned, s.collector.ProcessingStatus())
	// then try to perform transition from caching to verifying
	err = s.collector.ChangeProcessingStatus(CachingApprovals, VerifyingApprovals)
	require.Error(s.T(), err)
	require.True(s.T(), errors.Is(err, ErrDifferentCollectorState))
}
