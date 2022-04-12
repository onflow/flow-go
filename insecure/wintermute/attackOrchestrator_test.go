package wintermute

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/testutil"
	enginemock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSingleExecutionReceipt tests that the orchestrator corrupts the execution receipt if the receipt is
// from a corrupted execution node.
// If an execution receipt is coming from a corrupt execution node,
// then orchestrator tampers with the receipt and generates a counterfeit receipt, and then
// enforces all corrupted execution nodes to send that counterfeit receipt on their behalf in the flow network.
func TestSingleExecutionReceipt(t *testing.T) {
	rootStateFixture, allIdentityList, corruptedIdentityList := bootstrapWintermuteFlowSystem(t)

	// identities of nodes who are expected targets of an execution receipt.
	receiptTargetIds, err := rootStateFixture.State.Final().Identities(filter.HasRole(flow.RoleAccess, flow.RoleConsensus, flow.RoleVerification))
	require.NoError(t, err)

	corruptedExecutionIds := flow.IdentifierList(corruptedIdentityList.Filter(filter.HasRole(flow.RoleExecution)).NodeIDs())
	eventMap, receipts := receiptsWithSameResultFixture(t, 1, corruptedExecutionIds[0:1], receiptTargetIds.NodeIDs())

	mockAttackNetwork := &mockinsecure.AttackNetwork{}
	corruptedReceiptsSentWG := mockAttackNetworkForCorruptedExecutionResult(t,
		mockAttackNetwork,
		receipts[0],
		receiptTargetIds.NodeIDs(),
		corruptedExecutionIds)

	wintermuteOrchestrator := NewOrchestrator(allIdentityList, corruptedIdentityList, unittest.Logger())

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

	rootStateFixture, allIdentityList, corruptedIdentityList := bootstrapWintermuteFlowSystem(t)
	corruptedExecutionIds := flow.IdentifierList(corruptedIdentityList.Filter(filter.HasRole(flow.RoleExecution)).NodeIDs())
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

	wintermuteOrchestrator := NewOrchestrator(allIdentityList, corruptedIdentityList, unittest.Logger())

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

// helper functions

// bootstrapWintermuteFlowSystem bootstraps flow network with following setup:
// verification nodes: 3 corrupted + 1 honest
// execution nodes: 2 corrupted + 1 honest
// other roles at the minimum required number and all honest.
func bootstrapWintermuteFlowSystem(t *testing.T) (*enginemock.StateFixture, flow.IdentityList, flow.IdentityList) {
	// creates identities to bootstrap system with
	corruptedVnIds := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleVerification))
	corruptedEnIds := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	identities := unittest.CompleteIdentitySet(append(corruptedVnIds, corruptedEnIds...)...)
	identities = append(identities, unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)))    // one honest execution node
	identities = append(identities, unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))) // one honest verification node

	// bootstraps the system
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	stateFixture := testutil.CompleteStateFixture(t, metrics.NewNoopCollector(), trace.NewNoopTracer(), rootSnapshot)

	return stateFixture, identities, append(corruptedEnIds, corruptedVnIds...)
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

// orchestratorOutputSanityCheck performs a sanity check on the output events dictated by the wintermute orchestrator to the corrupted nodes.
// It checks that: (1) exactly one execution result is dictated to ALL corrupted execution nodes. (2) except that one execution result, all other
// incoming execution receipts are bounced back to corrupted execution node who sent it originally.
//
// An execution result is "dictated" when orchestrator corrupts a given result.
// An execution receipt is "bounced" back when orchestrator doesn't tamper with it, and let it go to the flow network as it is.
func orchestratorOutputSanityCheck(
	t *testing.T,
	outputEvents []*insecure.Event, // list of all output events of the wintermute orchestrator.
	corrEnIds flow.IdentifierList, // list of all corrupted execution node ids.
	orgReceiptIds flow.IdentifierList, // list of all execution receipt ids originally sent to orchestrator.
	expBouncedReceiptCount int, // expected number of execution receipts that must remain uncorrupted.
) {

	// keeps a map of (corrupted results ids -> execution node ids)
	dictatedResults := make(map[flow.Identifier]flow.IdentifierList)

	// keeps a list of all bounced back events.
	bouncedReceipts := flow.IdentifierList{}

	for _, outputEvent := range outputEvents {
		switch event := outputEvent.FlowProtocolEvent.(type) {
		case *flow.ExecutionReceipt:
			// makes sure sender is a corrupted execution node.
			ok := corrEnIds.Contains(outputEvent.CorruptedId)
			require.True(t, ok)
			// uses union to avoid adding duplicate.
			bouncedReceipts = bouncedReceipts.Union(flow.IdentifierList{event.ID()})
		case *flow.ExecutionResult:
			resultId := event.ID()
			if dictatedResults[resultId] == nil {
				dictatedResults[resultId] = flow.IdentifierList{}
			}
			// uses union to avoid adding duplicate.
			dictatedResults[resultId] = dictatedResults[resultId].Union(flow.IdentifierList{outputEvent.CorruptedId})
		}
	}

	// there must be only one corrupted result during a wintermute attack, and
	// that corrupted result must be dictated to all corrupted execution ids.
	require.Len(t, dictatedResults, 1)
	for _, actualCorrEnIds := range dictatedResults {
		require.ElementsMatch(t, corrEnIds, actualCorrEnIds)
	}

	// number of bounced receipts should match the expected value.
	actualBouncedReceiptCount := 0
	for _, originalReceiptId := range orgReceiptIds {
		if bouncedReceipts.Contains(originalReceiptId) {
			actualBouncedReceiptCount++
		}
	}
	require.Equal(t, expBouncedReceiptCount, actualBouncedReceiptCount)
}

