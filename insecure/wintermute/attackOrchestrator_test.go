package wintermute

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSingleExecutionReceipt tests that the orchestrator corrupts the execution receipt if the receipt is
// from a corrupted execution node.
// If an execution receipt is coming from a corrupt execution node,
// then orchestrator tampers with the receipt and generates a counterfeit receipt, and then
// enforces all corrupted execution nodes to send that counterfeit receipt on their behalf in the flow network.
func TestSingleExecutionReceipt(t *testing.T) {
	rootStateFixture, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)

	// identities of nodes who are expected targets of an execution receipt.
	receiptTargetIds, err := rootStateFixture.State.Final().Identities(filter.HasRole(flow.RoleAccess, flow.RoleConsensus, flow.RoleVerification))
	require.NoError(t, err)

	corruptedExecutionIds := flow.IdentifierList(
		allIds.Filter(
			filter.And(filter.HasRole(flow.RoleExecution),
				filter.HasNodeID(corruptedIds...)),
		).NodeIDs())
	eventMap, receipts := receiptsWithSameResultFixture(t, 1, corruptedExecutionIds[0:1], receiptTargetIds.NodeIDs())

	mockAttackNetwork := &mockinsecure.AttackNetwork{}
	corruptedReceiptsSentWG := mockAttackNetworkForCorruptedExecutionResult(t,
		mockAttackNetwork,
		receipts[0],
		receiptTargetIds.NodeIDs(),
		corruptedExecutionIds)

	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	// register mock network with orchestrator
	wintermuteOrchestrator.WithAttackNetwork(mockAttackNetwork)
	err = wintermuteOrchestrator.HandleEventFromCorruptedNode(eventMap[receipts[0].ID()])
	require.NoError(t, err)

	// waits till corrupted receipts dictated to all execution nodes.
	unittest.RequireReturnsBefore(t,
		corruptedReceiptsSentWG.Wait,
		1*time.Second,
		"orchestrator could not send corrupted receipts on time")
}

// TestMultipleExecutionReceipts_DistinctResult evaluates the following scenario:
// Orchestrator receives two receipts with distinct results from distinct corrupted execution nodes.
// Each receipt has a distinct result.
// The receipts are coming concurrently.
// Orchestrator only corrupts one of them (whichever it receives first), while bouncing back the other.
// Orchestrator sends the corrupted one to both corrupted execution nodes.
func TestTwoConcurrentExecutionReceipts_DistinctResult(t *testing.T) {
	testConcurrentExecutionReceipts(
		t,
		1,     // two receipts one per execution node.
		false, // receipts have contradicting results.
		3,     // one corrupted execution result sent to two execution node (total 2) + 1 bounced back
		1,     // one receipt bounces back.
	)
}

// TestMultipleConcurrentExecutionReceipts_DistinctResult evaluates the following scenario:
// Orchestrator receives multiple receipts from corrupted execution nodes (5 from each).
// The receipts are coming concurrently.
// Each receipt has a distinct result.
// Orchestrator corrupts result of first receipt (whichever it receives first).
// Orchestrator sends the corrupted one to both corrupted execution nodes.
// When the receipt (with the same result) arrives since it has the same result, and the corrupted version of that already sent to both
// execution node, the orchestrator does nothing.
// For the result of receipts, orchestrator simply bounces them back (since already conducted an corruption).
func TestMultipleConcurrentExecutionReceipts_DistinctResult(t *testing.T) {
	testConcurrentExecutionReceipts(
		t,
		5,     // 5 receipts per execution node.
		false, // receipts have distinct results.
		11,    // one corrupted result is sent back to two execution nodes (total 2) + 9 bounce back.
		9)     // 9 receipts bounce back.
}

// TestTwoConcurrentExecutionReceipts_SameResult evaluates the following scenario:
// Orchestrator receives two receipts the same result from distinct corrupted execution nodes.
// The receipts are coming concurrently.
// Orchestrator corrupts result of first receipt (whichever it receives first).
// Orchestrator sends the corrupted one to both corrupted execution nodes.
// When the second receipt arrives since it has the same result, and the corrupted version of that already sent to both
// execution node, the orchestrator does nothing.
func TestTwoConcurrentExecutionReceipts_SameResult(t *testing.T) {
	testConcurrentExecutionReceipts(
		t,
		1,    // one receipt per corrupted execution node.
		true, // both execution receipts have same results.
		2,    // orchestrator is supposed send two events
		0,    // no receipts bounce back.
	)
}

