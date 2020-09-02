package queue

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/network"
)

type EventPriority struct {
	codec network.Codec
}

func GetEventPriority(message interface{}) (Priority, error) {

}

func getPriorityByType(message interface{}) Priority {
	switch v.(type) {
	// consensus
	case *messages.BlockProposal:
		return High_Priority
	case *messages.BlockVote:
		return High_Priority

	// protocol state sync
	case *messages.SyncRequest:
		return Low_Priority
	case *messages.SyncResponse:
		return Low_Priority
	case *messages.RangeRequest:
		return Low_Priority
	case *messages.BatchRequest:
		return Low_Priority
	case *messages.BlockResponse:
		return Low_Priority

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return High_Priority
	case *messages.ClusterBlockVote:
		return High_Priority
	case *messages.ClusterBlockResponse:
		return High_Priority

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return High_Priority
	case *flow.TransactionBody:
		return High_Priority
	case *flow.Transaction:
		return High_Priority

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return High_Priority
	case *flow.ResultApproval:
		return High_Priority

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return Medium_Priority
	case *messages.ExecutionStateDelta:
		return Medium_Priority

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return High_Priority
	case *messages.ChunkDataResponse:
		return High_Priority

	// generic entity exchange engines
	case *messages.EntityRequest:
		return Low_Priority
	case *messages.EntityResponse:
		return Low_Priority

	default:
		return Medium_Priority
	}
}
