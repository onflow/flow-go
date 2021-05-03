package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/testutil"
	enginemock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
)

// VerificationHappyPath runs `verNodeCount`-many verification nodes
// and checks that concurrently received execution receipts with the same result part that
// by each verification node results in:
// - the selection of the assigned chunks by the ingest engine
// - request of the associated chunk data pack to the assigned chunks
// - formation of a complete verifiable chunk by the ingest engine for each assigned chunk
// - submitting a verifiable chunk locally to the verify engine by the ingest engine
// - dropping the ingestion of the ERs that share the same result once the verifiable chunk is submitted to verify engine
// - broadcast of a matching result approval to consensus nodes for each assigned chunk
func VerificationHappyPath(t *testing.T,
	verNodeCount int,
	chunkNum int,
	verCollector module.VerificationMetrics,
	mempoolCollector module.MempoolMetrics) {
	// to demarcate the debug logs
	log.Debug().
		Int("verification_nodes_count", verNodeCount).
		Int("chunk_num", chunkNum).
		Msg("TestHappyPath started")

	// ingest engine parameters
	// set based on following issue (3443)
	processInterval := 1 * time.Second
	requestInterval := 1 * time.Second
	failureThreshold := uint(2)

	// generates network hub
	hub := stub.NewNetworkHub()

	chainID := flow.Testnet

	// generates identities of nodes, one of each type, `verNodeCount` many of verification nodes
	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentities := unittest.IdentityListFixture(verNodeCount, unittest.WithRole(flow.RoleVerification))
	conIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity}
	identities = append(identities, verIdentities...)

	// creates verification nodes
	verNodes := make([]enginemock.VerificationNode, 0)
	assigner := &mock.ChunkAssigner{}
	for _, verIdentity := range verIdentities {
		verNode := testutil.VerificationNode(t,
			hub,
			verIdentity,
			identities,
			assigner,
			requestInterval,
			processInterval,
			failureThreshold,
			uint(10),          // limits size of receipt related mempools to 10
			uint(10*chunkNum), // limits size of chunks related mempools to 10 * chunkNum
			chainID,
			verCollector,
			mempoolCollector)

		// starts all the engines
		<-verNode.FinderEngine.Ready()
		<-verNode.MatchEngine.(module.ReadyDoneAware).Ready()
		<-verNode.VerifierEngine.(module.ReadyDoneAware).Ready()

		verNodes = append(verNodes, verNode)
	}

	// extracts root block (at height 0) to build a child block succeeding that.
	// since all nodes bootstrapped with same fixture, their root block is same.
	root, err := verNodes[0].State.Params().Root()
	require.NoError(t, err)

	// creates a child block of root, with its corresponding execution result.
	completeER := CompleteExecutionReceiptFixture(t, chunkNum, chainID.Chain(), root)

	// imitates follower engine on verification nodes
	// received block of `completeER` and mutate state accordingly.
	for _, node := range verNodes {
		// ensures all nodes have same root block
		// this is necessary for state mutation.
		rootBlock, err := node.State.Params().Root()
		require.NoError(t, err)
		require.Equal(t, root, rootBlock)

		// extends state of node by block of `completeER`.
		err = node.State.Extend(completeER.ReceiptsData[0].ReferenceBlock)
		assert.Nil(t, err)
	}

	// mocks the assignment to only assign "some" chunks to each verification node.
	// the assignment is done based on `isAssigned` function
	_, expectedChunkIDs := MockChunkAssignmentFixture(assigner, verIdentities, CompleteExecutionReceiptList{completeER}, evenChunkIndexAssigner)

	// mock execution node
	exeNode, exeEngine := setupChunkDataPackProvider(t,
		hub,
		exeIdentity,
		identities,
		chainID,
		CompleteExecutionReceiptList{completeER},
		expectedChunkIDs,
		respondChunkDataPackRequest) // always responds to chunk data pack requests.

	// mock consensus node
	conNode, conEngine, conWG := SetupMockConsensusNode(t,
		hub,
		conIdentity,
		verIdentities,
		identities,
		completeER,
		chainID)

	// sends execution receipt to each of verification nodes
	verWG := sync.WaitGroup{}
	for _, verNode := range verNodes {
		verWG.Add(1)
		go func(vn enginemock.VerificationNode, receipt *flow.ExecutionReceipt) {
			defer verWG.Done()
			err := vn.FinderEngine.Process(exeIdentity.NodeID, receipt)
			require.NoError(t, err)
		}(verNode, completeER.Receipts[0])
	}

	// requires all verification nodes process the receipt
	unittest.RequireReturnsBefore(t, verWG.Wait, time.Duration(chunkNum*verNodeCount*5)*time.Second,
		"verification node process")

	// creates a network instance for each verification node
	// and sets it in continuous delivery mode
	// then flushes the collection requests
	verNets := make([]*stub.Network, 0)
	for _, verIdentity := range verIdentities {
		verNet, ok := hub.GetNetwork(verIdentity.NodeID)
		assert.True(t, ok)
		verNet.StartConDev(requestInterval, true)
		verNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
			return m.Channel == engine.RequestCollections
		})

		verNets = append(verNets, verNet)
	}

	// requires all verification nodes send a result approval per assigned chunk
	unittest.RequireReturnsBefore(t, conWG.Wait, time.Duration(chunkNum*verNodeCount*5)*time.Second,
		"consensus node process")
	// assert that the RA was received
	conEngine.AssertExpectations(t)

	// assert proper number of calls made
	exeEngine.AssertExpectations(t)

	// stops verification nodes
	// Note: this should be done prior to any evaluation to make sure that
	// the process method of Ingest engines is done working.
	for _, verNode := range verNodes {
		// stops all the engines
		<-verNode.FinderEngine.Done()
		<-verNode.MatchEngine.(module.ReadyDoneAware).Done()
		<-verNode.VerifierEngine.(module.ReadyDoneAware).Done()
	}

	// stops continuous delivery of nodes
	for _, verNet := range verNets {
		verNet.StopConDev()
	}

	conNode.Done()
	exeNode.Done()

	// asserts that all processing pipeline of verification node is fully
	// cleaned up.
	for _, verNode := range verNodes {
		assert.Equal(t, verNode.ChunkIDsByResult.Size(), uint(0))
		assert.Equal(t, verNode.CachedReceipts.Size(), uint(0))
		assert.Equal(t, verNode.ReadyReceipts.Size(), uint(0))
		assert.Equal(t, verNode.PendingChunks.Size(), uint(0))
		assert.Equal(t, verNode.PendingReceiptIDsByBlock.Size(), uint(0))
		assert.Equal(t, verNode.PendingReceipts.Size(), uint(0))
		assert.Equal(t, verNode.PendingResults.Size(), uint(0))
		assert.Equal(t, verNode.ReceiptIDsByResult.Size(), uint(0))
	}

	// to demarcate the debug logs
	log.Debug().
		Int("verification_nodes_count", verNodeCount).
		Int("chunk_num", chunkNum).
		Msg("TestHappyPath finishes")
}

