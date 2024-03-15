package wintermute

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/rand"
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
	receiptTargetIds, err := rootStateFixture.State.Final().Identities(filter.HasRole[flow.Identity](flow.RoleAccess, flow.RoleConsensus, flow.RoleVerification))
	require.NoError(t, err)

	corruptedExecutionIds := flow.IdentifierList(
		allIds.Filter(
			filter.And(filter.HasRole[flow.Identity](flow.RoleExecution),
				filter.HasNodeID[flow.Identity](corruptedIds...)),
		).NodeIDs())
	eventMap, receipts := receiptsWithSameResultFixture(t, 1, corruptedExecutionIds[0:1], receiptTargetIds.NodeIDs())

	mockOrchestratorNetwork := &mockinsecure.OrchestratorNetwork{}
	corruptedReceiptsSentWG := mockOrchestratorNetworkForCorruptedExecutionResult(t,
		mockOrchestratorNetwork,
		receipts[0],
		receiptTargetIds.NodeIDs(),
		corruptedExecutionIds)

	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	// register mock network with orchestrator
	wintermuteOrchestrator.Register(mockOrchestratorNetwork)
	err = wintermuteOrchestrator.HandleEgressEvent(eventMap[receipts[0].ID()])
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
// Orchestrator only corrupts one of them (whichever it receives first), while passing through the other.
// Orchestrator sends the corrupted one to both corrupted execution nodes.
func TestTwoConcurrentExecutionReceipts_DistinctResult(t *testing.T) {
	testConcurrentExecutionReceipts(
		t,
		1,     // two receipts one per execution node.
		false, // receipts have contradicting results.
		3,     // one corrupted execution result sent to two execution node (total 2) + 1 is passed through.
		1,     // one receipt is passed through.
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
// For the result of receipts, orchestrator simply passes them through them (since already conducted a corruption).
func TestMultipleConcurrentExecutionReceipts_DistinctResult(t *testing.T) {
	testConcurrentExecutionReceipts(
		t,
		5,     // 5 receipts per execution node.
		false, // receipts have distinct results.
		11,    // one corrupted result is sent back to two execution nodes (total 2) + 9 pass through.
		9)     // 9 receipts is passed through.
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
		0,    // no receipts is passed through.
	)
}

// TestMultipleConcurrentExecutionReceipts_SameResult evaluates the following scenario:
// Orchestrator receives multiple receipts from corrupted execution nodes (5 from each), where pairs of receipts have the same result.
// The receipts are coming concurrently.
// Orchestrator corrupts result of first receipt (whichever it receives first).
// Orchestrator sends the corrupted one to both corrupted execution nodes.
// When the receipt (with the same result) arrives since it has the same result, and the corrupted version of that already sent to both
// execution node, the orchestrator does nothing.
// For the result of receipts, orchestrator simply passes them through (since already conducted a corruption).
func TestMultipleConcurrentExecutionReceipts_SameResult(t *testing.T) {
	testConcurrentExecutionReceipts(
		t,
		5,    // 5 receipts one per execution node.
		true, // pairwise receipts of execution nodes have same results.
		10,   // one corrupted execution result sent to each execution nodes (total 2) + 8 pass through.
		8,    // 4 receipts are passed through per execution nodes (4 * 2 = 8)
	)
}

// testConcurrentExecutionReceipts sends two execution receipts concurrently to the orchestrator. Depending on the "sameResult" parameter, receipts
// may have the same execution result or not.
// It then sanity checks the behavior of orchestrator regarding the total expected number of events it sends to the orchestrator network, as well as
// the execution receipts it passes through.
func testConcurrentExecutionReceipts(t *testing.T,
	count int,
	sameResult bool,
	expectedOrchestratorOutputEvents int,
	passThroughReceipts int) {

	rootStateFixture, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	corruptedExecutionIds := flow.IdentifierList(
		allIds.Filter(
			filter.And(filter.HasRole[flow.Identity](flow.RoleExecution),
				filter.HasNodeID[flow.Identity](corruptedIds...)),
		).NodeIDs())
	// identities of nodes who are expected targets of an execution receipt.
	receiptTargetIds, err := rootStateFixture.State.Final().Identities(filter.HasRole[flow.Identity](flow.RoleAccess, flow.RoleConsensus, flow.RoleVerification))
	require.NoError(t, err)

	var eventMap map[flow.Identifier]*insecure.EgressEvent
	var receipts []*flow.ExecutionReceipt

	if sameResult {
		eventMap, receipts = receiptsWithSameResultFixture(t, count, corruptedExecutionIds, receiptTargetIds.NodeIDs())
	} else {
		eventMap, receipts = receiptsWithDistinctResultFixture(t, count, corruptedExecutionIds, receiptTargetIds.NodeIDs())
	}

	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	// keeps list of output events sent by orchestrator to the orchestrator network.
	orchestratorOutputEvents := make([]*insecure.EgressEvent, 0)
	orchestratorSentAllEventsWg := &sync.WaitGroup{}
	orchestratorSentAllEventsWg.Add(expectedOrchestratorOutputEvents)

	// mocks orchestrator network to record and keep the output events of
	// orchestrator for further sanity check.
	mockOrchestratorNetwork := &mockinsecure.OrchestratorNetwork{}
	mockOrchestratorNetwork.
		On("SendEgress", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts EgressEvent sent
			event, ok := args[0].(*insecure.EgressEvent)
			require.True(t, ok)

			orchestratorOutputEvents = append(orchestratorOutputEvents, event)
			orchestratorSentAllEventsWg.Done()
		}).Return(nil)
	// registers mock network with orchestrator
	wintermuteOrchestrator.Register(mockOrchestratorNetwork)

	// imitates sending events from corrupted execution nodes to the attacker orchestrator.
	corruptedEnEventSendWG := &sync.WaitGroup{}
	l := len(eventMap)
	corruptedEnEventSendWG.Add(l)
	for _, event := range eventMap {
		event := event // suppress loop variable

		go func() {
			err := wintermuteOrchestrator.HandleEgressEvent(event)
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
		passThroughReceipts,
	)
}

func mockOrchestratorNetworkForCorruptedExecutionResult(
	t *testing.T,
	orchestratorNetwork *mockinsecure.OrchestratorNetwork,
	receipt *flow.ExecutionReceipt,
	receiptTargetIds flow.IdentifierList,
	corruptedExecutionIds flow.IdentifierList) *sync.WaitGroup {

	wg := &sync.WaitGroup{}

	// expecting to receive a corrupted receipt from each of corrupted execution nodes.
	wg.Add(corruptedExecutionIds.Len())
	seen := make(map[flow.Identifier]struct{})

	orchestratorNetwork.
		On("SendEgress", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct

			// extract EgressEvent sent
			event, ok := args[0].(*insecure.EgressEvent)
			require.True(t, ok)

			// make sure sender is a corrupted execution node.
			ok = corruptedExecutionIds.Contains(event.CorruptOriginId)
			require.True(t, ok)

			// makes sure sender is unique
			_, ok = seen[event.CorruptOriginId]
			require.False(t, ok)
			seen[event.CorruptOriginId] = struct{}{}

			// make sure message being sent on correct channel
			require.Equal(t, channels.PushReceipts, event.Channel)

			corruptedResult, ok := event.FlowProtocolEvent.(*flow.ExecutionReceipt)
			require.True(t, ok)

			// make sure the original uncorrupted execution receipt is NOT sent to orchestrator
			require.NotEqual(t, receipt.ExecutionResult, corruptedResult)
			require.ElementsMatch(t, receiptTargetIds, event.TargetIds)

			wg.Done()
		}).Return(nil)

	return wg
}

