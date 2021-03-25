package test

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/testutil"
	enginemock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/engine/verification"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/stub"
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
	completeER := utils.CompleteExecutionReceiptFixture(t, chunkNum, chainID.Chain(), root)
	result := &completeER.Receipt.ExecutionResult

	// imitates follower engine on verification nodes
	// received block of `completeER` and mutate state accordingly.
	for _, node := range verNodes {
		// ensures all nodes have same root block
		// this is necessary for state mutation.
		rootBlock, err := node.State.Params().Root()
		require.NoError(t, err)
		require.Equal(t, root, rootBlock)

		// extends state of node by block of `completeER`.
		err = node.State.Extend(completeER.TestData.ReferenceBlock)
		assert.Nil(t, err)
	}

	// mocks the assignment to only assign "some" chunks to each verification node.
	// the assignment is done based on `isAssigned` function
	a := ChunkAssignmentFixture(verIdentities, completeER.Receipt.ExecutionResult, evenChunkIndexAssigner)
	assigner.On("Assign", result, result.BlockID).Return(a, nil)

	// mock execution node
	exeNode, exeEngine := SetupMockExeNode(t, hub, exeIdentity, verIdentities, identities, chainID, completeER)

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
		}(verNode, completeER.Receipt)
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

// SetupMockExeNode creates and returns an execution node and its registered engine in the network (hub)
// it mocks the process method of execution node that on receiving a chunk data pack request from
// a certain verifier node (verIdentity) about a chunk that is assigned to it, replies the chunk back
// data pack back to the node. Otherwise, if the request is not a chunk data pack request, or if the
// requested chunk data pack is not about an assigned chunk to the verifier node (verIdentity), it fails the
// test.
func SetupMockExeNode(t *testing.T,
	hub *stub.Hub,
	exeIdentity *flow.Identity,
	verIdentities flow.IdentityList,
	othersIdentity flow.IdentityList,
	chainID flow.ChainID,
	completeER *utils.CompleteExecutionReceipt) (*enginemock.GenericNode, *mocknetwork.Engine) {
	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeIdentity, othersIdentity, chainID)
	exeEngine := new(mocknetwork.Engine)

	// determines the expected number of result chunk data pack requests
	chunkDataPackCount := 0
	chunksNum := len(completeER.Receipt.ExecutionResult.Chunks)
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if evenChunkIndexAssigner(chunk.Index, chunksNum) {
			chunkDataPackCount++
		}
	}

	exeChunkDataConduit, err := exeNode.Net.Register(engine.ProvideChunks, exeEngine)
	assert.Nil(t, err)

	chunkNum := len(completeER.TestData.ChunkDataPacks)

	exeEngine.On("Process", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			if originID, ok := args[0].(flow.Identifier); ok {
				if req, ok := args[1].(*messages.ChunkDataRequest); ok {
					require.True(t, ok)
					for i := 0; i < chunkNum; i++ {
						chunk, ok := completeER.Receipt.ExecutionResult.Chunks.ByIndex(uint64(i))
						require.True(t, ok, "chunk out of range requested")
						chunkID := chunk.ID()
						if chunkID == req.ChunkID {
							if !evenChunkIndexAssigner(chunk.Index, chunksNum) {
								require.Error(t, fmt.Errorf(" requested an unassigned chunk data pack %x", req))
							}

							// publishes the chunk data pack response to the network
							res := &messages.ChunkDataResponse{
								ChunkDataPack: *completeER.TestData.ChunkDataPacks[i],
								Nonce:         rand.Uint64(),
							}

							// only non-system chunks have a collection
							if !isSystemChunk(uint64(i), chunksNum) {
								res.Collection = *completeER.TestData.Collections[i]
							}

							err := exeChunkDataConduit.Unicast(res, originID)
							assert.Nil(t, err)

							log.Debug().
								Hex("origin_id", logging.ID(originID)).
								Hex("chunk_id", logging.ID(chunkID)).
								Msg("chunk data pack request answered by execution node")

							return
						}
					}
					require.Error(t, fmt.Errorf(" requested an unidentifed chunk data pack %v", req))
				}
			}

			require.Error(t, fmt.Errorf("unknown request to execution node %v", args[1]))

		}).
		Return(nil)

	return &exeNode, exeEngine
}