// setupChunkDataPackProvider creates and returns an execution node that only has a chunk data pack provider engine.
//
// The mock chunk provider engine replies the chunk back requests by invoking the injected provider method. All chunk data pack
// requests should come from a verification node, and should has one of the assigned chunk IDs. Otherwise, it fails the test.
func setupChunkDataPackProvider(t *testing.T,
	hub *stub.Hub,
	exeIdentity *flow.Identity,
	participants flow.IdentityList,
	chainID flow.ChainID,
	completeERs CompleteExecutionReceiptList,
	assignedChunkIDs flow.IdentifierList,
	provider func(*testing.T, CompleteExecutionReceiptList, flow.Identifier, flow.Identifier, network.Conduit)) (*enginemock.GenericNode,
	*mocknetwork.Engine) {

	exeNode := testutil.GenericNode(t, hub, exeIdentity, participants, chainID)
	exeEngine := new(mocknetwork.Engine)

	exeChunkDataConduit, err := exeNode.Net.Register(engine.ProvideChunks, exeEngine)
	assert.Nil(t, err)

	exeEngine.On("Process", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			originID, ok := args[0].(flow.Identifier)
			require.True(t, ok)
			// request should be dispatched by a verification node.
			require.Contains(t, participants.Filter(filter.HasRole(flow.RoleVerification)).NodeIDs(), originID)

			req, ok := args[1].(*messages.ChunkDataRequest)
			require.True(t, ok)
			require.Contains(t, assignedChunkIDs, req.ChunkID) // only assigned chunks should be requested.

			provider(t, completeERs, req.ChunkID, originID, exeChunkDataConduit)
		}).Return(nil)

	return &exeNode, exeEngine
}

