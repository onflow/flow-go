package verification

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

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	mock2 "github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/logging"
	"github.com/dapperlabs/flow-go/utils/unittest"
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
	chunkNum int) {
	// to demarcate the debug logs
	log.Debug().
		Int("verification_nodes_count", verNodeCount).
		Int("chunk_num", chunkNum).
		Msg("TestHappyPath started")

	// ingest engine parameters
	// set based on issue (3443)
	requestInterval := uint(1000)
	failureThreshold := uint(2)

	// generates network hub
	hub := stub.NewNetworkHub()

	chainID := flow.Mainnet

	// generates identities of nodes, one of each type, `verNodeCount` many of verification nodes
	colIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	exeIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentities := unittest.IdentityListFixture(verNodeCount, unittest.WithRole(flow.RoleVerification))
	conIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	identities := flow.IdentityList{colIdentity, conIdentity, exeIdentity}
	identities = append(identities, verIdentities...)

	// Execution receipt and chunk assignment
	//
	// creates an execution receipt and its associated data
	// with `chunkNum` chunks
	completeER := utils.CompleteExecutionResultFixture(t, chunkNum, flow.Testnet.Chain())

	// mocks the assignment to only assign "some" chunks to the verIdentity
	// the assignment is done based on `isAssgined` function
	assigner := &mock.ChunkAssigner{}
	a := chmodel.NewAssignment()
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		assignees := make([]flow.Identifier, 0)
		for _, verIdentity := range verIdentities {
			if IsAssigned(chunk.Index) {
				assignees = append(assignees, verIdentity.NodeID)
			}
		}
		a.Add(chunk, assignees)
	}
	assigner.On("Assign",
		testifymock.Anything,
		completeER.Receipt.ExecutionResult.Chunks,
		testifymock.Anything).
		Return(a, nil)

	// nodes and engines
	//
	// verification node
	verNodes := make([]mock2.VerificationNode, 0)
	for _, verIdentity := range verIdentities {
		verNode := testutil.VerificationNode(t,
			hub,
			verIdentity,
			identities,
			assigner,
			requestInterval,
			failureThreshold,
			chainID)

		// starts all the engines
		<-verNode.FinderEngine.Ready()
		<-verNode.MatchEngine.(module.ReadyDoneAware).Ready()
		<-verNode.VerifierEngine.(module.ReadyDoneAware).Ready()

		// assumes the verification node has received the block
		err := verNode.Blocks.Store(completeER.Block)
		assert.Nil(t, err)

		verNodes = append(verNodes, verNode)
	}

	// mock execution node
	exeNode, exeEngine := SetupMockExeNode(t, hub, exeIdentity, verIdentities, identities, chainID, completeER)

	// mock consensus node
	conNode, conEngine, conWG := SetupMockConsensusNode(t, hub, conIdentity, verIdentities, identities, completeER, chainID)

	// sends execution receipt to each of verification nodes
	verWG := sync.WaitGroup{}
	for _, verNode := range verNodes {
		verWG.Add(1)
		go func(vn mock2.VerificationNode, receipt *flow.ExecutionReceipt) {
			defer verWG.Done()
			err := vn.FinderEngine.Process(exeIdentity.NodeID, receipt)
			require.NoError(t, err)
		}(verNode, completeER.Receipt)
	}

	// requires all verification nodes process the receipt
	unittest.RequireReturnsBefore(t, verWG.Wait, time.Duration(chunkNum*verNodeCount*5)*time.Second)

	// creates a network instance for each verification node
	// and sets it in continuous delivery mode
	// then flushes the collection requests
	verNets := make([]*stub.Network, 0)
	for _, verIdentity := range verIdentities {
		verNet, ok := hub.GetNetwork(verIdentity.NodeID)
		assert.True(t, ok)
		verNet.StartConDev(requestInterval, true)
		verNet.DeliverSome(true, func(m *stub.PendingMessage) bool {
			return m.ChannelID == engine.CollectionProvider
		})

		verNets = append(verNets, verNet)
	}

	// requires all verification nodes send a result approval per assigned chunk
	unittest.RequireReturnsBefore(t, conWG.Wait, time.Duration(chunkNum*verNodeCount*5)*time.Second)
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
	completeER utils.CompleteExecutionResult) (*mock2.GenericNode, *network.Engine) {
	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeIdentity, othersIdentity, chainID)
	exeEngine := new(network.Engine)

	// determines the expected number of result chunk data pack requests
	chunkDataPackCount := 0
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if IsAssigned(chunk.Index) {
			chunkDataPackCount++
		}
	}

	exeChunkDataConduit, err := exeNode.Net.Register(engine.ChunkDataPackProvider, exeEngine)
	assert.Nil(t, err)

	chunkNum := len(completeER.ChunkDataPacks)

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
							if !IsAssigned(chunk.Index) {
								require.Error(t, fmt.Errorf(" requested an unassigned chunk data pack %x", req))
							}

							// publishes the chunk data pack response to the network
							res := &messages.ChunkDataResponse{
								ChunkDataPack: *completeER.ChunkDataPacks[i],
								Collection:    *completeER.Collections[i],
								Nonce:         rand.Uint64(),
							}
							err := exeChunkDataConduit.Submit(res, originID)
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
	completeER utils.CompleteExecutionResult,
	chainID flow.ChainID) (*mock2.GenericNode, *network.Engine, *sync.WaitGroup) {
	// determines the expected number of result approvals this node should receive
	approvalsCount := 0
	for _, chunk := range completeER.Receipt.ExecutionResult.Chunks {
		if IsAssigned(chunk.Index) {
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
	conEngine := new(network.Engine)

	// map form verIds --> result approval ID
	resultApprovalSeen := make(map[flow.Identifier]map[flow.Identifier]struct{})
	for _, verIdentity := range verIdentities {
		resultApprovalSeen[verIdentity.NodeID] = make(map[flow.Identifier]struct{})
	}

	conEngine.On("Process", testifymock.Anything, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			originID, ok := args[0].(flow.Identifier)
			assert.True(t, ok)

			resultApproval, ok := args[1].(*flow.ResultApproval)
			assert.True(t, ok)

			// asserts that result approval has not been seen from this
			_, ok = resultApprovalSeen[originID][resultApproval.ID()]
			assert.False(t, ok)

			// marks result approval as seen
			resultApprovalSeen[originID][resultApproval.ID()] = struct{}{}

			// asserts that the result approval is assigned to the verifier
			assert.True(t, IsAssigned(resultApproval.Body.ChunkIndex))

			wg.Done()
		}).Return(nil)

	_, err := conNode.Net.Register(engine.ApprovalProvider, conEngine)
	assert.Nil(t, err)

	return &conNode, conEngine, wg
}

// SetupMockVerifierEng sets up a mock verifier engine that asserts the followings:
// - that a set of chunks are delivered to it.
// - that each chunk is delivered exactly once
// SetupMockVerifierEng returns the mock engine and a wait group that unblocks when all ERs are received.
func SetupMockVerifierEng(t testing.TB, vChunks []*verification.VerifiableChunkData) (*network.Engine, *sync.WaitGroup) {
	eng := new(network.Engine)

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
	for _, c := range vChunks {
		if IsAssigned(c.Chunk.Index) {
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

func VerifiableDataChunk(chunkIndex uint64, er utils.CompleteExecutionResult) *verification.VerifiableChunkData {
	var endState flow.StateCommitment
	// last chunk
	if int(chunkIndex) == len(er.Receipt.ExecutionResult.Chunks)-1 {
		endState = er.Receipt.ExecutionResult.FinalStateCommit
	} else {
		endState = er.Receipt.ExecutionResult.Chunks[chunkIndex+1].StartState
	}

	return &verification.VerifiableChunkData{
		Chunk:         er.Receipt.ExecutionResult.Chunks[chunkIndex],
		Header:        er.Block.Header,
		Result:        &er.Receipt.ExecutionResult,
		Collection:    er.Collections[chunkIndex],
		ChunkDataPack: er.ChunkDataPacks[chunkIndex],
		EndState:      endState,
	}
}

// IsAssigned is a helper function that returns true for the even indices in [0, chunkNum-1]
func IsAssigned(index uint64) bool {
	return index%2 == 0
}
