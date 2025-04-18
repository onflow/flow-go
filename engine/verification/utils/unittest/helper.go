package verificationtest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/testutil"
	enginemock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state/state"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
)

// MockChunkDataProviderFunc is a test helper function encapsulating the logic of whether to reply a chunk data pack request.
type MockChunkDataProviderFunc func(*testing.T, CompleteExecutionReceiptList, flow.Identifier, flow.Identifier, network.Conduit) bool

// SetupChunkDataPackProvider creates and returns an execution node that only has a chunk data pack provider engine.
//
// The mock chunk provider engine replies the chunk back requests by invoking the injected provider method. All chunk data pack
// requests should come from a verification node, and should has one of the assigned chunk IDs. Otherwise, it fails the test.
func SetupChunkDataPackProvider(t *testing.T,
	hub *stub.Hub,
	exeIdentity bootstrap.NodeInfo,
	participants flow.IdentityList,
	chainID flow.ChainID,
	completeERs CompleteExecutionReceiptList,
	assignedChunkIDs flow.IdentifierList,
	provider MockChunkDataProviderFunc) (*enginemock.GenericNode,
	*mocknetwork.Engine, *sync.WaitGroup) {

	exeNode := testutil.GenericNodeFromParticipants(t, hub, exeIdentity, participants, chainID)
	exeEngine := new(mocknetwork.Engine)

	exeChunkDataConduit, err := exeNode.Net.Register(channels.ProvideChunks, exeEngine)
	assert.NoError(t, err)

	replied := make(map[flow.Identifier]struct{})

	wg := &sync.WaitGroup{}
	wg.Add(len(assignedChunkIDs))

	mu := &sync.Mutex{} // making testify Run thread-safe

	exeEngine.On("Process", testifymock.AnythingOfType("channels.Channel"), testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			originID, ok := args[1].(flow.Identifier)
			require.True(t, ok)
			// request should be dispatched by a verification node.
			require.Contains(t, participants.Filter(filter.HasRole[flow.Identity](flow.RoleVerification)).NodeIDs(), originID)

			req, ok := args[2].(*messages.ChunkDataRequest)
			require.True(t, ok)
			require.Contains(t, assignedChunkIDs, req.ChunkID) // only assigned chunks should be requested.

			shouldReply := provider(t, completeERs, req.ChunkID, originID, exeChunkDataConduit)
			_, alreadyReplied := replied[req.ChunkID]
			if shouldReply && !alreadyReplied {
				/*
					the wait group keeps track of unique chunk requests addressed.
					we make it done only upon the first successful request of a chunk.
				*/
				wg.Done()
				replied[req.ChunkID] = struct{}{}
			}
		}).Return(nil)

	return &exeNode, exeEngine, wg
}