// TestMultipleConcurrentExecutionReceipts_SameResult evaluates the following scenario:
// Orchestrator receives multiple receipts from corrupted execution nodes (5 from each), where pairs of receipts have the same result.
// The receipts are coming concurrently.
// Orchestrator corrupts result of first receipt (whichever it receives first).
// Orchestrator sends the corrupted one to both corrupted execution nodes.
// When the receipt (with the same result) arrives since it has the same result, and the corrupted version of that already sent to both
// execution node, the orchestrator does nothing.
// For the result of receipts, orchestrator simply bounces them back (since already conducted an corruption).
func TestMultipleConcurrentExecutionReceipts_SameResult(t *testing.T) {
	testConcurrentExecutionReceipts(
		t,
		5,    // 5 receipts one per execution node.
		true, // pairwise receipts of execution nodes have same results.
		10,   // one corrupted execution result sent to each execution nodes (total 2) + 8 bounced back
		8,    // 4 receipts bounce back per execution nodes (4 * 2 = 8)
	)
}

// testConcurrentExecutionReceipts sends two execution receipts concurrently to the orchestrator. Depending on the "sameResult" parameter, receipts
// may have the same execution result or not.
// It then sanity checks the behavior of orchestrator regarding the total expected number of events it sends to the attack network, as well as
// the execution receipts it bounces back.
func testConcurrentExecutionReceipts(t *testing.T,
	count int,
	sameResult bool,
	expectedOrchestratorOutputEvents int,
	expBouncedBackReceipts int) {

	rootStateFixture, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	corruptedExecutionIds := flow.IdentifierList(
		allIds.Filter(
			filter.And(filter.HasRole(flow.RoleExecution),
				filter.HasNodeID(corruptedIds...)),
		).NodeIDs())
	// identities of nodes who are expected targets of an execution receipt.
	receiptTargetIds, err := rootStateFixture.State.Final().Identities(filter.HasRole(flow.RoleAccess, flow.RoleConsensus, flow.RoleVerification))
	require.NoError(t, err)

	eventMap := make(map[flow.Identifier]*insecure.Event)
	receipts := make([]*flow.ExecutionReceipt, 0)

	if sameResult {
		eventMap, receipts = receiptsWithSameResultFixture(t, count, corruptedExecutionIds, receiptTargetIds.NodeIDs())
	} else {
		eventMap, receipts = receiptsWithDistinctResultFixture(t, count, corruptedExecutionIds, receiptTargetIds.NodeIDs())
	}

	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	// keeps list of output events sent by orchestrator to the attack network.
	orchestratorOutputEvents := make([]*insecure.Event, 0)
	orchestratorSentAllEventsWg := &sync.WaitGroup{}
	orchestratorSentAllEventsWg.Add(expectedOrchestratorOutputEvents)

	// mocks attack network to record and keep the output events of
	// orchestrator for further sanity check.
	mockAttackNetwork := &mockinsecure.AttackNetwork{}
	mockAttackNetwork.
		On("Send", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts Event sent
			event, ok := args[0].(*insecure.Event)
			require.True(t, ok)

			orchestratorOutputEvents = append(orchestratorOutputEvents, event)
			orchestratorSentAllEventsWg.Done()
		}).Return(nil)
	// registers mock network with orchestrator
	wintermuteOrchestrator.WithAttackNetwork(mockAttackNetwork)

	// imitates sending events from corrupted execution nodes to the attacker orchestrator.
	corruptedEnEventSendWG := sync.WaitGroup{}
	corruptedEnEventSendWG.Add(len(eventMap))
	for _, event := range eventMap {
		event := event // suppress loop variable

		go func() {
			err = wintermuteOrchestrator.HandleEventFromCorruptedNode(event)
			require.NoError(t, err)

			corruptedEnEventSendWG.Done()
		}()
	}

	// waits till corrupted nodes send their protocol layer events to orchestrator.
	unittest.RequireReturnsBefore(t,
		corruptedEnEventSendWG.Wait,
		1*time.Second,
		"orchestrator could not send corrupted receipts on time")

	// waits till corrupted receipts dictated to all execution nodes.
	unittest.RequireReturnsBefore(t,
		orchestratorSentAllEventsWg.Wait,
		1*time.Second,
		"orchestrator could not receive corrupted receipts on time")

	// executes post-attack scenario
	orchestratorOutputSanityCheck(
		t,
		orchestratorOutputEvents,
		corruptedExecutionIds,
		flow.GetIDs(receipts),
		expBouncedBackReceipts,
	)
}

