package approvals

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// CachingAssignmentCollectorTestSuite is a test suite for testing CachingAssignmentCollector. Contains a minimal set of
// helper mocks to test the behavior.
type CachingAssignmentCollectorTestSuite struct {
	suite.Suite

	executedBlock flow.Header
	result        *flow.ExecutionResult
	collector     *CachingAssignmentCollector
}

func TestCachingAssignmentCollector(t *testing.T) {
	suite.Run(t, new(CachingAssignmentCollectorTestSuite))
}

func (s *CachingAssignmentCollectorTestSuite) SetupTest() {
	s.executedBlock = unittest.BlockHeaderFixture()
	s.result = unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
		result.BlockID = s.executedBlock.ID()
	})
	s.collector = NewCachingAssignmentCollector(AssignmentCollectorBase{
		executedBlock: &s.executedBlock,
		result:        s.result,
		resultID:      s.result.ID(),
	})
}

// TestCheckEmergencySealing tests that emergency sealing is no-op
func (s *CachingAssignmentCollectorTestSuite) TestCheckEmergencySealing() {
	// should be no-op
	err := s.collector.CheckEmergencySealing(nil, 0)
	require.NoError(s.T(), err)
}

// TestProcessApproval tests that collector caches approval when requested to process it
func (s *CachingAssignmentCollectorTestSuite) TestProcessApproval() {
	s.T().Parallel()

	approval := unittest.ResultApprovalFixture()
	err := s.collector.ProcessApproval(approval)
	require.Error(s.T(), err)

	approval = unittest.ResultApprovalFixture(unittest.WithExecutionResultID(s.result.ID()))
	err = s.collector.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err))

	var expected []*flow.ResultApproval
	for i := 0; i < 5; i++ {
		approval := unittest.ResultApprovalFixture(
			unittest.WithBlockID(s.executedBlock.ID()),
			unittest.WithExecutionResultID(s.result.ID()),
			unittest.WithChunk(uint64(i)))
		err := s.collector.ProcessApproval(approval)
		require.NoError(s.T(), err)
		expected = append(expected, approval)
	}
	require.ElementsMatch(s.T(), expected, s.collector.GetApprovals())
}

// TestProcessIncorporatedResult tests that collector caches result when requested to processes flow.IncorporatedResult
func (s *CachingAssignmentCollectorTestSuite) TestProcessIncorporatedResult() {
	s.T().Parallel()

	// processing invalid result should error
	err := s.collector.ProcessIncorporatedResult(unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithResult(unittest.ExecutionResultFixture()),
	))
	require.Error(s.T(), err)

	// processing valid IR should result in no error
	var expected []*flow.IncorporatedResult
	for i := 0; i < 5; i++ {
		IR := unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithResult(s.result),
		)
		err := s.collector.ProcessIncorporatedResult(IR)
		require.NoError(s.T(), err)
		expected = append(expected, IR)
	}

	require.ElementsMatch(s.T(), expected, s.collector.GetIncorporatedResults())
}

func (s *CachingAssignmentCollectorTestSuite) TestProcessingStatus() {
	require.Equal(s.T(), CachingApprovals, s.collector.ProcessingStatus())
}

// TestRequestMissingApprovals tests that requesting missing approvals is no-op
func (s *CachingAssignmentCollectorTestSuite) TestRequestMissingApprovals() {
	// should be no-op
	_, err := s.collector.RequestMissingApprovals(nil, 0)
	require.NoError(s.T(), err)
}