// RespondChunkDataPackRequestImmediately immediately qualifies a chunk data request for reply by chunk data provider.
func RespondChunkDataPackRequestImmediately(t *testing.T,
	completeERs CompleteExecutionReceiptList,
	chunkID flow.Identifier,
	verID flow.Identifier,
	con network.Conduit) bool {

	// finds the chunk data pack of the requested chunk and sends it back.
	res := completeERs.ChunkDataResponseOf(t, chunkID)

	err := con.Unicast(res, verID)
	assert.NoError(t, err)

	log.Debug().
		Hex("origin_id", logging.ID(verID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("chunk data pack request answered by provider")

	return true
}

// RespondChunkDataPackRequestAfterNTrials only qualifies a chunk data request for reply by chunk data provider after n times.
func RespondChunkDataPackRequestAfterNTrials(n int) MockChunkDataProviderFunc {
	tryCount := make(map[flow.Identifier]int)

	return func(t *testing.T, completeERs CompleteExecutionReceiptList, chunkID flow.Identifier, verID flow.Identifier, con network.Conduit) bool {
		tryCount[chunkID]++

		if tryCount[chunkID] >= n {
			// finds the chunk data pack of the requested chunk and sends it back.
			res := completeERs.ChunkDataResponseOf(t, chunkID)

			err := con.Unicast(res, verID)
			assert.NoError(t, err)

			log.Debug().
				Hex("origin_id", logging.ID(verID)).
				Hex("chunk_id", logging.ID(chunkID)).
				Int("trial_time", tryCount[chunkID]).
				Msg("chunk data pack request answered by provider")

			return true
		}

		return false
	}
}

// SetupMockConsensusNode creates and returns a mock consensus node (conIdentity) and its registered engine in the
// network (hub). It mocks the process method of the consensus engine to receive a message from a certain
// verification node (verIdentity) evaluates whether it is a result approval about an assigned chunk to that verifier node.
func SetupMockConsensusNode(t *testing.T,
	log zerolog.Logger,
	hub *stub.Hub,
	conIdentity bootstrap.NodeInfo,
	verIdentities flow.IdentityList,
	othersIdentity flow.IdentityList,
	completeERs CompleteExecutionReceiptList,
	chainID flow.ChainID,
	assignedChunkIDs flow.IdentifierList) (*enginemock.GenericNode, *mocknetwork.Engine, *sync.WaitGroup) {

	lg := log.With().Str("role", "mock-consensus").Logger()

	wg := &sync.WaitGroup{}
	// each verification node is assigned to issue one result approval per assigned chunk.
	// and there are `len(verIdentities)`-many verification nodes
	// so there is a total of len(verIdentities) * len*(assignedChunkIDs) expected
	// result approvals.
	wg.Add(len(verIdentities) * len(assignedChunkIDs))

	// mock the consensus node with a generic node and mocked engine to assert
	// that the result approval is broadcast
	conNode := testutil.GenericNodeFromParticipants(t, hub, conIdentity, othersIdentity, chainID)
	conEngine := new(mocknetwork.Engine)

	// map form verIds --> result approval ID
	resultApprovalSeen := make(map[flow.Identifier]map[flow.Identifier]struct{})
	for _, verIdentity := range verIdentities {
		resultApprovalSeen[verIdentity.NodeID] = make(map[flow.Identifier]struct{})
	}

	// creates a hasher for spock
	hasher := msig.NewBLSHasher(msig.SPOCKTag)
	mu := &sync.Mutex{} // making testify mock thread-safe

	conEngine.On("Process", testifymock.AnythingOfType("channels.Channel"), testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			originID, ok := args[1].(flow.Identifier)
			assert.True(t, ok)

			resultApproval, ok := args[2].(*flow.ResultApproval)
			assert.True(t, ok)

			lg.Debug().
				Hex("result_approval_id", logging.ID(resultApproval.ID())).
				Msg("result approval received")

			// asserts that result approval has not been seen from this
			_, ok = resultApprovalSeen[originID][resultApproval.ID()]
			assert.False(t, ok)

			// marks result approval as seen
			resultApprovalSeen[originID][resultApproval.ID()] = struct{}{}

			// result approval should belong to an assigned chunk to the verification node.
			chunk := completeERs.ChunkOf(t, resultApproval.Body.ExecutionResultID, resultApproval.Body.ChunkIndex)
			assert.Contains(t, assignedChunkIDs, chunk.ID())

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
				completeERs.ReceiptDataOf(t, chunk.ID()).SpockSecrets[resultApproval.Body.ChunkIndex],
				hasher,
			)
			assert.NoError(t, err)

			if !valid {
				// When chunk verifier returns chunk fault, a placeholder
				// signature is generated for that chunk.
				isChunkFaultSignature, err := crypto.SPOCKVerifyAgainstData(
					pk,
					resultApproval.Body.Spock,
					nil, // chunk fault has no spock secret
					hasher,
				)
				assert.NoError(t, err)

				if isChunkFaultSignature {
					assert.Fail(t, "chunk verifier returned chunk fault")
				} else {
					assert.Fail(t, "spock secret mismatch")
				}
			}

			wg.Done()
		}).Return(nil)

	_, err := conNode.Net.Register(channels.ReceiveApprovals, conEngine)
	assert.NoError(t, err)

	return &conNode, conEngine, wg
}

// isSystemChunk returns true if the index corresponds to the system chunk, i.e., last chunk in
// the receipt.
func isSystemChunk(index uint64, chunkNum int) bool {
	return int(index) == chunkNum-1
}

func CreateExecutionResult(blockID flow.Identifier, options ...func(result *flow.ExecutionResult, assignments *chunks.AssignmentBuilder)) (*flow.ExecutionResult, *chunks.Assignment) {
	result := &flow.ExecutionResult{
		BlockID: blockID,
		Chunks:  flow.ChunkList{},
	}
	assignmentsBuilder := chunks.NewAssignmentBuilder()

	for _, option := range options {
		option(result, assignmentsBuilder)
	}
	return result, assignmentsBuilder.Build()
}