func respondChunkDataPackRequest(t *testing.T,
	completeERs CompleteExecutionReceiptList,
	chunkID flow.Identifier,
	verID flow.Identifier,
	con network.Conduit) {

	// finds the chunk data pack of the requested chunk and sends it back.
	res := completeERs.ChunkDataResponse(t, chunkID)

	err := con.Unicast(res, verID)
	assert.Nil(t, err)

	log.Debug().
		Hex("origin_id", logging.ID(verID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("chunk data pack request answered by provider")
}

// SetupMockConsensusNode creates and returns a mock consensus node (conIdentity) and its registered engine in the
// network (hub). It mocks the process method of the consensus engine to receive a message from a certain
// verification node (verIdentity) evaluates whether it is a result approval about an assigned chunk to that verifier node.
func SetupMockConsensusNode(t *testing.T,
	hub *stub.Hub,
	conIdentity *flow.Identity,
	verIdentities flow.IdentityList,
	othersIdentity flow.IdentityList,
	completeER *CompleteExecutionReceipt,
	chainID flow.ChainID) (*enginemock.GenericNode, *mocknetwork.Engine, *sync.WaitGroup) {
	// determines the expected number of result approvals this node should receive
	approvalsCount := 0
	chunks := completeER.Receipts[0].ExecutionResult.Chunks
	chunksNum := len(chunks)
	for _, chunk := range chunks {
		if evenChunkIndexAssigner(chunk.Index, chunksNum) {
			approvalsCount++
		}
	}

	wg := &sync.WaitGroup{}
	// each verification node is assigned to `approvalsCount`-many independent chunks
	// and there are `len(verIdentities)`-many verification nodes
	// so there is a total of len(verIdentities) * approvalsCount expected
	// result approvals
	wg.Add(len(verIdentities) * approvalsCount)

	// mock the consensus node with a generic node and mocked engine to assert
	// that the result approval is broadcast
	conNode := testutil.GenericNode(t, hub, conIdentity, othersIdentity, chainID)
	conEngine := new(mocknetwork.Engine)

	// map form verIds --> result approval ID
	resultApprovalSeen := make(map[flow.Identifier]map[flow.Identifier]struct{})
	for _, verIdentity := range verIdentities {
		resultApprovalSeen[verIdentity.NodeID] = make(map[flow.Identifier]struct{})
	}

	// creates a hasher for spock
	hasher := crypto.NewBLSKMAC(encoding.SPOCKTag)

	conEngine.On("Process", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			originID, ok := args[0].(flow.Identifier)
			assert.True(t, ok)

			resultApproval, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)

			log.Debug().
				Hex("result_approval_id", logging.ID(resultApproval.ID())).
				Msg("result approval received")

			// asserts that result approval has not been seen from this
			_, ok = resultApprovalSeen[originID][resultApproval.ID()]
			assert.False(t, ok)

			// marks result approval as seen
			resultApprovalSeen[originID][resultApproval.ID()] = struct{}{}

			// asserts that the result approval is assigned to the verifier
			assert.True(t, evenChunkIndexAssigner(resultApproval.Body.ChunkIndex, chunksNum))

			// verifies SPoCK proof of result approval
			// against the SPoCK secret of the execution result
			//
			// retrieves public key of verification node
			var pk crypto.PublicKey
			found := false
			for _, identity := range verIdentities {
				if originID == identity.NodeID {
					pk = identity.StakingPubKey
					found = true
				}
			}
			require.True(t, found)

			// verifies spocks
			valid, err := crypto.SPOCKVerifyAgainstData(
				pk,
				resultApproval.Body.Spock,
				completeER.ReceiptsData[0].SpockSecrets[resultApproval.Body.ChunkIndex],
				hasher,
			)
			assert.NoError(t, err)
			assert.True(t, valid)

			wg.Done()
		}).Return(nil)

	_, err := conNode.Net.Register(engine.ReceiveApprovals, conEngine)
	assert.Nil(t, err)

	return &conNode, conEngine, wg
}

