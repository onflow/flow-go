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

// TestExecutionForkWithDuplicateAssignedChunks evaluates behavior of fetcher engine with respect to
// receiving duplicate identical assigned chunks on execution forks, i.e., an execution fork with two distinct results, where
// first chunk of both execution results are the same (i.e., duplicate), and both duplicate chunks are assigned to this
// verification node.
//
// The test asserts that a chunk data pack is requested for those duplicate chunks, and
// once the chunk data pack arrives, a verifiable chunk is shaped for each of those duplicate chunks, and
// verifiable chunks are pushed to verifier engine.
func TestExecutionForkWithDuplicateAssignedChunks(t *testing.T) {
	s := setupTest()
	e := newFetcherEngine(s)

	// resultsA and resultB belong to an execution fork and their first chunk (statusA and statusB)
	// is duplicate and assigned to this verification node.
	block, resultA, statusA, resultB, statusB, collMap := executionResultForkFixture(t)
	assignedChunkStatuses := verification.ChunkStatusList{statusA, statusB}

	// executorsA and executorsB are execution node identities that executed resultA and resultB, respectively.
	_, _, executorsA, executorsB := mockReceiptsBlockIDForConflictingResults(t, block.ID(), s.receipts, resultA, resultB)
	s.metrics.On("OnAssignedChunkReceivedAtFetcher").Return().Times(2)
	mockStateAtBlockIDForIdentities(s.state, block.ID(), executorsA.Union(executorsB))

	// the chunks belong to an unsealed block, so their chunk data pack is requested.
	mockBlockSealingStatus(s.state, s.headers, block, false)

	// mocks resources on fetcher engine side.
	mockResultsByIDs(s.results, []*flow.ExecutionResult{resultA, resultB})
	mockBlocksStorage(s.blocks, s.headers, block)
	mockPendingChunksAdd(t, s.pendingChunks, assignedChunkStatuses, true)
	mockPendingChunksRem(t, s.pendingChunks, assignedChunkStatuses, true)
	mockPendingChunksGet(s.pendingChunks, assignedChunkStatuses)

	// fetcher engine must create a chunk data request for each of chunk statusA and statusB
	requestA := chunkRequestFixture(resultA.ID(), statusA, executorsA, executorsB)
	requestB := chunkRequestFixture(resultB.ID(), statusB, executorsB, executorsA)
	requests := make(map[flow.Identifier]*verification.ChunkDataPackRequest)
	requests[requestA.ID()] = requestA
	requests[requestB.ID()] = requestB
	s.metrics.On("OnChunkDataPackRequestSentByFetcher").Return().Times(len(assignedChunkStatuses))

	// each chunk data request is answered by requester engine on a distinct chunk data response
	chunkALocatorID := chunks.ChunkLocatorID(statusA.ExecutionResult.ID(), statusA.ChunkIndex)
	chunkBLocatorID := chunks.ChunkLocatorID(statusB.ExecutionResult.ID(), statusB.ChunkIndex)
	chunkDataResponse := make(map[flow.Identifier]*verification.ChunkDataPackResponse)
	chunkDataResponse[chunkALocatorID] = chunkDataPackResponseFixture(t, statusA.Chunk(), collMap[statusA.Chunk().ID()], resultA)
	chunkDataResponse[chunkBLocatorID] = chunkDataPackResponseFixture(t, statusB.Chunk(), collMap[statusA.Chunk().ID()], resultB)
	s.metrics.On("OnChunkDataPackArrivedAtFetcher").Return().Times(len(assignedChunkStatuses))

	// on receiving the chunk data responses, fetcher engine creates verifiable chunks
	verifiableChunks := make(map[flow.Identifier]*verification.VerifiableChunkData)
	verifiableChunks[chunkALocatorID] = verifiableChunkFixture(t, statusA.Chunk(), block, resultA, chunkDataResponse[chunkALocatorID].Cdp)
	verifiableChunks[chunkBLocatorID] = verifiableChunkFixture(t, statusA.Chunk(), block, resultB, chunkDataResponse[chunkBLocatorID].Cdp)
	s.metrics.On("OnVerifiableChunkSentToVerifier").Return().Times(len(assignedChunkStatuses))

	requesterWg := mockRequester(t, s.requester, requests, chunkDataResponse,
		func(originID flow.Identifier, response *verification.ChunkDataPackResponse) {
			// mocks replying to the requests by sending a chunk data pack.
			e.HandleChunkDataPack(originID, response)
		})

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

	unittest.RequireReturnsBefore(t, requesterWg.Wait, 100*time.Millisecond, "could not handle received chunk data pack on time")
	unittest.RequireReturnsBefore(t, verifierWG.Wait, 100*time.Millisecond, "could not push verifiable chunk on time")
	unittest.RequireReturnsBefore(t, processWG.Wait, 100*time.Millisecond, "could not process chunks on time")

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
	collMap[statusA.Chunk().ID()] = collection

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
