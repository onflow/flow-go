package wintermute

import (
	"testing"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// chunkDataPackRequestForReceipts is a test helper that creates and returns chunk data pack requests as well as their corresponding events for the
// given set of receipts.
func chunkDataPackRequestForReceipts(
	_ *testing.T, // emphasizing this is a test helper.
	receipts []*flow.ExecutionReceipt, // set of receipts for which chunk data pack requests are created.
	corVnIds flow.IdentifierList, // identifier of corrupted verification nodes.
	// returns:
	// map of chunk ids -> chunk data pack requests from each of corrupted verificaiton nodes.
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
