package chunks

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	protocolMock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/seed"
	"github.com/onflow/flow-go/utils/unittest"
)

// PublicAssignmentTestSuite contains tests against methods of the ChunkAssigner scheme
type PublicAssignmentTestSuite struct {
	suite.Suite
}

// Setup test with n verification nodes
func (a *PublicAssignmentTestSuite) SetupTest(n int) (*flow.Header, *protocolMock.Snapshot, *protocolMock.State) {
	nodes := make([]flow.Role, 0)
	for i := 1; i < n; i++ {
		nodes = append(nodes, flow.RoleVerification)
	}
	participants, _, _ := unittest.CreateNParticipantsWithMyRole(flow.RoleVerification, nodes...)

	// setup protocol state
	block, snapshot, state, _ := unittest.FinalizedProtocolStateWithParticipants(participants)
	head := block.Header

	return head, snapshot, state
}

// TestAssignment invokes all the tests in this test suite
func TestAssignment(t *testing.T) {
	suite.Run(t, new(PublicAssignmentTestSuite))
}

// TestByNodeID evaluates the correctness of ByNodeID method of ChunkAssigner
func (a *PublicAssignmentTestSuite) TestByNodeID() {
	size := 5
	// creates ids and twice chunks of the ids
	ids := unittest.IdentityListFixture(size)
	chunks := a.CreateChunks(2*size, a.T())
	assignment := chmodels.NewAssignment()

	// assigns two chunks to each verifier node
	// j keeps track of chunks
	j := 0
	for i := 0; i < size; i++ {
		c, ok := chunks.ByIndex(uint64(j))
		require.True(a.T(), ok, "chunk out of range requested")
		assignment.Add(c, append(assignment.Verifiers(c), ids[i].NodeID))
		j++
		c, ok = chunks.ByIndex(uint64(j))
		require.True(a.T(), ok, "chunk out of range requested")
		assignment.Add(c, append(assignment.Verifiers(c), ids[i].NodeID))
	}

	// evaluating the chunk assignment
	// each verifier should have two certain chunks based on the assignment
	// j keeps track of chunks
	j = 0
	for i := 0; i < size; i++ {
		assignedChunks := assignment.ByNodeID(ids[i].NodeID)
		require.Len(a.T(), assignedChunks, 2)
		c, ok := chunks.ByIndex(uint64(j))
		require.True(a.T(), ok, "chunk out of range requested")
		require.Contains(a.T(), assignedChunks, c.Index)
		j++
		c, ok = chunks.ByIndex(uint64(j))
		require.True(a.T(), ok, "chunk out of range requested")
		require.Contains(a.T(), assignedChunks, c.Index)
	}

}

// TestAssignDuplicate tests assign Add duplicate verifiers
func (a *PublicAssignmentTestSuite) TestAssignDuplicate() {
	size := 5
	// creates ids and twice chunks of the ids
	var ids flow.IdentityList = unittest.IdentityListFixture(size)
	chunks := a.CreateChunks(2, a.T())
	assignment := chmodels.NewAssignment()

	// assigns first chunk to non-duplicate list of verifiers
	c, ok := chunks.ByIndex(uint64(0))
	require.True(a.T(), ok, "chunk out of range requested")
	assignment.Add(c, ids.NodeIDs())
	require.Len(a.T(), assignment.Verifiers(c), size)

	// duplicates first verifier, hence size increases by 1
	ids = append(ids, ids[0])
	require.Len(a.T(), ids, size+1)
	// assigns second chunk to a duplicate list of verifiers
	c, ok = chunks.ByIndex(uint64(1))
	require.True(a.T(), ok, "chunk out of range requested")
	assignment.Add(c, ids.NodeIDs())
	// should be size not size + 1
	require.Len(a.T(), assignment.Verifiers(c), size)
}

