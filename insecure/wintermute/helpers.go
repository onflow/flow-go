package wintermute

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	enginemock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

// chunkDataPackRequestForReceipts is a test helper that creates and returns chunk data pack requests as well as their corresponding events for the
// given set of receipts.
func chunkDataPackRequestForReceipts(
	_ *testing.T, // emphasizing this is a test helper.
	receipts []*flow.ExecutionReceipt, // set of receipts for which chunk data pack requests are created.
	corVnIds flow.IdentifierList, // identifier of corrupted verification nodes.
	// returns:
	// map of chunk ids -> chunk data pack requests from each of corrupted verification nodes.
	// list of chunk ids in the receipt.
) (map[flow.Identifier][]*insecure.EgressEvent, flow.IdentifierList) {

	// stratifies result ids based on executor.
	executorIds := make(map[flow.Identifier]flow.IdentifierList)
	for _, receipt := range receipts {
		resultId := receipt.ExecutionResult.ID()
		executorIds[resultId] = flow.IdentifierList{receipt.ExecutorID}.Union(executorIds[resultId])
	}

	chunkIds := flow.IdentifierList{}
	cdpReqMap := make(map[flow.Identifier][]*insecure.EgressEvent)
	for _, receipt := range receipts {
		result := receipt.ExecutionResult
		for _, chunk := range result.Chunks {
			chunkId := chunk.ID()

			if _, ok := cdpReqMap[chunkId]; ok {
				// chunk data pack request already created
				continue
			}

			cdpReq := &messages.ChunkDataRequest{
				ChunkID: chunkId,
			}
			chunkIds = chunkIds.Union(flow.IdentifierList{chunkId})

			requests := make([]*insecure.EgressEvent, 0)

			// creates a request event per verification node
			for _, verId := range corVnIds {
				event := &insecure.EgressEvent{
					CorruptOriginId:   verId,
					Channel:           channels.RequestChunks,
					Protocol:          insecure.Protocol_PUBLISH,
					TargetNum:         0,
					TargetIds:         executorIds[result.ID()],
					FlowProtocolEvent: cdpReq,
				}

				requests = append(requests, event)
			}

			cdpReqMap[chunkId] = requests
		}
	}

	return cdpReqMap, chunkIds
}

// receiptsWithSameResultFixture creates a set of receipts (all with the same result) per given executor id.
// It returns a map of execution receipts to their relevant orchestrator network events.
func receiptsWithSameResultFixture(
	t *testing.T,
	count int, // total receipts per execution id.
	exeIds flow.IdentifierList, // identifier of execution nodes.
	targetIds flow.IdentifierList, // target recipients of the execution receipts.
) (map[flow.Identifier]*insecure.EgressEvent, []*flow.ExecutionReceipt) {
	// list of execution receipts
	receipts := make([]*flow.ExecutionReceipt, 0)

	// map of execution receipt ids to their event.
	eventMap := make(map[flow.Identifier]*insecure.EgressEvent)

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

// executionReceiptEvent creates the orchestrator network event of the corresponding execution receipt.
func executionReceiptEvent(receipt *flow.ExecutionReceipt, targetIds flow.IdentifierList) *insecure.EgressEvent {
	return &insecure.EgressEvent{
		CorruptOriginId:   receipt.ExecutorID,
		Channel:           channels.PushReceipts,
		Protocol:          insecure.Protocol_UNICAST,
		TargetIds:         targetIds,
		FlowProtocolEvent: receipt,
	}
}

// chunkDataPackResponseForReceipts creates and returns chunk data pack response as well as their corresponding events for the given set of receipts.
func chunkDataPackResponseForReceipts(receipts []*flow.ExecutionReceipt, verIds flow.IdentifierList) ([]*insecure.EgressEvent, flow.IdentifierList) {
	chunkIds := flow.IdentifierList{}
	responseList := make([]*insecure.EgressEvent, 0)

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
				event := &insecure.EgressEvent{
					CorruptOriginId:   receipt.ExecutorID,
					Channel:           channels.RequestChunks,
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

// bootstrapWintermuteFlowSystem bootstraps flow network with following setup:
// verification nodes: 3 corrupted + 1 honest
// execution nodes: 2 corrupted + 1 honest
// other roles at the minimum required number and all honest.
func bootstrapWintermuteFlowSystem(t *testing.T) (*enginemock.StateFixture, flow.IdentityList, flow.IdentifierList) {
	// creates identities to bootstrap system with
	corruptedVnIds := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleVerification))
	corruptedEnIds := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	identities := unittest.CompleteIdentitySet(append(corruptedVnIds, corruptedEnIds...)...)
	identities = append(identities, unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)))    // one honest execution node
	identities = append(identities, unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))) // one honest verification node

	// bootstraps the system
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	stateFixture := testutil.CompleteStateFixture(t, zerolog.Nop(), metrics.NewNoopCollector(), trace.NewNoopTracer(), rootSnapshot)

	return stateFixture, identities, append(corruptedEnIds, corruptedVnIds...).NodeIDs()
}

