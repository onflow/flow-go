package wintermute

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
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
) (map[flow.Identifier][]*insecure.Event, flow.IdentifierList) {

	// stratifies result ids based on executor.
	executorIds := make(map[flow.Identifier]flow.IdentifierList)
	for _, receipt := range receipts {
		resultId := receipt.ExecutionResult.ID()
		executorIds[resultId] = flow.IdentifierList{receipt.ExecutorID}.Union(executorIds[resultId])
	}

	chunkIds := flow.IdentifierList{}
	cdpReqMap := make(map[flow.Identifier][]*insecure.Event)
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

			requests := make([]*insecure.Event, 0)

			// creates a request event per verification node
			for _, verId := range corVnIds {
				event := &insecure.Event{
					CorruptedId:       verId,
					Channel:           engine.RequestChunks,
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