// IsSystemChunk returns true if the index corresponds to the system chunk, i.e., last chunk in
// the receipt.
func IsSystemChunk(index uint64, chunkNum int) bool {
	return int(index) == chunkNum-1
}

func CreateExecutionResult(blockID flow.Identifier, options ...func(result *flow.ExecutionResult, assignments *chunks.Assignment)) (*flow.ExecutionResult, *chunks.Assignment) {
	result := &flow.ExecutionResult{
		BlockID: blockID,
		Chunks:  flow.ChunkList{},
	}
	assignments := chunks.NewAssignment()

	for _, option := range options {
		option(result, assignments)
	}
	return result, assignments
}

func WithChunks(setAssignees ...func(flow.Identifier, uint64, *chunks.Assignment) *flow.Chunk) func(*flow.ExecutionResult, *chunks.Assignment) {
	return func(result *flow.ExecutionResult, assignment *chunks.Assignment) {
		for i, setAssignee := range setAssignees {
			chunk := setAssignee(result.BlockID, uint64(i), assignment)
			result.Chunks.Insert(chunk)
		}
	}
}

func ChunkWithIndex(blockID flow.Identifier, index int) *flow.Chunk {
	chunk := &flow.Chunk{
		Index: uint64(index),
		ChunkBody: flow.ChunkBody{
			CollectionIndex: uint(index),
			EventCollection: blockID, // ensure chunks from different blocks with the same index will have different chunk ID
			BlockID:         blockID,
		},
		EndState: unittest.StateCommitmentFixture(),
	}
	return chunk
}

func WithAssignee(assignee flow.Identifier) func(flow.Identifier, uint64, *chunks.Assignment) *flow.Chunk {
	return func(blockID flow.Identifier, index uint64, assignment *chunks.Assignment) *flow.Chunk {
		chunk := ChunkWithIndex(blockID, int(index))
		fmt.Printf("with assignee: %v, chunk id: %v\n", index, chunk.ID())
		assignment.Add(chunk, flow.IdentifierList{assignee})
		return chunk
	}
}

func FromChunkID(chunkID flow.Identifier) flow.ChunkDataPack {
	return flow.ChunkDataPack{
		ChunkID: chunkID,
	}
}

type ChunkAssignerFunc func(chunkIndex uint64, chunks int) bool

