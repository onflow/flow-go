package queue

import (
	"fmt"
	"math"

	"github.com/dapperlabs/flow-go/model/flow"
	testmessage "github.com/dapperlabs/flow-go/model/libp2p/message"
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
	qm, ok := message.(QueueMessage)
	if !ok {
		return 0, fmt.Errorf("invalid message format: %T", message)
	}
	priorityByType, err := getPriorityByType(qm.Payload)
	if err != nil {
		return 0, err
	}
	priorityBySize := getPriorityBySize(qm.Size)
	return Priority(math.Ceil(float64(priorityByType+priorityBySize) / 2)), nil
}

// getPriorityByType returns maps a message type to its priority
func getPriorityByType(message interface{}) (Priority, error) {
	switch t := message.(type) {
	// consensus
	case *messages.BlockProposal:
		return HighPriority, nil
	case *messages.BlockVote:
		return HighPriority, nil

	// protocol state sync
	case *messages.SyncRequest:
		return LowPriority, nil
	case *messages.SyncResponse:
		return LowPriority, nil
	case *messages.RangeRequest:
		return LowPriority, nil
	case *messages.BatchRequest:
		return LowPriority, nil
	case *messages.BlockResponse:
		return LowPriority, nil

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return HighPriority, nil
	case *messages.ClusterBlockVote:
		return HighPriority, nil
	case *messages.ClusterBlockResponse:
		return HighPriority, nil

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return HighPriority, nil
	case *flow.TransactionBody:
		return HighPriority, nil
	case *flow.Transaction:
		return HighPriority, nil

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return HighPriority, nil
	case *flow.ResultApproval:
		return HighPriority, nil

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return MediumPriority, nil
	case *messages.ExecutionStateDelta:
		return MediumPriority, nil

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return HighPriority, nil
	case *messages.ChunkDataResponse:
		return HighPriority, nil

	// generic entity exchange engines
	case *messages.EntityRequest:
		return LowPriority, nil
	case *messages.EntityResponse:
		return LowPriority, nil

	// test message
	case *testmessage.TestMessage:
		return LowPriority, nil

	// anything else
	default:
		return 0, fmt.Errorf("priroity not found for %T", t)
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
