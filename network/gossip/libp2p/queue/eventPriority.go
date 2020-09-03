package queue

import (
	"math"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

const (
	_   = iota
	KiB = 1 << (10 * iota)
	MiB
)

// QueueMessage is the message that is enqueued for each incoming message
type QueueMessage struct {
	Payload   interface{}     // the decoded message
	Size      int             // the size of the message in bytes
	ChannelID uint8           // the channel id to use to lookup the engine
	SenderID  flow.Identifier // senderID for logging
}

// GetEventPriority returns the priority of the flow event message.
// It is an average of the priority by message type and priority by message size
func GetEventPriority(message interface{}) Priority {
	qm := message.(QueueMessage)
	priorityByType := getPriorityByType(qm.Payload)
	priorityBySize := getPriorityBySize(qm.Size)
	return Priority(math.Ceil(float64(priorityByType+priorityBySize) / 2))
}

// getPriorityByType returns the priority of a message by it's type
func getPriorityByType(message interface{}) Priority {
	switch message.(type) {
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

// getPriorityBySize returns a priority of a message by size. Smaller messages have higher priority than larger ones.
func getPriorityBySize(size int) Priority {
	switch {
	case size > MiB:
		return Low_Priority
	case size > KiB:
		return Medium_Priority
	default:
		return High_Priority
	}
}