// TestRespondingWithCorruptedAttestation evaluates when the Wintermute orchestrator receives a chunk data pack request from a CORRUPTED
// verification node for a CORRUPTED chunk, it replies that with a result approval attestation.
func TestRespondingWithCorruptedAttestation(t *testing.T) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	corruptedVerIds := flow.IdentifierList(
		allIds.Filter(
			filter.And(filter.HasRole[flow.Identity](flow.RoleVerification),
				filter.HasNodeID[flow.Identity](corruptedIds...)),
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
	// mocks orchestrator network to record and keep the output events of orchestrator
	mockOrchestratorNetwork := &mockinsecure.OrchestratorNetwork{}
	mockOrchestratorNetwork.On("SendEgress", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts EgressEvent sent
			event, ok := args[0].(*insecure.EgressEvent)
			require.True(t, ok)

			// output of orchestrator for a corrupted chunk request from a corrupted verification node
			// should be a result approval containing a dictated attestation.
			approval, ok := event.FlowProtocolEvent.(*flow.ResultApproval)
			require.True(t, ok)
			attestation := approval.Body.Attestation

			// checking content of attestation
			require.Equal(t, attestation.BlockID, wintermuteOrchestrator.state.corruptedResult.BlockID)
			require.Equal(t, attestation.ExecutionResultID, wintermuteOrchestrator.state.corruptedResult.ID())
			chunk := wintermuteOrchestrator.state.corruptedResult.Chunks[attestation.ChunkIndex]
			require.True(t, wintermuteOrchestrator.state.containsCorruptedChunkId(chunk.ID())) // attestation must be for a corrupted chunk

			corruptedAttestationWG.Done()
		}).Return(nil)

	// registers mock network with orchestrator
	wintermuteOrchestrator.Register(mockOrchestratorNetwork)

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
				err := wintermuteOrchestrator.HandleEgressEvent(cdpReq)
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

// TestPassingThroughChunkDataRequests evaluates when no attacks yet conducted, all chunk data pack requests from corrupted
// verification nodes are passed through.
func TestPassingThroughChunkDataRequests(t *testing.T) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	corruptedVerIds := flow.IdentifierList(
		allIds.Filter(
			filter.And(filter.HasRole[flow.Identity](flow.RoleVerification),
				filter.HasNodeID[flow.Identity](corruptedIds...)),
		).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	wintermuteOrchestrator.state = nil // no attack yet conducted

	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(
			unittest.ExecutionResultFixture(
				unittest.WithChunks(uint(totalChunks)))))
	cdpReqs, chunkIds := chunkDataPackRequestForReceipts(t, []*flow.ExecutionReceipt{receipt}, corruptedVerIds)

	chunkRequestPassThrough := &sync.WaitGroup{}
	chunkRequestPassThrough.Add(totalChunks * len(corruptedVerIds))
	// mocks orchestrator network to record and keep the output events of orchestrator
	mockOrchestratorNetwork := &mockinsecure.OrchestratorNetwork{}
	mockOrchestratorNetwork.On("SendEgress", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts EgressEvent sent
			event, ok := args[0].(*insecure.EgressEvent)
			require.True(t, ok)

			// since no attack yet conducted, the chunk data request must be passed through.
			request, ok := event.FlowProtocolEvent.(*messages.ChunkDataRequest)
			require.True(t, ok)

			// request must be a pass-through.
			require.True(t, chunkIds.Contains(request.ChunkID))

			chunkRequestPassThrough.Done()
		}).Return(nil)

	// registers mock network with orchestrator
	wintermuteOrchestrator.Register(mockOrchestratorNetwork)

	corruptedChunkRequestWG := &sync.WaitGroup{}
	corruptedChunkRequestWG.Add(totalChunks * len(corruptedVerIds))
	for _, cdpReqList := range cdpReqs {
		for _, cdpReq := range cdpReqList {
			cdpReq := cdpReq // suppress loop variable

			go func() {
				err := wintermuteOrchestrator.HandleEgressEvent(cdpReq)
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
		chunkRequestPassThrough.Wait,
		1*time.Second,
		"orchestrator could not send corrupted attestations on time")
}

// TestPassingThroughChunkDataResponse_NoAttack evaluates all chunk data pack responses that do not match the corrupted chunk are passed through by
// orchestrator. In this test, the state of orchestrator is set to nil meaning no attack has been conducted yet.
func TestPassingThroughChunkDataResponse_NoAttack(t *testing.T) {
	testPassingThroughChunkDataResponse(t, nil)
}

// TestPassingThroughChunkDataResponse_WithAttack evaluates all chunk data pack responses that do not match the corrupted chunk are passed through by
// orchestrator. In this test, the state of orchestrator is set, meaning an attack has already been conducted.
func TestPassingThroughChunkDataResponse_WithAttack(t *testing.T) {
	originalResult := unittest.ExecutionResultFixture()
	corruptedResult := unittest.ExecutionResultFixture(unittest.WithChunks(1))
	state := &attackState{
		originalResult:  originalResult,
		corruptedResult: corruptedResult,
	}

	testPassingThroughChunkDataResponse(t, state)
}

// testPassingThroughChunkDataResponse evaluates all chunk data pack responses that do not match the corrupted chunk are passed through by
// orchestrator.
func testPassingThroughChunkDataResponse(t *testing.T, state *attackState) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	verIds := flow.IdentifierList(allIds.Filter(filter.HasRole[flow.Identity](flow.RoleVerification)).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	wintermuteOrchestrator.state = state

	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(
			unittest.ExecutionResultFixture(
				unittest.WithChunks(uint(totalChunks)))))
	// creates chunk data pack response for all verification nodes.
	cdpReps, _ := chunkDataPackResponseForReceipts([]*flow.ExecutionReceipt{receipt}, verIds)

	chunkResponsePassThrough := &sync.WaitGroup{}
	chunkResponsePassThrough.Add(totalChunks * len(verIds))

	// mocks orchestrator network to record and keep the output events of orchestrator
	mockOrchestratorNetwork := &mockinsecure.OrchestratorNetwork{}
	mockOrchestratorNetwork.On("SendEgress", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts EgressEvent sent
			event, ok := args[0].(*insecure.EgressEvent)
			require.True(t, ok)

			_, ok = event.FlowProtocolEvent.(*messages.ChunkDataResponse)
			require.True(t, ok)

			// response must be a pass through
			require.Contains(t, cdpReps, event)

			chunkResponsePassThrough.Done()
		}).Return(nil)

	// registers mock network with orchestrator
	wintermuteOrchestrator.Register(mockOrchestratorNetwork)

	// sends responses to the orchestrator
	corruptedChunkResponseWG := &sync.WaitGroup{}
	corruptedChunkResponseWG.Add(totalChunks * len(verIds))
	for _, cdpRep := range cdpReps {
		cdpRep := cdpRep // suppress loop variable

		go func() {
			err := wintermuteOrchestrator.HandleEgressEvent(cdpRep)
			require.NoError(t, err)

			corruptedChunkResponseWG.Done()
		}()
	}

	// waits till all chunk data pack responses are sent to orchestrator
	unittest.RequireReturnsBefore(t,
		corruptedChunkResponseWG.Wait,
		1*time.Second,
		"could not send all chunk data pack responses on time")

	// waits till all chunk data pack responses are passed through.
	unittest.RequireReturnsBefore(t,
		chunkResponsePassThrough.Wait,
		1*time.Second,
		"orchestrator could not pass through chunk data responses on time")
}

