package approvals

import (
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type BaseApprovalsTestSuite struct {
	suite.Suite

	Block                         flow.Header
	VerID                         flow.Identifier
	Chunks                        flow.ChunkList
	ChunksAssignment              *chunks.Assignment
	AuthorizedVerifiers           map[flow.Identifier]struct{}
	AuthorizedVerifiersIdentities map[flow.Identifier]*flow.Identity
	IncorporatedResult            *flow.IncorporatedResult
}

func (s *BaseApprovalsTestSuite) SetupTest() {
	s.Block = unittest.BlockHeaderFixture()
	verifiers := make(flow.IdentifierList, 0)
	s.AuthorizedVerifiers = make(map[flow.Identifier]struct{})
	s.AuthorizedVerifiersIdentities = make(map[flow.Identifier]*flow.Identity)
	s.ChunksAssignment = chunks.NewAssignment()
	s.Chunks = unittest.ChunkListFixture(50, s.Block.ID())

	for j := 0; j < 5; j++ {
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		verifiers = append(verifiers, identity.NodeID)
		s.AuthorizedVerifiersIdentities[identity.NodeID] = identity
		s.AuthorizedVerifiers[identity.NodeID] = struct{}{}
	}

	for _, chunk := range s.Chunks {
		s.ChunksAssignment.Add(chunk, verifiers)
	}

	s.VerID = verifiers[0]
	result := unittest.ExecutionResultFixture()
	result.BlockID = s.Block.ID()
	result.Chunks = s.Chunks
	s.IncorporatedResult = unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(result))
}
