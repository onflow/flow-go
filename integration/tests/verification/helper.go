package verification

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	mock2 "github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// setupMockExeNode creates and returns an execution node and its registered engine in the network (hub)
// it mocks the process method of execution node that on receiving a chunk data pack request from
// a certain verifier node (verIdentity) about a chunk that is assigned to it, replies the chunk back
// data pack back to the node. Otherwise, if the request is not a chunk data pack request, or if the
// requested chunk data pack is not about an assigned chunk to the verifier node (verIdentity), it fails the
// test.
func setupMockExeNode(t *testing.T,
	hub *stub.Hub,
	exeIdentity *flow.Identity,
	verIdentities flow.IdentityList,
	othersIdentity flow.IdentityList,
	completeER utils.CompleteExecutionResult) (*mock2.GenericNode, *network.Engine) {
	// mock the execution node with a generic node and mocked engine
	// to handle request for chunk state
	exeNode := testutil.GenericNode(t, hub, exeIdentity, othersIdentity)
	exeEngine := new(network.Engine)

	var mu sync.Mutex // to secure the mutual exclusion of process method

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
			mu.Lock()
			defer mu.Unlock()

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

// setupMockConsensusNode creates and returns a mock consensus node (conIdentity) and its registered engine in the
// network (hub). It mocks the process method of the consensus engine to receive a message from a certain
// verification node (verIdentity) evaluates whether it is a result approval about an assigned chunk to that verifier node.
func setupMockConsensusNode(t *testing.T,
	hub *stub.Hub,
	conIdentity *flow.Identity,
	verIdentities flow.IdentityList,
	othersIdentity flow.IdentityList,
	completeER utils.CompleteExecutionResult) (*mock2.GenericNode, *network.Engine, *sync.WaitGroup) {
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
	conNode := testutil.GenericNode(t, hub, conIdentity, othersIdentity)
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
func SetupMockVerifierEng(t testing.TB, vChunks []*verification.VerifiableChunk) (*network.Engine, *sync.WaitGroup) {
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
		if IsAssigned(c.ChunkIndex) {
			expected++
		}
	}
	wg.Add(expected)

	eng.On("ProcessLocal", testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			// the received entity should be a verifiable chunk
			vchunk, ok := args[0].(*verification.VerifiableChunk)
			assert.True(t, ok)

			// retrieves the content of received chunk
			chunk, ok := vchunk.Receipt.ExecutionResult.Chunks.ByIndex(vchunk.ChunkIndex)
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

func VerifiableChunk(chunkIndex uint64, er verification.CompleteExecutionResult) *verification.VerifiableChunk {
	var endState flow.StateCommitment
	// last chunk
	if int(chunkIndex) == len(er.Receipt.ExecutionResult.Chunks)-1 {
		endState = er.Receipt.ExecutionResult.FinalStateCommit
	} else {
		endState = er.Receipt.ExecutionResult.Chunks[chunkIndex+1].StartState
	}

	return &verification.VerifiableChunk{
		ChunkIndex:    chunkIndex,
		EndState:      endState,
		Block:         er.Block,
		Receipt:       er.Receipt,
		Collection:    er.Collections[chunkIndex],
		ChunkDataPack: er.ChunkDataPacks[chunkIndex],
	}
}