// orchestratorOutputSanityCheck performs a sanity check on the output events dictated by the wintermute orchestrator to the corrupted nodes.
// It checks that: (1) exactly one execution result is dictated to ALL corrupted execution nodes. (2) except that one execution result, all other
// incoming execution receipts are bounced back to corrupted execution node who sent it originally.
//
// An execution result is "dictated" when orchestrator corrupts a given result.
// An execution receipt is "bounced" back when orchestrator doesn't tamper with it, and let it go to the flow network as it is.
func orchestratorOutputSanityCheck(
	t *testing.T,
	outputEvents []*insecure.EgressEvent, // list of all output events of the wintermute orchestrator.
	corrEnIds flow.IdentifierList, // list of all corrupted execution node ids.
	orgReceiptIds flow.IdentifierList, // list of all execution receipt ids originally sent to orchestrator.
	expBouncedReceiptCount int, // expected number of execution receipts that must remain uncorrupted.
) {

	// keeps a map of (corrupted results ids -> execution node ids)
	dictatedResults := make(map[flow.Identifier]flow.IdentifierList)

	// keeps a list of all pass through receipts.
	passThroughReceipts := flow.IdentifierList{}

	for _, outputEvent := range outputEvents {
		switch event := outputEvent.FlowProtocolEvent.(type) {
		case *flow.ExecutionReceipt:
			if len(event.ExecutorSignature.Bytes()) != 0 {
				// a receipt with a non-empty signature is a pass-through receipt.
				// makes sure sender is a corrupted execution node.
				ok := corrEnIds.Contains(outputEvent.CorruptOriginId)
				require.True(t, ok)
				// uses union to avoid adding duplicate.
				passThroughReceipts = passThroughReceipts.Union(flow.IdentifierList{event.ID()})
			} else {
				// a receipt with an empty signature contains a dictated result from wintermute orchestrator.
				// the rest of receipt will be filled by the corrupted node
				resultId := event.ExecutionResult.ID()
				if dictatedResults[resultId] == nil {
					dictatedResults[resultId] = flow.IdentifierList{}
				}
				// uses union to avoid adding duplicate.
				dictatedResults[resultId] = dictatedResults[resultId].Union(flow.IdentifierList{outputEvent.CorruptOriginId})
			}
		}
	}

	// there must be only one corrupted result during a wintermute attack, and
	// that corrupted result must be dictated to all corrupted execution ids.
	require.Len(t, dictatedResults, 1)
	for _, actualCorrEnIds := range dictatedResults {
		require.ElementsMatch(t, corrEnIds, actualCorrEnIds)
	}

	// number of passed through receipts should match the expected value.
	actualPassedThroughReceiptCount := 0
	for _, originalReceiptId := range orgReceiptIds {
		if passThroughReceipts.Contains(originalReceiptId) {
			actualPassedThroughReceiptCount++
		}
	}
	require.Equal(t, expBouncedReceiptCount, actualPassedThroughReceiptCount)
}

// receiptsWithDistinctResultFixture creates a set of execution receipts (with distinct result) one per given executor id.
// It returns a map of execution receipts to their relevant orchestrator network events.
func receiptsWithDistinctResultFixture(
	t *testing.T,
	count int,
	exeIds flow.IdentifierList,
	targetIds flow.IdentifierList,
) (map[flow.Identifier]*insecure.EgressEvent, []*flow.ExecutionReceipt) {

	// list of execution receipts
	receipts := make([]*flow.ExecutionReceipt, 0)

	// map of execution receipt ids to their event.
	eventMap := make(map[flow.Identifier]*insecure.EgressEvent)

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
