package fetcher_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	verificationtest "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// testProcessAssignChunkHappyPath evaluates behavior of fetcher engine respect to receiving some assigned chunks,
// it should request the requester a chunk data pack for each chunk.
// Then the test mocks sending a chunk data response for what fetcher engine requested.
// On receiving the response, fetcher engine should validate it and create and pass a verifiable chunk
// to the verifier engine.
// Once the verifier engine returns, the fetcher engine should notify the chunk consumer that it is done with
// this chunk.
func TestProcessDuplicateChunksWithDifferentResults(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// creates two results
	// also, the result has been created by two execution nodes, while the rest two have a conflicting result with it.
	block, resultA, statusA, resultB, statusB, collMap := executionResultForkFixture(t)
	_, _, executorsA, executorsB := mockReceiptsBlockIDForConflictingResults(t, block.ID(), s.receipts, resultA, resultB)
	s.metrics.On("OnAssignedChunkReceivedAtFetcher").Return().Times(2)
	assignedChunkStatuses := verification.ChunkStatusList{statusA, statusB}

	// the chunks belong to an unsealed block.
	mockBlockSealingStatus(s.state, s.headers, block, false)

	// mocks resources on fetcher engine side.
	mockResultsByIDs(s.results, []*flow.ExecutionResult{resultA, resultB})
	mockBlocksStorage(s.blocks, s.headers, block)
	mockPendingChunksAdd(t, s.pendingChunks, assignedChunkStatuses, true)
	mockPendingChunksRem(t, s.pendingChunks, assignedChunkStatuses, true)
	mockPendingChunksGet(s.pendingChunks, assignedChunkStatuses)
	mockStateAtBlockIDForIdentities(s.state, block.ID(), executorsA.Union(executorsB))

	// generates and mocks requesting chunk data pack fixture
	requestA := chunkRequestFixture(resultA.ID(), statusA, executorsA, executorsB)
	requestB := chunkRequestFixture(resultB.ID(), statusB, executorsB, executorsA)
	requests := make(map[flow.Identifier]*verification.ChunkDataPackRequest)
	requests[requestA.ID()] = requestA
	requests[requestB.ID()] = requestB

	chunkALocatorID := chunks.ChunkLocatorID(statusA.ExecutionResult.ID(), statusA.ChunkIndex)
	chunkBLocatorID := chunks.ChunkLocatorID(statusB.ExecutionResult.ID(), statusB.ChunkIndex)

	// chunk data responses
	chunkDataResponse := make(map[flow.Identifier]*verification.ChunkDataPackResponse)
	chunkDataResponse[chunkALocatorID] = chunkDataPackResponseFixture(t, statusA, collMap[statusA.Chunk().ID()], resultA)
	chunkDataResponse[chunkBLocatorID] = chunkDataPackResponseFixture(t, statusB, collMap[statusA.Chunk().ID()], resultB)

	// verifiable chunks
	verifiableChunks := make(map[flow.Identifier]*verification.VerifiableChunkData)
	verifiableChunks[chunkALocatorID] = verifiableChunkFixture(t, statusA.Chunk(), block, resultA, chunkDataResponse[chunkALocatorID].Cdp)
	verifiableChunks[chunkBLocatorID] = verifiableChunkFixture(t, statusA.Chunk(), block, resultB, chunkDataResponse[chunkBLocatorID].Cdp)

	// fetcher engine should request chunk data for received (assigned) chunk locators
	s.metrics.On("OnChunkDataPackRequestSentByFetcher").Return().Times(len(requests))
	s.metrics.On("OnChunkDataPackArrivedAtFetcher").Return().Times(len(chunkDataResponse))
	requesterWg := mockRequester(t, s.requester, requests, chunkDataResponse,
		func(originID flow.Identifier, response *verification.ChunkDataPackResponse) {

			// mocks replying to the requests by sending a chunk data pack.
			e.HandleChunkDataPack(originID, response)
		})

	// fetcher engine should create and pass a verifiable chunk to verifier engine upon receiving each
	// chunk data responses, and notify the consumer that it is done with processing chunk.
	s.metrics.On("OnVerifiableChunkSentToVerifier").Return().Times(len(verifiableChunks))
	verifierWG := mockVerifierEngine(t, s.verifier, verifiableChunks)
	mockChunkConsumerNotifier(t, s.chunkConsumerNotifier, flow.IdentifierList{chunkALocatorID, chunkBLocatorID})

	// passes chunk data requests in parallel.
	processWG := &sync.WaitGroup{}
	processWG.Add(len(assignedChunkStatuses))
	for _, status := range assignedChunkStatuses {
		locator := &chunks.Locator{
			Index:    status.ChunkIndex,
			ResultID: status.ExecutionResult.ID(),
		}

		go func(l *chunks.Locator) {
			e.ProcessAssignedChunk(l)
			processWG.Done()
		}(locator)

	}

	unittest.RequireReturnsBefore(t, requesterWg.Wait, 1*time.Second, "could not handle received chunk data pack on time")
	unittest.RequireReturnsBefore(t, verifierWG.Wait, 1*time.Second, "could not push verifiable chunk on time")
	unittest.RequireReturnsBefore(t, processWG.Wait, 1*time.Second, "could not process chunks on time")

	mock.AssertExpectationsForObjects(t, s.results, s.requester, s.pendingChunks, s.chunkConsumerNotifier, s.metrics)
}