func chunkDataPackRequestsSanityCheck(t *testing.T,
	outputEvents []*insecure.Event, // list of all output events of the wintermute orchestrator.
	originalChunkRequestIds flow.IdentifierList, // list of original chunk data requests sent by corrupted verification nodes.
	expectedBouncedBackReqs int, // number of expected bounced back chunk data pack requests.
) {

	bouncedBackRequestedChunkIds := flow.IdentifierList{}

	for _, outputEvent := range outputEvents {
		switch event := outputEvent.FlowProtocolEvent.(type) {
		case *messages.ChunkDataRequest:
			bouncedBackRequestedChunkIds = bouncedBackRequestedChunkIds.Union(flow.IdentifierList{event.ChunkID})
		}
	}

	// number of bounced receipts should match the expected value.
	actualBouncedChunkDataRequestCount := 0
	for _, chunkId := range bouncedBackRequestedChunkIds {
		if originalChunkRequestIds.Contains(chunkId) {
			actualBouncedChunkDataRequestCount++
		}
	}
	require.Equal(t, expectedBouncedBackReqs, actualBouncedChunkDataRequestCount)
}

// receiptsWithDistinctResultFixture creates a set of execution receipts (with distinct result) one per given executor id.
// It returns a map of execution receipts to their relevant attack network events.
func receiptsWithDistinctResultFixture(
	t *testing.T,
	count int,
	exeIds flow.IdentifierList,
	targetIds flow.IdentifierList,
) (map[flow.Identifier]*insecure.Event, []*flow.ExecutionReceipt) {

	// list of execution receipts
	receipts := make([]*flow.ExecutionReceipt, 0)

	// map of execution receipt ids to their event.
	eventMap := make(map[flow.Identifier]*insecure.Event)

	for i := 0; i < count; i++ {
		for _, exeId := range exeIds {
			receipt := unittest.ExecutionReceiptFixture(unittest.WithExecutorID(exeId))
			event := executionReceiptEvent(receipt, targetIds)

			_, ok := eventMap[receipt.ID()]
			require.False(t, ok) // checks for duplicate receipts.

			receipts = append(receipts, receipt)
			eventMap[receipt.ID()] = event
		}
	}

	require.Len(t, eventMap, count*len(exeIds))
	return eventMap, receipts
}

// receiptsWithSameResultFixture creates a set of receipts (all with the same result) per given executor id.
// It returns a map of execution receipts to their relevant attack network events.
func receiptsWithSameResultFixture(
	t *testing.T,
	count int, // total receipts per execution id.
	exeIds flow.IdentifierList, // identifier of execution nodes.
	targetIds flow.IdentifierList, // target recipients of the execution receipts.
) (map[flow.Identifier]*insecure.Event, []*flow.ExecutionReceipt) {
	// list of execution receipts
	receipts := make([]*flow.ExecutionReceipt, 0)

	// map of execution receipt ids to their event.
	eventMap := make(map[flow.Identifier]*insecure.Event)

	// generates "count"-many receipts per execution nodes with the same
	// set of results.
	for i := 0; i < count; i++ {

		result := unittest.ExecutionResultFixture()

		for _, exeId := range exeIds {
			receipt := unittest.ExecutionReceiptFixture(
				unittest.WithExecutorID(exeId),
				unittest.WithResult(result))

			require.Equal(t, result.ID(), receipt.ExecutionResult.ID())

			event := executionReceiptEvent(receipt, targetIds)

			_, ok := eventMap[receipt.ID()]
			require.False(t, ok) // check for duplicate receipts.

			receipts = append(receipts, receipt)
			eventMap[receipt.ID()] = event
		}
	}

	require.Len(t, eventMap, count*len(exeIds))
	return eventMap, receipts
}

// executionReceiptEvent creates the attack network event of the corresponding execution receipt.
func executionReceiptEvent(receipt *flow.ExecutionReceipt, targetIds flow.IdentifierList) *insecure.Event {
	return &insecure.Event{
		CorruptedId:       receipt.ExecutorID,
		Channel:           engine.PushReceipts,
		Protocol:          insecure.Protocol_UNICAST,
		TargetIds:         targetIds,
		FlowProtocolEvent: receipt,
	}
}