// TestPermuteEntirely tests permuting an entire IdentityList against
// randomness and deterministicity
func (a *PublicAssignmentTestSuite) TestPermuteEntirely() {
	_, snapshot, state := a.SetupTest(10)

	// create a assigner object with alpha = 10
	assigner, err := NewChunkAssigner(10, state)
	require.NoError(a.T(), err)

	// create seed
	seed := a.GetSeed(a.T())

	snapshot.On("RandomSource").Return(seed, nil)

	// creates random ids
	count := 10
	var idList flow.IdentityList = unittest.IdentityListFixture(count)
	var ids flow.IdentifierList = idList.NodeIDs()
	original := make(flow.IdentifierList, count)
	copy(original, ids)

	// Randomness:
	rng1, err := assigner.rngByBlockID(snapshot)
	require.NoError(a.T(), err)
	err = rng1.Shuffle(len(ids), ids.Swap)
	require.NoError(a.T(), err)

	// permutation should not change length of the list
	require.Len(a.T(), ids, count)

	// list should be permuted
	require.NotEqual(a.T(), ids, original)

	// Deterministiciy:
	// shuffling same list with the same seed should generate the same permutation
	rng2, err := assigner.rngByBlockID(snapshot)
	require.NoError(a.T(), err)
	// permutes original list with the same seed
	err = rng2.Shuffle(len(original), original.Swap)
	require.NoError(a.T(), err)
	require.Equal(a.T(), ids, original)
}

// TestPermuteSublist tests permuting an a sublist of an
// IdentityList against randomness and deterministicity
func (a *PublicAssignmentTestSuite) TestPermuteSublist() {
	_, snapshot, state := a.SetupTest(10)

	// create a assigner object with alpha = 10
	assigner, err := NewChunkAssigner(10, state)
	require.NoError(a.T(), err)

	// create seed
	seed := a.GetSeed(a.T())
	snapshot.On("RandomSource").Return(seed, nil)

	// creates random ids
	count := 10
	subset := 4

	var idList flow.IdentityList = unittest.IdentityListFixture(count)
	var ids flow.IdentifierList = idList.NodeIDs()
	original := make([]flow.Identifier, count)
	copy(original, ids)

	// Randomness:
	rng1, err := assigner.rngByBlockID(snapshot)
	require.NoError(a.T(), err)
	err = rng1.Samples(len(ids), subset, ids.Swap)
	require.NoError(a.T(), err)

	// permutation should not change length of the list
	require.Len(a.T(), ids, count)

	// the initial subset of the list that is permuted should
	// be different than the original
	require.NotEqual(a.T(), ids[:subset], original[:subset])
}

// TestDeterministicy evaluates deterministic behavior of chunk assignment when
// chunks, random generator, and nodes are the same
func (a *PublicAssignmentTestSuite) TestDeterministicy() {
	head, snapshot, state := a.SetupTest(10)

	c := 10          // keeps number of chunks
	n := 10          // keeps number of verifier nodes
	alpha := uint(1) // each chunk requires alpha verifiers

	// create seed
	result := a.CreateResult(head, c, a.T())
	seed := a.GetSeed(a.T())
	snapshot.On("RandomSource").Return(seed, nil)

	// creates two set of the same nodes
	nodes1 := unittest.IdentityListFixture(n)
	nodes2 := make([]*flow.Identity, n)
	require.Equal(a.T(), copy(nodes2, nodes1), n)

	// chunk assignment of the first set
	snapshot.On("Identities", mock.Anything).Return(nodes1, nil).Once()
	a1, err := NewChunkAssigner(alpha, state)
	require.NoError(a.T(), err)
	p1, err := a1.Assign(result, head.ID())
	require.NoError(a.T(), err)

	// chunk assignment of the second set
	snapshot.On("Identities", mock.Anything).Return(nodes2, nil).Once()
	a2, err := NewChunkAssigner(alpha, state)
	require.NoError(a.T(), err)
	p2, err := a2.Assign(result, head.ID())
	require.NoError(a.T(), err)

	// list of nodes should get shuffled after public assignment
	// but it should contain same elements
	require.Equal(a.T(), p1, p2)
}

// TestChunkAssignmentOneToOne evaluates chunk assignment against
// several single chunk to single node assignment
func (a *PublicAssignmentTestSuite) TestChunkAssignmentOneToOne() {
	// covers an edge case assigning 1 chunk to a single verifier node
	a.ChunkAssignmentScenario(1, 1, 1)

	// assigning 10 chunks to one node
	a.ChunkAssignmentScenario(10, 1, 1)
	// assigning 10 chunks to 2 nodes
	// each chunk to one verifier
	a.ChunkAssignmentScenario(10, 2, 1)
	// each chunk to 2 verifiers
	a.ChunkAssignmentScenario(10, 2, 2)

	// assigning 10 chunks to 10 nodes
	// each chunk to one verifier
	a.ChunkAssignmentScenario(10, 10, 1)
	// each chunk to 6 verifiers
	a.ChunkAssignmentScenario(10, 10, 6)
	// each chunk to 9 verifiers
	a.ChunkAssignmentScenario(10, 10, 9)
}

