package approvals

import (
	"github.com/gammazero/workerpool"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
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
		base: base{
			workerPool:                           workerpool.New(4),
			assigner:                             s.assigner,
			state:                                s.state,
			headers:                              s.headers,
			verifier:                             s.sigVerifier,
			seals:                                s.sealsPL,
			approvalConduit:                      s.conduit,
			requestTracker:                       s.requestTracker,
			requiredApprovalsForSealConstruction: 5,
		},
		executedBlock: &s.Block,
		result:        s.IncorporatedResult.Result,
		resultID:      s.IncorporatedResult.Result.ID(),
	})
}

// TestChangeProcessingStatus_CachingToVerifying tests that state machine correctly performs transition from CachingApprovals to
// VerifyingApprovals state. After transition all caches approvals and results need to be applied to new state.
func (s *AssignmentCollectorStateMachineTestSuite) TestChangeProcessingStatus_CachingToVerifying() {
	require.Equal(s.T(), CachingApprovals, s.collector.ProcessingStatus())
	results := make([]*flow.IncorporatedResult, 0, 3)
	approvals := make([]*flow.ResultApproval, 0, s.Chunks.Len())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := range results {
			block := unittest.BlockHeaderWithParentFixture(&s.Block)
			s.blocks[block.ID()] = &block
			result := unittest.IncorporatedResult.Fixture(
				unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()),
				unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result),
			)
			results[i] = result
			require.NoError(s.T(), s.collector.ProcessIncorporatedResult(result))
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := range approvals {
			approval := unittest.ResultApprovalFixture(
				unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()),
				unittest.WithChunk(uint64(i)),
				unittest.WithBlockID(s.Block.ID()),
			)
			approvals[i] = approval
			require.NoError(s.T(), s.collector.ProcessApproval(approval))
		}
		wg.Done()
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