func WithChunks(setAssignees ...func(flow.Identifier, uint64, *chunks.AssignmentBuilder) *flow.Chunk) func(*flow.ExecutionResult, *chunks.AssignmentBuilder) {
	return func(result *flow.ExecutionResult, assignmentBuilder *chunks.AssignmentBuilder) {
		for i, setAssignee := range setAssignees {
			chunk := setAssignee(result.BlockID, uint64(i), assignmentBuilder)
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

func WithAssignee(t *testing.T, assignee flow.Identifier) func(flow.Identifier, uint64, *chunks.AssignmentBuilder) *flow.Chunk {
	return func(blockID flow.Identifier, index uint64, assignmentBuilder *chunks.AssignmentBuilder) *flow.Chunk {
		chunk := ChunkWithIndex(blockID, int(index))
		fmt.Printf("with assignee: %v, chunk id: %v\n", index, chunk.ID())
		require.NoError(t, assignmentBuilder.Add(chunk.Index, flow.IdentifierList{assignee}))
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
func MockChunkAssignmentFixture(t *testing.T,
	chunkAssigner *mock.ChunkAssigner,
	verIds flow.IdentityList,
	completeERs CompleteExecutionReceiptList,
	isAssigned ChunkAssignerFunc) (flow.IdentifierList, flow.IdentifierList) {

	expectedLocatorIds := flow.IdentifierList{}
	expectedChunkIds := flow.IdentifierList{}

	// keeps track of duplicate results (receipts that share same result)
	visited := make(map[flow.Identifier]struct{})

	for _, completeER := range completeERs {
		for _, receipt := range completeER.Receipts {
			a := chunks.NewAssignmentBuilder()

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
					require.NoError(t, a.Add(chunk.Index, verIds.NodeIDs()))
				} else {
					// the chunk has no verifiers assigned
					require.NoError(t, a.Add(chunk.Index, flow.IdentifierList{}))
				}

			}
			assignment := a.Build()

			chunkAssigner.On("Assign", &receipt.ExecutionResult, completeER.ContainerBlock.ID()).Return(assignment, nil)
			visited[receipt.ExecutionResult.ID()] = struct{}{}
		}
	}

	return expectedLocatorIds, expectedChunkIds
}

// EvenChunkIndexAssigner is a helper function that returns true for the even indices in [0, chunkNum-1]
// It also returns true if the index corresponds to the system chunk.
func EvenChunkIndexAssigner(index uint64, chunkNum int) bool {
	ok := index%2 == 0 || isSystemChunk(index, chunkNum)
	return ok
}

// ExtendStateWithFinalizedBlocks is a test helper to extend the execution state and return the list of blocks.
// It receives a list of complete execution receipt fixtures in the form of (R1,1 <- R1,2 <- ... <- C1) <- (R2,1 <- R2,2 <- ... <- C2) <- .....
// Where R and C are the reference and container blocks.
// Reference blocks contain guarantees, and container blocks contain execution receipt for their preceding reference blocks,
// e.g., C1 contains receipts for R1,1, R1,2, etc.
// Note: for sake of simplicity we do not include guarantees in the container blocks for now.
func ExtendStateWithFinalizedBlocks(t *testing.T, completeExecutionReceipts CompleteExecutionReceiptList,
	state protocol.ParticipantState) []*flow.Block {
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

			err := state.Extend(context.Background(), receipt.ReferenceBlock)
			require.NoError(t, err, fmt.Errorf("can not extend block %v: %w", receipt.ReferenceBlock.ID(), err))
			err = state.Finalize(context.Background(), refBlockID)
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
		err := state.Extend(context.Background(), completeER.ContainerBlock)
		require.NoError(t, err)
		err = state.Finalize(context.Background(), containerBlockID)
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
	snapshot.On("Head").Return(header, nil)
}

func NewVerificationHappyPathTest(t *testing.T,
	authorized bool,
	blockCount int,
	eventRepetition int,
	verCollector module.VerificationMetrics,
	mempoolCollector module.MempoolMetrics,
	retry int,
	ops ...CompleteExecutionReceiptBuilderOpt) {

	withConsumers(t, authorized, blockCount, verCollector, mempoolCollector, RespondChunkDataPackRequestAfterNTrials(retry), func(
		blockConsumer *blockconsumer.BlockConsumer,
		blocks []*flow.Block,
		resultApprovalsWG *sync.WaitGroup,
		chunkDataRequestWG *sync.WaitGroup) {

		for i := 0; i < len(blocks)*eventRepetition; i++ {
			// consumer is only required to be "notified" that a new finalized block available.
			// It keeps track of the last finalized block it has read, and read the next height upon
			// getting notified as follows:
			blockConsumer.OnFinalizedBlock(&model.Block{})
		}

		unittest.RequireReturnsBefore(t, chunkDataRequestWG.Wait, time.Duration(10*retry*blockCount)*time.Second,
			"could not receive chunk data requests on time")
		unittest.RequireReturnsBefore(t, resultApprovalsWG.Wait, time.Duration(2*retry*blockCount)*time.Second,
			"could not receive result approvals on time")

	}, ops...)
}

// withConsumers is a test helper that sets up the following pipeline:
// block reader -> block consumer (3 workers) -> assigner engine -> chunks queue -> chunks consumer (3 workers) -> mock chunk processor
//
// The block consumer operates on a block reader with a chain of specified number of finalized blocks
// ready to read.
func withConsumers(t *testing.T,
	authorized bool,
	blockCount int,
	verCollector module.VerificationMetrics, // verification metrics collector
	mempoolCollector module.MempoolMetrics, // memory pool metrics collector
	providerFunc MockChunkDataProviderFunc,
	withBlockConsumer func(*blockconsumer.BlockConsumer, []*flow.Block, *sync.WaitGroup, *sync.WaitGroup),
	ops ...CompleteExecutionReceiptBuilderOpt) {

	tracer := trace.NewNoopTracer()
	log := zerolog.Nop()

	// bootstraps system with one node of each role.
	s, verID, bootstrapNodesInfo := bootstrapSystem(t, log, tracer, authorized)

	participants := bootstrap.ToIdentityList(bootstrapNodesInfo)
	exeIndex := slices.IndexFunc(bootstrapNodesInfo, func(info bootstrap.NodeInfo) bool {
		return info.Role == flow.RoleExecution
	})
	conIndex := slices.IndexFunc(bootstrapNodesInfo, func(info bootstrap.NodeInfo) bool {
		return info.Role == flow.RoleConsensus
	})
	// generates a chain of blocks in the form of root <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
	// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
	// Container blocks only contain receipts of their preceding reference blocks. But they do not
	// hold any guarantees.
	root, err := s.State.Final().Head()
	require.NoError(t, err)
	protocolState, err := s.State.Final().ProtocolState()
	require.NoError(t, err)
	protocolStateID := protocolState.ID()

	chainID := root.ChainID
	ops = append(ops, WithExecutorIDs(
		participants.Filter(filter.HasRole[flow.Identity](flow.RoleExecution)).NodeIDs()), func(builder *CompleteExecutionReceiptBuilder) {
		// needed for the guarantees to have the correct chainID and signer indices
		builder.clusterCommittee = participants.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))
	})

	// random sources for all blocks:
	//  - root block (block[0]) is executed with sources[0] (included in QC of child block[1])
	//  - block[i] is executed with sources[i] (included in QC of child block[i+1])
	sources := unittest.RandomSourcesFixture(30)
	completeERs := CompleteExecutionReceiptChainFixture(t, root, protocolStateID, blockCount, sources, ops...)
	blocks := ExtendStateWithFinalizedBlocks(t, completeERs, s.State)

	// chunk assignment
	chunkAssigner := &mock.ChunkAssigner{}
	assignedChunkIDs := flow.IdentifierList{}
	if authorized {
		// only authorized verification node has some chunks assigned to it.
		_, assignedChunkIDs = MockChunkAssignmentFixture(t,
			chunkAssigner,
			flow.IdentityList{verID.Identity()},
			completeERs,
			EvenChunkIndexAssigner)
	}

	hub := stub.NewNetworkHub()
	collector := &metrics.NoopCollector{}
	chunksLimit := 100
	genericNode := testutil.GenericNodeWithStateFixture(t,
		s,
		hub,
		verID,
		unittest.Logger().With().Str("role", "verification").Logger(),
		collector,
		tracer,
		chainID)

	// execution node
	exeNode, exeEngine, exeWG := SetupChunkDataPackProvider(t,
		hub,
		bootstrapNodesInfo[exeIndex],
		participants,
		chainID,
		completeERs,
		assignedChunkIDs,
		providerFunc)

	// consensus node
	conNode, conEngine, conWG := SetupMockConsensusNode(t,
		unittest.Logger(),
		hub,
		bootstrapNodesInfo[conIndex],
		flow.IdentityList{verID.Identity()},
		participants,
		completeERs,
		chainID,
		assignedChunkIDs)

	verNode := testutil.VerificationNode(t,
		hub,
		verID,
		participants,
		chunkAssigner,
		uint(chunksLimit),
		chainID,
		verCollector,
		mempoolCollector,
		testutil.WithGenericNode(&genericNode))

	// turns on components and network
	verNet, ok := hub.GetNetwork(verID.NodeID)
	require.True(t, ok)
	unittest.RequireReturnsBefore(t, func() {
		verNet.StartConDev(100*time.Millisecond, true)
	}, 100*time.Millisecond, "failed to start verification network")

	unittest.RequireComponentsReadyBefore(t, 1*time.Second,
		verNode.BlockConsumer,
		verNode.ChunkConsumer,
		verNode.AssignerEngine,
		verNode.FetcherEngine,
		verNode.RequesterEngine,
		verNode.VerifierEngine)

	// plays test scenario
	withBlockConsumer(verNode.BlockConsumer, blocks, conWG, exeWG)

	// tears down engines and nodes
	unittest.RequireReturnsBefore(t, verNet.StopConDev, 100*time.Millisecond, "failed to stop verification network")
	unittest.RequireComponentsDoneBefore(t, 100*time.Millisecond,
		verNode.BlockConsumer,
		verNode.ChunkConsumer,
		verNode.AssignerEngine,
		verNode.FetcherEngine,
		verNode.RequesterEngine,
		verNode.VerifierEngine)

	enginemock.RequireGenericNodesDoneBefore(t, 1*time.Second,
		conNode,
		exeNode)

	if !authorized {
		// in unauthorized mode, no message should be received by consensus and execution node.
		conEngine.AssertNotCalled(t, "Process")
		exeEngine.AssertNotCalled(t, "Process")
	}

	// verifies memory resources are cleaned up all over pipeline
	assert.Zero(t, verNode.BlockConsumer.Size())
	assert.Zero(t, verNode.ChunkConsumer.Size())
	assert.Zero(t, verNode.ChunkStatuses.Size())
	assert.Zero(t, verNode.ChunkRequests.Size())
}

// bootstrapSystem is a test helper that bootstraps a flow system with node of each main roles (except execution nodes that are two).
// If authorized set to true, it bootstraps verification node as an authorized one.
// Otherwise, it bootstraps the verification node as unauthorized in current epoch.
//
// As the return values, it returns the state, local module, and list of identities in system.
func bootstrapSystem(
	t *testing.T,
	log zerolog.Logger,
	tracer module.Tracer,
	authorized bool,
) (
	*enginemock.StateFixture,
	bootstrap.NodeInfo,
	[]bootstrap.NodeInfo,
) {
	// creates bootstrapNodesInfo to bootstrap system with
	bootstrapNodesInfo := make([]bootstrap.NodeInfo, 0)
	var verID bootstrap.NodeInfo
	for _, missingRole := range unittest.CompleteIdentitySet() {
		nodeInfo := unittest.PrivateNodeInfoFixture(unittest.WithRole(missingRole.Role))
		if nodeInfo.Role == flow.RoleVerification {
			verID = nodeInfo
		}
		bootstrapNodesInfo = append(bootstrapNodesInfo, nodeInfo)
	}
	bootstrapNodesInfo = append(bootstrapNodesInfo, unittest.PrivateNodeInfoFixture(unittest.WithRole(flow.RoleExecution))) // adds extra execution node
	identities := bootstrap.ToIdentityList(bootstrapNodesInfo)

	collector := &metrics.NoopCollector{}
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	stateFixture := testutil.CompleteStateFixture(t, log, collector, tracer, rootSnapshot)
	// bootstraps the system

	if !authorized {
		// creates a new verification node identity that is unauthorized for this epoch
		verID = unittest.PrivateNodeInfoFixture(unittest.WithRole(flow.RoleVerification))
		bootstrapNodesInfo = append(bootstrapNodesInfo, verID)
		identities = append(identities, verID.Identity())

		mutableProtocolState := protocol_state.NewMutableProtocolState(
			log,
			stateFixture.Storage.EpochProtocolStateEntries,
			stateFixture.Storage.ProtocolKVStore,
			stateFixture.State.Params(),
			stateFixture.Storage.Headers,
			stateFixture.Storage.Results,
			stateFixture.Storage.Setups,
			stateFixture.Storage.EpochCommits,
		)
		epochBuilder := unittest.NewEpochBuilder(t, mutableProtocolState, stateFixture.State)
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(identities.ToSkeleton())).
			BuildEpoch()
	}

	return stateFixture, verID, bootstrapNodesInfo
}