func mockAttackNetworkForCorruptedExecutionResult(
	t *testing.T,
	attackNetwork *mockinsecure.AttackNetwork,
	receipt *flow.ExecutionReceipt,
	receiptTargetIds flow.IdentifierList,
	corruptedExecutionIds flow.IdentifierList) *sync.WaitGroup {

	wg := &sync.WaitGroup{}

	// expecting to receive a corrupted receipt from each of corrupted execution nodes.
	wg.Add(corruptedExecutionIds.Len())
	seen := make(map[flow.Identifier]struct{})

	attackNetwork.
		On("Send", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct

			// extract Event sent
			event, ok := args[0].(*insecure.Event)
			require.True(t, ok)

			// make sure sender is a corrupted execution node.
			ok = corruptedExecutionIds.Contains(event.CorruptedId)
			require.True(t, ok)

			// makes sure sender is unique
			_, ok = seen[event.CorruptedId]
			require.False(t, ok)
			seen[event.CorruptedId] = struct{}{}

			// make sure message being sent on correct channel
			require.Equal(t, engine.PushReceipts, event.Channel)

			corruptedResult, ok := event.FlowProtocolEvent.(*flow.ExecutionResult)
			require.True(t, ok)

			// make sure the original uncorrupted execution receipt is NOT sent to orchestrator
			require.NotEqual(t, receipt.ExecutionResult, corruptedResult)
			require.ElementsMatch(t, receiptTargetIds, event.TargetIds)

			wg.Done()
		}).Return(nil)

	return wg
}

// TestRespondingWithCorruptedAttestation evaluates when the Wintermute orchestrator receives a chunk data pack request from a CORRUPTED
//	verification node for a CORRUPTED chunk, it replies that with a result approval attestation.
func TestRespondingWithCorruptedAttestation(t *testing.T) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	corruptedVerIds := flow.IdentifierList(
		allIds.Filter(
			filter.And(filter.HasRole(flow.RoleVerification),
				filter.HasNodeID(corruptedIds...)),
		).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	originalResult := unittest.ExecutionResultFixture()
	corruptedResult := unittest.ExecutionResultFixture(unittest.WithChunks(uint(totalChunks)))
	wintermuteOrchestrator.state = &attackState{
		originalResult:  originalResult,
		corruptedResult: corruptedResult,
	}

	corruptedAttestationWG := &sync.WaitGroup{}
	corruptedAttestationWG.Add(totalChunks * len(corruptedVerIds))
	// mocks attack network to record and keep the output events of orchestrator
	mockAttackNetwork := &mockinsecure.AttackNetwork{}
	mockAttackNetwork.On("Send", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts Event sent
			event, ok := args[0].(*insecure.Event)
			require.True(t, ok)

			// output of orchestrator for a corrupted chunk request from a corrupted verification node
			// should be an attestation.
			attestation, ok := event.FlowProtocolEvent.(*flow.Attestation)
			require.True(t, ok)

			// checking content of attestation
			require.Equal(t, attestation.BlockID, wintermuteOrchestrator.state.corruptedResult.BlockID)
			require.Equal(t, attestation.ExecutionResultID, wintermuteOrchestrator.state.corruptedResult.ID())
			chunk := wintermuteOrchestrator.state.corruptedResult.Chunks[attestation.ChunkIndex]
			require.True(t, wintermuteOrchestrator.state.containsCorruptedChunkId(chunk.ID())) // attestation must be for a corrupted chunk

			corruptedAttestationWG.Done()
		}).Return(nil)

	// registers mock network with orchestrator
	wintermuteOrchestrator.WithAttackNetwork(mockAttackNetwork)

	// chunk data pack request event for original receipt
	cdpReqs, _ := chunkDataPackRequestForReceipts(t,
		[]*flow.ExecutionReceipt{unittest.ExecutionReceiptFixture(unittest.WithResult(corruptedResult))},
		corruptedVerIds)

	corruptedChunkRequestWG := &sync.WaitGroup{}
	corruptedChunkRequestWG.Add(totalChunks * len(corruptedVerIds))
	for _, cdpReqList := range cdpReqs {
		for _, cdpReq := range cdpReqList {
			cdpReq := cdpReq // suppress loop variable

			go func() {
				err := wintermuteOrchestrator.HandleEventFromCorruptedNode(cdpReq)
				require.NoError(t, err)

				corruptedChunkRequestWG.Done()
			}()
		}
	}

	// waits till all chunk data pack requests are sent to orchestrator
	unittest.RequireReturnsBefore(t,
		corruptedChunkRequestWG.Wait,
		1*time.Second,
		"could not send all chunk data pack requests on time")

	// waits till all chunk data pack requests replied with corrupted attestation.
	unittest.RequireReturnsBefore(t,
		corruptedAttestationWG.Wait,
		1*time.Second,
		"orchestrator could not send corrupted attestations on time")
}