// MockChunkAssignmentFixture is a test helper that mocks a chunk assigner for a set of verification nodes for the
// execution results in the given complete execution receipts, and based on the given chunk assigner function.
//
// It returns the list of chunk locator ids assigned to the input verification nodes, as well as the list of their chunk IDs.
// All verification nodes are assigned the same chunks.
func MockChunkAssignmentFixture(chunkAssigner *mock.ChunkAssigner,
	verIds flow.IdentityList,
	completeERs CompleteExecutionReceiptList,
	isAssigned ChunkAssignerFunc) (flow.IdentifierList, flow.IdentifierList) {

	expectedLocatorIds := flow.IdentifierList{}
	expectedChunkIds := flow.IdentifierList{}

	// keeps track of duplicate results (receipts that share same result)
	visited := make(map[flow.Identifier]struct{})

	for _, completeER := range completeERs {
		for _, receipt := range completeER.Receipts {
			a := chunks.NewAssignment()

			_, duplicate := visited[receipt.ExecutionResult.ID()]
			if duplicate {
				// skips mocking chunk assignment for duplicate results
				continue
			}

			for _, chunk := range receipt.ExecutionResult.Chunks {
				if isAssigned(chunk.Index, len(receipt.ExecutionResult.Chunks)) {
					locatorID := chunks.Locator{
						ResultID: receipt.ExecutionResult.ID(),
						Index:    chunk.Index,
					}.ID()
					expectedLocatorIds = append(expectedLocatorIds, locatorID)
					expectedChunkIds = append(expectedChunkIds, chunk.ID())
					a.Add(chunk, verIds.NodeIDs())
				}

			}

			chunkAssigner.On("Assign", &receipt.ExecutionResult, receipt.ExecutionResult.BlockID).Return(a, nil)
			visited[receipt.ExecutionResult.ID()] = struct{}{}
		}
	}

	return expectedLocatorIds, expectedChunkIds
}

// evenChunkIndexAssigner is a helper function that returns true for the even indices in [0, chunkNum-1]
// It also returns true if the index corresponds to the system chunk.
func evenChunkIndexAssigner(index uint64, chunkNum int) bool {
	ok := index%2 == 0 || IsSystemChunk(index, chunkNum)
	return ok
}

// ExtendStateWithFinalizedBlocks is a test helper to extend the execution state and return the list of blocks.
// It receives a list of complete execution receipt fixtures in the form of (R1,1 <- R1,2 <- ... <- C1) <- (R2,1 <- R2,2 <- ... <- C2) <- .....
// Where R and C are the reference and container blocks.
// Reference blocks contain guarantees, and container blocks contain execution receipt for their preceding reference blocks,
// e.g., C1 contains receipts for R1,1, R1,2, etc.
// Note: for sake of simplicity we do not include guarantees in the container blocks for now.
func ExtendStateWithFinalizedBlocks(t *testing.T, completeExecutionReceipts CompleteExecutionReceiptList, state protocol.MutableState) []*flow.Block {
	blocks := make([]*flow.Block, 0)

	// tracks of duplicate reference blocks
	// since receipts may share the same execution result, hence
	// their reference block is the same (and we should not extend for it).
	duplicate := make(map[flow.Identifier]struct{})

	// extends protocol state with the chain of blocks.
	for _, completeER := range completeExecutionReceipts {
		// extends state with reference blocks of the receipts
		for _, receipt := range completeER.ReceiptsData {
			refBlockID := receipt.ReferenceBlock.ID()
			_, dup := duplicate[refBlockID]
			if dup {
				// skips extending state with already duplicate reference block
				continue
			}

			err := state.Extend(receipt.ReferenceBlock)
			require.NoError(t, err)
			err = state.Finalize(refBlockID)
			require.NoError(t, err)
			blocks = append(blocks, receipt.ReferenceBlock)
			duplicate[refBlockID] = struct{}{}
		}

		// extends state with container block of receipt.
		containerBlockID := completeER.ContainerBlock.ID()
		_, dup := duplicate[containerBlockID]
		if dup {
			// skips extending state with already duplicate container block
			continue
		}
		err := state.Extend(completeER.ContainerBlock)
		require.NoError(t, err)
		err = state.Finalize(containerBlockID)
		require.NoError(t, err)
		blocks = append(blocks, completeER.ContainerBlock)
		duplicate[containerBlockID] = struct{}{}
	}

	return blocks
}

// MockLastSealedHeight mocks the protocol state for the specified last sealed height.
func MockLastSealedHeight(state *mockprotocol.State, height uint64) {
	snapshot := &mockprotocol.Snapshot{}
	header := unittest.BlockHeaderFixture()
	header.Height = height
	state.On("Sealed").Return(snapshot)
	snapshot.On("Head").Return(&header, nil)
}