// executionResultForkFixture creates a reference block with two conflicting execution results that share the same first chunk, and
// creates chunk statuses for that first duplicate chunk on both results.
//
// It returns the block, results, assigned chunk statuses, their corresponding locators, and a map between chunks to their collections.
func executionResultForkFixture(t *testing.T) (*flow.Block,
	*flow.ExecutionResult,
	*verification.ChunkStatus,
	*flow.ExecutionResult,
	*verification.ChunkStatus,
	map[flow.Identifier]*flow.Collection) {

	resultA, resultB, collection, block := verificationtest.ExecutionResultForkFixture(t)

	// creates chunk statuses for shared chunk of result A and B.
	// this imitates that both identical chunks on execution fork are assigned to
	// verification node.
	statusA := &verification.ChunkStatus{
		ChunkIndex:      0,
		ExecutionResult: resultA,
		BlockHeight:     block.Header.Height,
	}
	statusB := &verification.ChunkStatus{
		ChunkIndex:      0,
		ExecutionResult: resultB,
		BlockHeight:     block.Header.Height,
	}

	// keeps collections of assigned chunks
	collMap := make(map[flow.Identifier]*flow.Collection)
	collMap[statusA.ChunkID()] = collection

	return block, resultA, statusA, resultB, statusB, collMap
}

func mockReceiptsBlockIDForConflictingResults(t *testing.T,
	blockID flow.Identifier,
	receipts *storage.ExecutionReceipts,
	resultA *flow.ExecutionResult,
	resultB *flow.ExecutionResult,
) (flow.ExecutionReceiptList, flow.ExecutionReceiptList, flow.IdentityList, flow.IdentityList) {

	executorIdsA := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	executorIdsB := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	require.Len(t, executorIdsA.Union(executorIdsB), 4) // no overlap must be between executor ids

	receiptsA := receiptsForResultFixture(resultA, executorIdsA.NodeIDs())
	receiptsB := receiptsForResultFixture(resultB, executorIdsB.NodeIDs())

	all := append(receiptsA, receiptsB...)

	receipts.On("ByBlockID", blockID).Return(all, nil)
	return receiptsA, receiptsB, executorIdsA, executorIdsB
}

func receiptsForResultFixture(result *flow.ExecutionResult, executors flow.IdentifierList) flow.ExecutionReceiptList {
	receipts := flow.ExecutionReceiptList{}

	for _, executor := range executors {
		receipt := unittest.ExecutionReceiptFixture(
			unittest.WithResult(result),
			unittest.WithExecutorID(executor))

		receipts = append(receipts, receipt)
	}

	return receipts
}