// TestWintermuteChunkResponseForCorruptedChunks evaluates that chunk data responses for corrupted chunks that are sent by corrupted execution nodes
// to HONEST verification nodes are wintermuted (i.e., dropped) by orchestrator.
func TestWintermuteChunkResponseForCorruptedChunks(t *testing.T) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	honestVnIds := flow.IdentifierList(
		allIds.Filter(filter.And(
			filter.HasRole[flow.Identity](flow.RoleVerification),
			filter.Not(filter.HasNodeID[flow.Identity](corruptedIds...)))).NodeIDs())
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
			err := wintermuteOrchestrator.HandleEgressEvent(cdpRep)
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

// TestPassingThroughMiscellaneousEvents checks that any incoming event from corrupted nodes that
// is not relevant to the context of wintermute attack will be passed through by the orchestrator.
// The only events related to the context of wintermute attack are: execution receipt, result approval,
// chunk data pack request, and chunk data pack response.
func TestPassingThroughMiscellaneousEvents(t *testing.T) {
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)

	// creates a block event fixture that is out of the context of
	// the wintermute attack.
	random, err := rand.Uintn(uint(len(corruptedIds)))
	require.NoError(t, err)

	miscellaneousEvent := &insecure.EgressEvent{
		CorruptOriginId:   corruptedIds[random],
		Channel:           channels.TestNetworkChannel,
		Protocol:          insecure.Protocol_MULTICAST,
		TargetNum:         3,
		TargetIds:         unittest.IdentifierListFixture(10),
		FlowProtocolEvent: unittest.BlockFixture(),
	}

	eventPassThrough := &sync.WaitGroup{}
	eventPassThrough.Add(1)

	// mocks orchestrator network to record and keep the output events of orchestrator
	mockOrchestratorNetwork := &mockinsecure.OrchestratorNetwork{}
	mockOrchestratorNetwork.On("SendEgress", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts EgressEvent sent
			event, ok := args[0].(*insecure.EgressEvent)
			require.True(t, ok)

			_, ok = event.FlowProtocolEvent.(flow.Block)
			require.True(t, ok)

			require.Equal(t, miscellaneousEvent, event)

			eventPassThrough.Done()
		}).Return(nil)

	// creates orchestrator
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)
	wintermuteOrchestrator.Register(mockOrchestratorNetwork)

	// sends miscellaneous event to orchestrator.
	eventPassThroughWG := &sync.WaitGroup{}
	eventPassThroughWG.Add(1)

	go func() {
		err := wintermuteOrchestrator.HandleEgressEvent(miscellaneousEvent)
		require.NoError(t, err)

		eventPassThroughWG.Done()
	}()

	// waits till miscellaneous event is sent to orchestrator.
	unittest.RequireReturnsBefore(t,
		eventPassThroughWG.Wait,
		1*time.Second,
		"could not send miscellaneous event on time to orchestrator")

	// waits till miscellaneous event is passed through by the orchestrator.
	unittest.RequireReturnsBefore(t,
		eventPassThroughWG.Wait,
		1*time.Second,
		"orchestrator could not pass through miscellaneous event on time")
}