// chunkDataPackResponseForReceipts creates and returns chunk data pack response as well as their corresponding events for the given set of receipts.
func chunkDataPackResponseForReceipts(receipts []*flow.ExecutionReceipt, verIds flow.IdentifierList) ([]*insecure.Event, flow.IdentifierList) {
	chunkIds := flow.IdentifierList{}
	responseList := make([]*insecure.Event, 0)

	for _, receipt := range receipts {
		result := receipt.ExecutionResult
		for _, chunk := range result.Chunks {
			chunkId := chunk.ID()

			if chunkIds.Contains(chunkId) {
				// chunk data pack request already created
				continue
			}

			cdpRep := &messages.ChunkDataResponse{
				ChunkDataPack: *unittest.ChunkDataPackFixture(chunkId),
			}
			chunkIds = chunkIds.Union(flow.IdentifierList{chunkId})

			// creates a request event per verification node
			for _, verId := range verIds {
				event := &insecure.Event{
					CorruptedId:       receipt.ExecutorID,
					Channel:           engine.RequestChunks,
					Protocol:          insecure.Protocol_PUBLISH,
					TargetNum:         0,
					TargetIds:         flow.IdentifierList{verId},
					FlowProtocolEvent: cdpRep,
				}

				responseList = append(responseList, event)
			}
		}
	}

	return responseList, chunkIds
}

// TestRespondingWithCorruptedAttestation evaluates when the Wintermute orchestrator receives a chunk data pack request from a CORRUPTED
//	verification node for a CORRUPTED chunk, it replies that with a result approval attestation.
func TestRespondingWithCorruptedAttestation(t *testing.T) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	corruptedVerIds := flow.IdentifierList(corruptedIds.Filter(filter.HasRole(flow.RoleVerification)).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(allIds, corruptedIds, unittest.Logger())

	originalResult := unittest.ExecutionResultFixture()
	corruptedResult := unittest.ExecutionResultFixture(unittest.WithChunks(uint(totalChunks)))
	wintermuteOrchestrator.state = &attackState{
		originalResult:         originalResult,
		corruptedResult:        corruptedResult,
		originalChunkIds:       flow.GetIDs(originalResult.Chunks),
		corruptedChunkIds:      flow.GetIDs(corruptedResult.Chunks),
		corruptedChunkIndexMap: chunkIndexMap(corruptedResult.Chunks),
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
			require.True(t, wintermuteOrchestrator.state.corruptedChunkIds.Contains(chunk.ID())) // attestation must be for a corrupted chunk

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
	corruptedVerIds := flow.IdentifierList(corruptedIds.Filter(filter.HasRole(flow.RoleVerification)).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(allIds, corruptedIds, unittest.Logger())

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
		originalResult:         originalResult,
		corruptedResult:        corruptedResult,
		originalChunkIds:       flow.GetIDs(originalResult.Chunks),
		corruptedChunkIds:      flow.GetIDs(corruptedResult.Chunks),
		corruptedChunkIndexMap: chunkIndexMap(corruptedResult.Chunks),
	}

	testBouncingBackChunkDataResponse(t, state)
}

// testBouncingBackChunkDataRequests evaluates all chunk data pack responses that do not match the corrupted chunk are bounced back from
//orchestrator.
func testBouncingBackChunkDataResponse(t *testing.T, state *attackState) {
	totalChunks := 10
	_, allIds, corruptedIds := bootstrapWintermuteFlowSystem(t)
	verIds := flow.IdentifierList(allIds.Filter(filter.HasRole(flow.RoleVerification)).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(allIds, corruptedIds, unittest.Logger())

	wintermuteOrchestrator.state = state

	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(
			unittest.ExecutionResultFixture(
				unittest.WithChunks(uint(totalChunks)))))
	// creates chunk data pack response for all verification nodes.
	cdpReps, _ := chunkDataPackResponseForReceipts([]*flow.ExecutionReceipt{receipt}, allIds.NodeIDs())

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
			filter.Not(filter.In(corruptedIds)))).NodeIDs())
	wintermuteOrchestrator := NewOrchestrator(allIds, corruptedIds, unittest.Logger())

	originalResult := unittest.ExecutionResultFixture()
	corruptedResult := unittest.ExecutionResultFixture(unittest.WithChunks(uint(totalChunks)))
	state := &attackState{
		originalResult:         originalResult,
		corruptedResult:        corruptedResult,
		originalChunkIds:       flow.GetIDs(originalResult.Chunks),
		corruptedChunkIds:      flow.GetIDs(corruptedResult.Chunks),
		corruptedChunkIndexMap: chunkIndexMap(corruptedResult.Chunks),
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