// TestChunkAssignmentOneToMay evaluates chunk assignment
func (a *PublicAssignmentTestSuite) TestChunkAssignmentOneToMany() {
	//  against assigning 52 chunks to 7 nodes
	//  each chunk to 5 verifiers
	a.ChunkAssignmentScenario(52, 7, 5)
	//  against assigning 49 chunks to 9 nodes
	//  each chunk to 8 verifiers
	a.ChunkAssignmentScenario(52, 9, 8)
}

// ChunkAssignmentScenario is a test helper that creates chunkNum chunks, verNum verifiers
// and then assign each chunk to alpha randomly chosen verifiers
// it also evaluates that each chuck is assigned to alpha many unique verifier nodes
func (a *PublicAssignmentTestSuite) ChunkAssignmentScenario(chunkNum, verNum, alpha int) {
	head, snapshot, state := a.SetupTest(alpha)

	result := a.CreateResult(head, chunkNum, a.T())
	seed := a.GetSeed(a.T())
	snapshot.On("RandomSource").Return(seed, nil)

	// creates nodes and keeps a copy of them
	nodes := unittest.IdentityListFixture(verNum)
	original := make([]*flow.Identity, verNum)
	require.Equal(a.T(), copy(original, nodes), verNum)

	snapshot.On("Identities", mock.Anything).Return(nodes, nil).Once()
	a1, err := NewChunkAssigner(uint(alpha), state)
	require.NoError(a.T(), err)
	p1, err := a1.Assign(result, head.ID())
	require.NoError(a.T(), err)

	// list of nodes should get shuffled after public assignment
	require.ElementsMatch(a.T(), nodes, original)

	for _, chunk := range result.Chunks {
		// each chunk should be assigned to alpha verifiers
		require.Equal(a.T(), p1.Verifiers(chunk).Len(), alpha)
	}
}

func (a *PublicAssignmentTestSuite) TestCacheAssignment() {
	head, snapshot, state := a.SetupTest(3)

	result := a.CreateResult(head, 20, a.T())
	seed := a.GetSeed(a.T())
	snapshot.On("RandomSource").Return(seed, nil)

	// creates nodes and keeps a copy of them
	nodes := unittest.IdentityListFixture(5)
	assigner, err := NewChunkAssigner(3, state)
	require.NoError(a.T(), err)

	// initially cache should be empty
	require.Equal(a.T(), assigner.Size(), uint(0))

	// new assignment should be cached
	// random generators are stateful and we need to
	// generate a new one if we want to have the same
	// state
	snapshot.On("Identities", mock.Anything).Return(nodes, nil).Once()
	_, err = assigner.Assign(result, head.ID())
	require.NoError(a.T(), err)
	require.Equal(a.T(), assigner.Size(), uint(1))

	// repetitive assignment should not be cached
	_, err = assigner.Assign(result, head.ID())
	require.NoError(a.T(), err)
	require.Equal(a.T(), assigner.Size(), uint(1))

	// performs the assignment using a different seed
	// should results in a different new assignment
	// which should be cached
	otherResult := a.CreateResult(head, 20, a.T())

	_, err = assigner.Assign(otherResult, head.ID())
	require.NoError(a.T(), err)
	require.Equal(a.T(), assigner.Size(), uint(2))
}

// CreateChunk creates and returns num chunks. It only fills the Index part of
// chunks to make them distinct from each other.
func (a *PublicAssignmentTestSuite) CreateChunks(num int, t *testing.T) flow.ChunkList {
	list := flow.ChunkList{}
	for i := 0; i < num; i++ {
		// creates random state for each chunk
		// to provide random ordering after sorting
		var state flow.StateCommitment
		_, err := rand.Read(state[:])
		require.NoError(t, err)

		blockID := unittest.IdentifierFixture()

		// creates chunk
		c := &flow.Chunk{
			Index: uint64(i),
			ChunkBody: flow.ChunkBody{
				StartState: state,
				BlockID:    blockID,
			},
		}
		list.Insert(c)
	}
	require.Equal(a.T(), num, list.Len())
	return list
}

func (a *PublicAssignmentTestSuite) CreateResult(head *flow.Header, num int, t *testing.T) *flow.ExecutionResult {
	list := a.CreateChunks(5, a.T())
	result := &flow.ExecutionResult{
		BlockID: head.ID(),
		Chunks:  list,
	}

	return result
}

func (a *PublicAssignmentTestSuite) GetSeed(t *testing.T) []byte {
	seed := make([]byte, seed.RandomSourceLength)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	return seed
}
