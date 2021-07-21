package approvals

import (
	"github.com/gammazero/workerpool"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	realstorage "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// BaseApprovalsTestSuite is a base suite for testing approvals processing related functionality
// At nutshell generates mock data that can be used to create approvals and provides all needed
// data to validate those approvals for respected execution result.
type BaseApprovalsTestSuite struct {
	suite.Suite

	ParentBlock         flow.Header     // parent of sealing candidate
	Block               flow.Header     // candidate for sealing
	IncorporatedBlock   flow.Header     // block that incorporated result
	VerID               flow.Identifier // for convenience, node id of first verifier
	Chunks              flow.ChunkList  // list of chunks of execution result
	ChunksAssignment    *chunks.Assignment
	AuthorizedVerifiers map[flow.Identifier]*flow.Identity // map of authorized verifier identities for execution result
	IncorporatedResult  *flow.IncorporatedResult
}

func (s *BaseApprovalsTestSuite) SetupTest() {
	s.ParentBlock = unittest.BlockHeaderFixture()
	s.Block = unittest.BlockHeaderWithParentFixture(&s.ParentBlock)
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

	s.IncorporatedBlock = unittest.BlockHeaderWithParentFixture(&s.Block)

	// compose incorporated result
	s.IncorporatedResult = unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithResult(result),
		unittest.IncorporatedResult.WithIncorporatedBlockID(s.IncorporatedBlock.ID()))
}

// BaseAssignmentCollectorTestSuite is a base suite for testing assignment collectors, contains mocks for all
// classes that are used in base assignment collector and can be reused in different test suites.
type BaseAssignmentCollectorTestSuite struct {
	BaseApprovalsTestSuite

	workerPool      *workerpool.WorkerPool
	blocks          map[flow.Identifier]*flow.Header
	state           *protocol.State
	headers         *storage.Headers
	assigner        *module.ChunkAssigner
	sealsPL         *mempool.IncorporatedResultSeals
	sigVerifier     *module.Verifier
	conduit         *mocknetwork.Conduit
	identitiesCache map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block
	requestTracker  *RequestTracker
}

func (s *BaseAssignmentCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.workerPool = workerpool.New(4)
	s.sealsPL = &mempool.IncorporatedResultSeals{}
	s.state = &protocol.State{}
	s.assigner = &module.ChunkAssigner{}
	s.sigVerifier = &module.Verifier{}
	s.conduit = &mocknetwork.Conduit{}
	s.headers = &storage.Headers{}

	s.requestTracker = NewRequestTracker(s.headers, 1, 3)

	// setup blocks cache for protocol state
	s.blocks = make(map[flow.Identifier]*flow.Header)
	s.blocks[s.Block.ID()] = &s.Block
	s.blocks[s.IncorporatedBlock.ID()] = &s.IncorporatedBlock

	// setup identities for each block
	s.identitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)
	s.identitiesCache[s.IncorporatedResult.Result.BlockID] = s.AuthorizedVerifiers

	s.assigner.On("Assign", mock.Anything, mock.Anything).Return(func(result *flow.ExecutionResult, blockID flow.Identifier) *chunks.Assignment {
		return s.ChunksAssignment
	}, func(result *flow.ExecutionResult, blockID flow.Identifier) error { return nil })

	s.headers.On("ByBlockID", mock.Anything).Return(func(blockID flow.Identifier) *flow.Header {
		return s.blocks[blockID]
	}, func(blockID flow.Identifier) error {
		_, found := s.blocks[blockID]
		if found {
			return nil
		} else {
			return realstorage.ErrNotFound
		}
	})

	s.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if block, found := s.blocks[blockID]; found {
				return unittest.StateSnapshotForKnownBlock(block, s.identitiesCache[blockID])
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)
}