// SetupMockConsensusNode creates and returns a mock consensus node (conIdentity) and its registered engine in the
// network (hub). It mocks the process method of the consensus engine to receive a message from a certain
// verification node (verIdentity) evaluates whether it is a result approval about an assigned chunk to that verifier node.
func SetupMockConsensusNode(t *testing.T,
	hub *stub.Hub,
	conIdentity *flow.Identity,
	verIdentities flow.IdentityList,
	othersIdentity flow.IdentityList,
	completeER *utils.CompleteExecutionReceipt,
	chainID flow.ChainID) (*enginemock.GenericNode, *mocknetwork.Engine, *sync.WaitGroup) {
	// determines the expected number of result approvals this node should receive
	approvalsCount := 0
	chunksNum := len(completeER.Receipt.ExecutionResult.Chunks)
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
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
				completeER.TestData.SpockSecrets[resultApproval.Body.ChunkIndex],
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

// SetupMockVerifierEng sets up a mock verifier engine that asserts the followings:
// - that a set of chunks are delivered to it.
// - that each chunk is delivered exactly once
// SetupMockVerifierEng returns the mock engine and a wait group that unblocks when all ERs are received.
func SetupMockVerifierEng(t testing.TB,
	vChunks []*verification.VerifiableChunkData,
	completeER *utils.CompleteExecutionReceipt) (*mocknetwork.Engine, *sync.WaitGroup) {
	eng := new(mocknetwork.Engine)

	// keep track of which verifiable chunks we have received
	receivedChunks := make(map[flow.Identifier]struct{})
	var (
		// decrement the wait group when each verifiable chunk received
		wg sync.WaitGroup
		// check one verifiable chunk at a time to ensure dupe checking works
		mu sync.Mutex
	)

	// computes expected number of assigned chunks
	expected := 0
	chunksNum := len(completeER.Receipt.ExecutionResult.Chunks)
	for _, c := range vChunks {
		if evenChunkIndexAssigner(c.Chunk.Index, chunksNum) {
			expected++
		}
	}
	wg.Add(expected)

	eng.On("ProcessLocal", testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			// the received entity should be a verifiable chunk
			vchunk, ok := args[0].(*verification.VerifiableChunkData)
			assert.True(t, ok)

			// retrieves the content of received chunk
			chunk, ok := vchunk.Result.Chunks.ByIndex(vchunk.Chunk.Index)
			require.True(t, ok, "chunk out of range requested")
			vID := chunk.ID()

			// verifies that it has not seen this chunk before
			_, alreadySeen := receivedChunks[vID]
			if alreadySeen {
				t.Logf("received duplicated chunk (id=%s)", vID)
				t.Fail()
				return
			}

			// ensure the received chunk matches one we expect
			for _, vc := range vChunks {
				if chunk.ID() == vID {
					// mark it as seen and decrement the waitgroup
					receivedChunks[vID] = struct{}{}
					// checks end states match as expected
					if !bytes.Equal(vchunk.EndState, vc.EndState) {
						t.Logf("end states are not equal: expected %x got %x", vchunk.EndState, chunk.EndState)
						t.Fail()
					}
					wg.Done()
					return
				}
			}

			// the received chunk doesn't match any expected ERs
			t.Logf("received unexpected ER (id=%s)", vID)
			t.Fail()
		}).
		Return(nil)

	return eng, &wg
}

func VerifiableDataChunk(t *testing.T, chunkIndex uint64, er utils.CompleteExecutionReceipt) *verification.VerifiableChunkData {
	var endState flow.StateCommitment
	// last chunk
	if int(chunkIndex) == len(er.Receipt.ExecutionResult.Chunks)-1 {
		finalState, ok := er.Receipt.ExecutionResult.FinalStateCommitment()
		require.True(t, ok)
		endState = finalState
	} else {
		endState = er.Receipt.ExecutionResult.Chunks[chunkIndex+1].StartState
	}

	return &verification.VerifiableChunkData{
		Chunk:         er.Receipt.ExecutionResult.Chunks[chunkIndex],
		Header:        er.TestData.ReferenceBlock.Header,
		Result:        &er.Receipt.ExecutionResult,
		Collection:    er.TestData.Collections[chunkIndex],
		ChunkDataPack: er.TestData.ChunkDataPacks[chunkIndex],
		EndState:      endState,
	}
}

// isSystemChunk returns true if the index corresponds to the system chunk, i.e., last chunk in
// the receipt.
func isSystemChunk(index uint64, chunkNum int) bool {
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

func ChunkAssignmentFixture(verIds flow.IdentityList, result flow.ExecutionResult, isAssigned ChunkAssignerFunc) *chunks.Assignment {
	a := chunks.NewAssignment()
	for _, chunk := range result.Chunks {
		assignees := make([]flow.Identifier, 0)
		for _, verIdentity := range verIds {
			if isAssigned(chunk.Index, len(result.Chunks)) {
				assignees = append(assignees, verIdentity.NodeID)
			}
		}
		a.Add(chunk, assignees)
	}
	return a
}

// evenChunkIndexAssigner is a helper function that returns true for the even indices in [0, chunkNum-1]
// It also returns true if the index corresponds to the system chunk.
func evenChunkIndexAssigner(index uint64, chunkNum int) bool {
	ok := index%2 == 0 || isSystemChunk(index, chunkNum)
	return ok
}
