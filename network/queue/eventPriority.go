package queue

import (
	"fmt"
	"math"

	"github.com/onflow/flow-go/model/flow"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
)

const (
	_   = iota
	KiB = 1 << (10 * iota)
	MiB
)

// QMessage is the message that is enqueued for each incoming message
type QMessage struct {
	Payload  any              // the decoded message
	Size     int              // the size of the message in bytes
	Target   channels.Channel // the target channel to lookup the engine
	SenderID flow.Identifier  // senderID for logging
}

// GetEventPriority returns the priority of the flow event message.
// It is an average of the priority by message type and priority by message size
func GetEventPriority(message any) (Priority, error) {
	qm, ok := message.(QMessage)
	if !ok {
		return 0, fmt.Errorf("invalid message format: %T", message)
	}
	priorityByType := getPriorityByType(qm.Payload)
	priorityBySize := getPriorityBySize(qm.Size)
	return Priority(math.Ceil(float64(priorityByType+priorityBySize) / 2)), nil
}

// getPriorityByType maps a message type to its priority
func getPriorityByType(message any) Priority {
	switch message.(type) {
	// consensus
	case *messages.Proposal:
		return HighPriority
	case *messages.BlockVote:
		return HighPriority

	// protocol state sync
	case *messages.SyncRequest:
		return LowPriority
	case *messages.SyncResponse:
		return LowPriority
	case *messages.RangeRequest:
		return MediumPriority
	case *messages.BatchRequest:
		return MediumPriority
	case *messages.BlockResponse:
		return HighPriority

	// cluster consensus (effectively collections)
	case *messages.ClusterProposal:
		return HighPriority
	case *messages.ClusterBlockVote:
		return HighPriority
	case *messages.ClusterBlockResponse:
		return HighPriority

	// collections, guarantees & transactions
	case *messages.CollectionGuarantee:
		return HighPriority
	case *messages.TransactionBody:
		return HighPriority

	// core messages for execution & verification
	case *messages.ExecutionReceipt:
		return HighPriority
	case *messages.ResultApproval:
		return HighPriority

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return HighPriority
	case *messages.ChunkDataResponse:
		return HighPriority

	// request/response for result approvals
	case *messages.ApprovalRequest:
		return MediumPriority
	case *messages.ApprovalResponse:
		return MediumPriority

	// generic entity exchange engines
	case *messages.EntityRequest:
		return LowPriority
	case *messages.EntityResponse:
		return LowPriority

	// test message
	case *libp2pmessage.TestMessage:
		return LowPriority

	// anything else
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