// TestPassingThrough_ResultApproval evaluates that wintermute attack orchestrator is passing through
// incoming result approvals if they do not belong to original execution result.
func TestPassingThrough_ResultApproval(t *testing.T) {
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	originalResult := unittest.ExecutionResultFixture()
	corruptedResult := unittest.ExecutionResultFixture(unittest.WithChunks(1))
	wintermuteOrchestrator.state = &attackState{
		originalResult:  originalResult,
		corruptedResult: corruptedResult,
	}

	// generates a test result approval that does not belong to original result.
	approval := unittest.ResultApprovalFixture()
	require.NotEqual(t, wintermuteOrchestrator.state.originalResult.ID(), approval.ID())
	require.NotEqual(t, wintermuteOrchestrator.state.corruptedResult.ID(), approval.ID())

	random, err := rand.Uintn(uint(len(corruptedIds)))
	require.NoError(t, err)
	approvalEvent := &insecure.EgressEvent{
		CorruptOriginId:   corruptedIds[random],
		Channel:           channels.TestNetworkChannel,
		Protocol:          insecure.Protocol_MULTICAST,
		TargetNum:         3,
		TargetIds:         unittest.IdentifierListFixture(10),
		FlowProtocolEvent: approval,
	}

	approvalPassThrough := &sync.WaitGroup{}
	approvalPassThrough.Add(1)

	// mocks orchestrator network to record and keep the output events of orchestrator
	mockOrchestratorNetwork := &mockinsecure.OrchestratorNetwork{}
	mockOrchestratorNetwork.On("SendEgress", mock.Anything).
		Run(func(args mock.Arguments) {
			// assert that args passed are correct
			// extracts EgressEvent sent
			event, ok := args[0].(*insecure.EgressEvent)
			require.True(t, ok)

			// passed through event must be a result approval
			_, ok = event.FlowProtocolEvent.(*flow.ResultApproval)
			require.True(t, ok)

			// response must be a pass through
			require.Equal(t, approvalEvent, event)

			approvalPassThrough.Done()
		}).Return(nil)

	// registers mock network with orchestrator
	wintermuteOrchestrator.Register(mockOrchestratorNetwork)

	// sends approval to the orchestrator
	resultApprovalPassThroughWG := &sync.WaitGroup{}
	resultApprovalPassThroughWG.Add(1)
	go func() {
		err := wintermuteOrchestrator.HandleEgressEvent(approvalEvent)
		require.NoError(t, err)

		resultApprovalPassThroughWG.Done()
	}()

	// waits till approval is sent to attack orchestrator
	unittest.RequireReturnsBefore(t,
		resultApprovalPassThroughWG.Wait,
		1*time.Second,
		"could not result approval event to orchestrator")

	// waits till approval is passed through by attack orchestrator
	unittest.RequireReturnsBefore(t,
		approvalPassThrough.Wait,
		1*time.Second,
		"orchestrator could not pass through result approval on time")
}

