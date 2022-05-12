package approvals

import (
	"github.com/gammazero/workerpool"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
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
	PublicKey           *module.PublicKey                  // public key used to mock signature verifications
	SigHasher           hash.Hasher                        // used to verify signatures
	IncorporatedResult  *flow.IncorporatedResult
}

func (s *BaseApprovalsTestSuite) SetupTest() {
	s.ParentBlock = unittest.BlockHeaderFixture()
	s.Block = unittest.BlockHeaderWithParentFixture(&s.ParentBlock)
	verifiers := make(flow.IdentifierList, 0)
	s.AuthorizedVerifiers = make(map[flow.Identifier]*flow.Identity)
	s.ChunksAssignment = chunks.NewAssignment()
	s.Chunks = unittest.ChunkListFixture(50, s.Block.ID())
	// mock public key to mock signature verifications
	s.PublicKey = &module.PublicKey{}

	// setup identities
	for j := 0; j < 5; j++ {
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		verifiers = append(verifiers, identity.NodeID)
		s.AuthorizedVerifiers[identity.NodeID] = identity
		// mock all verifier's valid signatures
		identity.StakingPubKey = s.PublicKey
	}
	s.SigHasher = crypto.NewBLSKMAC("test_tag")

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

	WorkerPool        *workerpool.WorkerPool
	Blocks            map[flow.Identifier]*flow.Header
	State             *protocol.State
	Snapshots         map[flow.Identifier]*protocol.Snapshot
	Headers           *storage.Headers
	Assigner          *module.ChunkAssigner
	SealsPL           *mempool.IncorporatedResultSeals
	Conduit           *mocknetwork.Conduit
	FinalizedAtHeight map[uint64]*flow.Header
	IdentitiesCache   map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block
	RequestTracker    *RequestTracker
}

func (s *BaseAssignmentCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.WorkerPool = workerpool.New(4)
	s.SealsPL = &mempool.IncorporatedResultSeals{}
	s.State = &protocol.State{}
	s.Assigner = &module.ChunkAssigner{}
	s.Conduit = &mocknetwork.Conduit{}
	s.Headers = &storage.Headers{}

	s.RequestTracker = NewRequestTracker(s.Headers, 1, 3)

	s.FinalizedAtHeight = make(map[uint64]*flow.Header)
	s.FinalizedAtHeight[s.ParentBlock.Height] = &s.ParentBlock
	s.FinalizedAtHeight[s.Block.Height] = &s.Block

	// setup blocks cache for protocol state
	s.Blocks = make(map[flow.Identifier]*flow.Header)
	s.Blocks[s.ParentBlock.ID()] = &s.ParentBlock
	s.Blocks[s.Block.ID()] = &s.Block
	s.Blocks[s.IncorporatedBlock.ID()] = &s.IncorporatedBlock
	s.Snapshots = make(map[flow.Identifier]*protocol.Snapshot)

	// setup identities for each block
	s.IdentitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)
	s.IdentitiesCache[s.IncorporatedResult.Result.BlockID] = s.AuthorizedVerifiers

	s.Assigner.On("Assign", mock.Anything, mock.Anything).Return(func(result *flow.ExecutionResult, blockID flow.Identifier) *chunks.Assignment {
		return s.ChunksAssignment
	}, func(result *flow.ExecutionResult, blockID flow.Identifier) error { return nil })

	s.Headers.On("ByBlockID", mock.Anything).Return(func(blockID flow.Identifier) *flow.Header {
		return s.Blocks[blockID]
	}, func(blockID flow.Identifier) error {
		_, found := s.Blocks[blockID]
		if found {
			return nil
		} else {
			return realstorage.ErrNotFound
		}
	})
	s.Headers.On("ByHeight", mock.Anything).Return(
		func(height uint64) *flow.Header {
			if block, found := s.FinalizedAtHeight[height]; found {
				return block
			} else {
				return nil
			}
		},
		func(height uint64) error {
			_, found := s.FinalizedAtHeight[height]
			if !found {
				return realstorage.ErrNotFound
			}
			return nil
		},
	)

	s.State.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if snapshot, found := s.Snapshots[blockID]; found {
				return snapshot
			}
			if block, found := s.Blocks[blockID]; found {
				snapshot := unittest.StateSnapshotForKnownBlock(block, s.IdentitiesCache[blockID])
				s.Snapshots[blockID] = snapshot
				return snapshot
			}
			return unittest.StateSnapshotForUnknownBlock()
		},
	)

	s.SealsPL.On("Size").Return(uint(0)).Maybe()                       // for metrics
	s.SealsPL.On("PruneUpToHeight", mock.Anything).Return(nil).Maybe() // noop on pruning
}

func (s *BaseAssignmentCollectorTestSuite) MarkFinalized(block *flow.Header) {
	s.FinalizedAtHeight[block.Height] = block
}

func (s *BaseAssignmentCollectorTestSuite) TearDownTest() {
	// Without this line we are risking running into weird situations where one test has finished but there are active workers
	// that are executing some work on the shared pool. Need to ensure that all pending work has been executed before
	// starting next test.
	s.WorkerPool.StopWait()
}
