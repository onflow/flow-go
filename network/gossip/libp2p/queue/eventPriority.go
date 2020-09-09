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
func GetEventPriority(message interface{}) (Priority, error) {
	qm := message.(QueueMessage)
	priorityByType := getPriorityByType(qm.Payload)
	priorityBySize := getPriorityBySize(qm.Size)
	return Priority(math.Ceil(float64(priorityByType+priorityBySize) / 2)), nil
}

// getPriorityByType returns the priority of a message by it's type
func getPriorityByType(message interface{}) Priority {
	switch message.(type) {
	// consensus
	case *messages.BlockProposal:
		return HighPriority
	case *messages.BlockVote:
		return HighPriority

	// protocol state sync
	case *messages.SyncRequest:
		return LowPriority
	case *messages.SyncResponse:
		return LowPriority
	case *messages.RangeRequest:
		return LowPriority
	case *messages.BatchRequest:
		return LowPriority
	case *messages.BlockResponse:
		return LowPriority

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return HighPriority
	case *messages.ClusterBlockVote:
		return HighPriority
	case *messages.ClusterBlockResponse:
		return HighPriority

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return HighPriority
	case *flow.TransactionBody:
		return HighPriority
	case *flow.Transaction:
		return HighPriority

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return HighPriority
	case *flow.ResultApproval:
		return HighPriority

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return MediumPriority
	case *messages.ExecutionStateDelta:
		return MediumPriority

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return HighPriority
	case *messages.ChunkDataResponse:
		return HighPriority

	// generic entity exchange engines
	case *messages.EntityRequest:
		return LowPriority
	case *messages.EntityResponse:
		return LowPriority

	default:
		return MediumPriority
	}
}

// getPriorityBySize returns a priority of a message by size. Smaller messages have higher priority than larger ones.
func getPriorityBySize(size int) Priority {
	switch {
	case size > MiB:
		return LowPriority
	case size > KiB:
		return MediumPriority
	default:
		return HighPriority
	}
}