// TestWintermute_ResultApproval evaluates that wintermute attack orchestrator is dropping (i.e., wintermuting)
// incoming result approvals if they belong to original execution result (i.e., the conflicting result with one that is
// corrupted).
func TestWintermute_ResultApproval(t *testing.T) {
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	wintermuteOrchestrator := NewOrchestrator(unittest.Logger(), corruptedIds, allIds)

	originalResult := unittest.ExecutionResultFixture()
	corruptedResult := unittest.ExecutionResultFixture(unittest.WithChunks(1))
	wintermuteOrchestrator.state = &attackState{
		originalResult:  originalResult,
		corruptedResult: corruptedResult,
	}

	// generates a result approval event for one of the chunks of the original result.
	random, err := rand.Uintn(uint(len(corruptedIds)))
	require.NoError(t, err)
	approvalEvent := &insecure.EgressEvent{
		CorruptOriginId: corruptedIds[random],
		Channel:         channels.TestNetworkChannel,
		Protocol:        insecure.Protocol_MULTICAST,
		TargetNum:       3,
		TargetIds:       unittest.IdentifierListFixture(10),
		FlowProtocolEvent: unittest.ResultApprovalFixture(
			unittest.WithExecutionResultID(originalResult.ID()),
			unittest.WithChunk(0)),
	}

	// mocks orchestrator network
	mockOrchestratorNetwork := &mockinsecure.OrchestratorNetwork{}
	// SendEgress() method should never be called, don't set method mock
	wintermuteOrchestrator.Register(mockOrchestratorNetwork)

	// sends approval to the orchestrator
	resultSendWG := &sync.WaitGroup{}
	resultSendWG.Add(1)
	go func() {
		err := wintermuteOrchestrator.HandleEgressEvent(approvalEvent)
		require.NoError(t, err)

		resultSendWG.Done()
	}()

	// waits till approval is sent to attack orchestrator
	unittest.RequireReturnsBefore(t,
		resultSendWG.Wait,
		1*time.Second,
		"could not result approval event to orchestrator")

	// orchestrator should drop (i.e., wintermute) any result approval belonging to the
	// original result.
	mockOrchestratorNetwork.AssertNotCalled(t, "SendEgress", mock.Anything)
}
