package approvals

import (
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// BaseApprovalsTestSuite is a base suite for testing approvals processing related functionality
// At nutshell generates mock data that can be used to create approvals and provides all needed
// data to validate those approvals for respected execution result.
type BaseApprovalsTestSuite struct {
	suite.Suite

	Block               flow.Header     // candidate for sealing
	VerID               flow.Identifier // for convenience, node id of first verifier
	Chunks              flow.ChunkList  // list of chunks of execution result
	ChunksAssignment    *chunks.Assignment
	AuthorizedVerifiers map[flow.Identifier]*flow.Identity // map of authorized verifier identities for execution result
	IncorporatedResult  *flow.IncorporatedResult
}

func (s *BaseApprovalsTestSuite) SetupTest() {
	s.Block = unittest.BlockHeaderFixture()
	verifiers := make(flow.IdentifierList, 0)
	s.AuthorizedVerifiers = make(map[flow.Identifier]*flow.Identity)
	s.ChunksAssignment = chunks.NewAssignment()
	s.Chunks = unittest.ChunkListFixture(50, s.Block.ID())

	// setup identities
	for j := 0; j < 5; j++ {
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		verifiers = append(verifiers, identity.NodeID)
		s.AuthorizedVerifiers[identity.NodeID] = identity
	}

	// create assignment
	for _, chunk := range s.Chunks {
		s.ChunksAssignment.Add(chunk, verifiers)
	}

	s.VerID = verifiers[0]
	result := unittest.ExecutionResultFixture()
	result.BlockID = s.Block.ID()
	result.Chunks = s.Chunks
	// compose incorporated result
	s.IncorporatedResult = unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(result))
}
