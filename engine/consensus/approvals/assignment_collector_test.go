package approvals

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	module "github.com/onflow/flow-go/module/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestAssignmentCollector(t *testing.T) {
	suite.Run(t, new(AssignmentCollectorTestSuite))

}

type AssignmentCollectorTestSuite struct {
	suite.Suite
	verID               flow.Identifier
	chunks              flow.ChunkList
	chunkAssignment     *chunks.Assignment // assignment for given execution result
	authorizedVerifiers map[flow.Identifier]struct{}
	incorporatedResult  *flow.IncorporatedResult

	state    *protocol.State
	assigner *module.ChunkAssigner
	sealsPL  *mempool.IncorporatedResultSeals

	collector *AssignmentCollector
}

func (s *AssignmentCollectorTestSuite) SetupTest() {
	s.sealsPL = &mempool.IncorporatedResultSeals{}

	blockID := unittest.IdentifierFixture()
	verifiers := make(flow.IdentifierList, 0)
	s.authorizedVerifiers = make(map[flow.Identifier]struct{})
	s.chunkAssignment = chunks.NewAssignment()
	s.chunks = unittest.ChunkListFixture(50, blockID)

	for j := 0; j < 5; j++ {
		id := unittest.IdentifierFixture()
		verifiers = append(verifiers, id)
		s.authorizedVerifiers[id] = struct{}{}
	}

	for _, chunk := range s.chunks {
		s.chunkAssignment.Add(chunk, verifiers)
	}

	s.verID = verifiers[0]
	result := unittest.ExecutionResultFixture()
	result.BlockID = blockID
	result.Chunks = s.chunks
	s.incorporatedResult = unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(result))

	//collector := NewAssignmentCollector(s.incorporatedResult.Result.ID(), s.state, s.assigner, s.sealsPL, s.sigVerifier, 3)
}

func (s *AssignmentCollectorTestSuite) TestProcessApproval_ValidApproval() {
	//approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunks[0].Index), unittest.WithApproverID(s.verID))
	//err := collector.ProcessApproval(approval)
	//require.NoError(s.T(), err)
}