// TestBouncingBackChunkDataRequests evaluates when no attacks yet conducted, all chunk data pack requests from corrupted
// verification nodes are bounced back.
func TestBouncingBackChunkDataRequests(t *testing.T) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	corruptedVerIds := flow.IdentifierList(
		allIds.Filter(
			filter.And(filter.HasRole(flow.RoleVerification),
				filter.HasNodeID(corruptedIds...)),
		).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	wintermuteOrchestrator.state = nil // no attack yet conducted

	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(
			unittest.ExecutionResultFixture(
				unittest.WithChunks(uint(totalChunks)))))
	cdpReqs, chunkIds := chunkDataPackRequestForReceipts(t, []*flow.ExecutionReceipt{receipt}, corruptedVerIds)

	chunkRequestBouncedBack := &sync.WaitGroup{}
	chunkRequestBouncedBack.Add(totalChunks * len(corruptedVerIds))
	// mocks attack network to record and keep the output events of orchestrator
	mockAttackNetwork := &mockinsecure.AttackNetwork{}
	mockAttackNetwork.On("Send", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts Event sent
			event, ok := args[0].(*insecure.Event)
			require.True(t, ok)

			// since no attack yet conducted, the chunk data request must bounce back.
			request, ok := event.FlowProtocolEvent.(*messages.ChunkDataRequest)
			require.True(t, ok)

			// request must be a bounced back
			require.True(t, chunkIds.Contains(request.ChunkID))

			chunkRequestBouncedBack.Done()
		}).Return(nil)

	// registers mock network with orchestrator
	wintermuteOrchestrator.WithAttackNetwork(mockAttackNetwork)

	corruptedChunkRequestWG := &sync.WaitGroup{}
	corruptedChunkRequestWG.Add(totalChunks * len(corruptedVerIds))
	for _, cdpReqList := range cdpReqs {
		for _, cdpReq := range cdpReqList {
			cdpReq := cdpReq // suppress loop variable

			go func() {
				err := wintermuteOrchestrator.HandleEventFromCorruptedNode(cdpReq)
				require.NoError(t, err)

				corruptedChunkRequestWG.Done()
			}()
		}
	}

	// waits till all chunk data pack requests are sent to orchestrator
	unittest.RequireReturnsBefore(t,
		corruptedChunkRequestWG.Wait,
		1*time.Second,
		"could not send all chunk data pack requests on time")

	// waits till all chunk data pack requests replied with corrupted attestation.
	unittest.RequireReturnsBefore(t,
		chunkRequestBouncedBack.Wait,
		1*time.Second,
		"orchestrator could not send corrupted attestations on time")
}

// TestBouncingBackChunkDataResponse_NoAttack evaluates all chunk data pack responses that do not match the corrupted chunk are bounced back from
// orchestrator. In this test, the state of orchestrator is set to nil meaning no attack has been conducted yet.
func TestBouncingBackChunkDataResponse_NoAttack(t *testing.T) {
	testBouncingBackChunkDataResponse(t, nil)
}

// TestBouncingBackChunkDataResponse_WithAttack evaluates all chunk data pack responses that do not match the corrupted chunk are bounced back from
// orchestrator. In this test, the state of orchestrator is set, meaning an attack has already been conducted.
func TestBouncingBackChunkDataResponse_WithAttack(t *testing.T) {
	originalResult := unittest.ExecutionResultFixture()
	corruptedResult := unittest.ExecutionResultFixture(unittest.WithChunks(1))
	state := &attackState{
		originalResult:  originalResult,
		corruptedResult: corruptedResult,
	}

	testBouncingBackChunkDataResponse(t, state)
}

