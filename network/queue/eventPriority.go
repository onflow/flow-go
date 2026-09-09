package queue

import (
	"fmt"
	"math"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
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
// Type priority is the primary ordering factor; size priority is only a secondary
// tie-breaker within the same type priority.
func GetEventPriority(message any) (Priority, error) {
	qm, ok := message.(QMessage)
	if !ok {
		return 0, fmt.Errorf("invalid message format: %T", message)
	}
	priorityByType := getPriorityByType(qm.Payload)
	priorityBySize := getPriorityBySize(qm.Size)

	// Weight type priority so that it always dominates size priority. The weight
	// must be larger than the largest possible gap between size priorities so that
	// even a high size priority cannot promote a low type priority above a medium
	// type priority, and similarly for medium vs high.
	const typePriorityWeight = 4
	const sizePriorityWeight = 1
	const sumOfWeights = typePriorityWeight + sizePriorityWeight

	weighted := float64(typePriorityWeight*int(priorityByType)+sizePriorityWeight*int(priorityBySize)) / sumOfWeights
	return Priority(math.Ceil(weighted)), nil
}

// getPriorityByType maps a message to its priority based on its internal type.
//
// The network layer converts wire messages (model/messages) to their internal
// representation via ToInternal before enqueueing them (see
// [Network.processAuthenticatedMessage]), so the queue only ever sees the internal
// types switched on here. The mapping is many-to-one: several wire types collapse
// onto the same internal type (e.g. consensus and cluster block votes both convert
// to [flow.BlockVote]).
//
// Priorities follow the message's role: consensus, execution, and verification
// traffic is high priority; request/response exchanges that drive bulk data sync
// are medium; protocol-state sync and generic entity exchange are low.
func getPriorityByType(message any) Priority {
	switch message.(type) {
	// consensus: proposals, votes, and timeouts are all required for liveness.
	// [flow.BlockVote] also covers cluster block votes, and [model.TimeoutObject]
	// also covers cluster timeout objects (their wire types convert to the same
	// internal types).
	case *flow.Proposal:
		return HighPriority
	case *flow.BlockVote:
		return HighPriority
	case *model.TimeoutObject:
		return HighPriority

	// cluster consensus (effectively collections)
	case *cluster.Proposal:
		return HighPriority
	case *cluster.BlockResponse:
		return HighPriority

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return HighPriority
	case *flow.TransactionBody:
		return HighPriority

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return HighPriority
	case *flow.ResultApproval:
		return HighPriority

	// data exchange for execution of blocks
	case *flow.ChunkDataRequest:
		return HighPriority
	case *flow.ChunkDataResponse:
		return HighPriority

	// block sync responses are latency-critical for catching up
	case *flow.BlockResponse:
		return HighPriority

	// protocol state sync requests
	case *flow.RangeRequest:
		return MediumPriority
	case *flow.BatchRequest:
		return MediumPriority

	// request/response for result approvals
	case *flow.ApprovalRequest:
		return MediumPriority
	case *flow.ApprovalResponse:
		return MediumPriority

	// DKG messages are exchanged only during epoch setup; low volume and not
	// latency-sensitive relative to consensus traffic
	case *flow.DKGMessage:
		return MediumPriority

	// protocol state sync and generic entity exchange engines
	case *flow.SyncRequest:
		return LowPriority
	case *flow.SyncResponse:
		return LowPriority
	case *flow.EntityRequest:
		return LowPriority
	case *flow.EntityResponse:
		return LowPriority

	// test message
	case *flow.TestMessage:
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