// testBouncingBackChunkDataRequests evaluates all chunk data pack responses that do not match the corrupted chunk are bounced back from
//orchestrator.
func testBouncingBackChunkDataResponse(t *testing.T, state *attackState) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	verIds := flow.IdentifierList(allIds.Filter(filter.HasRole(flow.RoleVerification)).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	wintermuteOrchestrator.state = state

	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(
			unittest.ExecutionResultFixture(
				unittest.WithChunks(uint(totalChunks)))))
	// creates chunk data pack response for all verification nodes.
	cdpReps, _ := chunkDataPackResponseForReceipts([]*flow.ExecutionReceipt{receipt}, verIds)

	chunkResponseBouncedBack := &sync.WaitGroup{}
	chunkResponseBouncedBack.Add(totalChunks * len(verIds))

	// mocks attack network to record and keep the output events of orchestrator
	mockAttackNetwork := &mockinsecure.AttackNetwork{}
	mockAttackNetwork.On("Send", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts Event sent
			event, ok := args[0].(*insecure.Event)
			require.True(t, ok)

			_, ok = event.FlowProtocolEvent.(*messages.ChunkDataResponse)
			require.True(t, ok)

			// response must be a bounced back
			require.Contains(t, cdpReps, event)

			chunkResponseBouncedBack.Done()
		}).Return(nil)

	// registers mock network with orchestrator
	wintermuteOrchestrator.WithAttackNetwork(mockAttackNetwork)

	// sends responses to the orchestrator
	corruptedChunkResponseWG := &sync.WaitGroup{}
	corruptedChunkResponseWG.Add(totalChunks * len(verIds))
	for _, cdpRep := range cdpReps {
		cdpRep := cdpRep // suppress loop variable

		go func() {
			err := wintermuteOrchestrator.HandleEventFromCorruptedNode(cdpRep)
			require.NoError(t, err)

			corruptedChunkResponseWG.Done()
		}()
	}

	// waits till all chunk data pack responses are sent to orchestrator
	unittest.RequireReturnsBefore(t,
		corruptedChunkResponseWG.Wait,
		1*time.Second,
		"could not send all chunk data pack responses on time")

	// waits till all chunk data pack responses are bounced back.
	unittest.RequireReturnsBefore(t,
		chunkResponseBouncedBack.Wait,
		1*time.Second,
		"orchestrator could not bounce back chunk data responses on time")
}

// TestWintermuteChunkResponseForCorruptedChunks evaluates that chunk data responses for corrupted chunks that are sent by corrupted execution nodes
// to HONEST verification nodes are wintermuted (i.e., dropped) by orchestrator.
func TestWintermuteChunkResponseForCorruptedChunks(t *testing.T) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	honestVnIds := flow.IdentifierList(
		allIds.Filter(filter.And(
			filter.HasRole(flow.RoleVerification),
			filter.Not(filter.HasNodeID(corruptedIds...)))).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	originalResult := unittest.ExecutionResultFixture()
	corruptedResult := unittest.ExecutionResultFixture(unittest.WithChunks(uint(totalChunks)))
	state := &attackState{
		originalResult:  originalResult,
		corruptedResult: corruptedResult,
	}
	wintermuteOrchestrator.state = state

	// creates chunk data pack response of corrupted chunks for HONEST verification nodes
	cdpReps, _ := chunkDataPackResponseForReceipts(
		[]*flow.ExecutionReceipt{unittest.ExecutionReceiptFixture(unittest.WithResult(corruptedResult))},
		honestVnIds)

	// sends responses to the orchestrator
	corruptedChunkResponseWG := &sync.WaitGroup{}
	corruptedChunkResponseWG.Add(totalChunks * len(honestVnIds))
	for _, cdpRep := range cdpReps {
		cdpRep := cdpRep // suppress loop variable

		go func() {
			err := wintermuteOrchestrator.HandleEventFromCorruptedNode(cdpRep)
			require.NoError(t, err)

			corruptedChunkResponseWG.Done()
		}()
	}

	// waits till all chunk data pack responses are sent to orchestrator
	unittest.RequireReturnsBefore(t,
		corruptedChunkResponseWG.Wait,
		1*time.Second,
		"could not send all chunk data pack responses on time")
}
