package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
)

// UntrustedMessage represents the set of allowed decode target types for messages received over the network.
// Conceptually, an UntrustedMessage implementation makes no guarantees whatsoever about its contents.
// UntrustedMessage's must implement a ToInternal method, which converts the network message to a corresponding internal type (typically in the model/flow package)
// This conversion provides an opportunity to:
//   - perform basic structural validity checks (required fields are non-nil, fields reference one another in a consistent way)
//   - attach auxiliary information (for example, caching the hash of a model)
//
// Internal models abide by basic structural validity requirements, but are not trusted.
// They may still represent invalid or Byzantine inputs in the context of the broader application state.
// It is the responsibility of engines operating on these models to fully validate them.
type UntrustedMessage interface {

	// ToInternal returns the internal type (from flow.* constructors) representation.
	// All errors indicate that the decode target contains a structurally invalid representation of the internal model.
	ToInternal() (any, error)
}

// InternalToMessage converts an internal types into the
// corresponding UntrustedMessage for network.
//
// This is the inverse of ToInternal: instead of decoding a network
// message into an internal model, it wraps or casts internal objects
// so they can be encoded and sent over the network. Encoding and always
// requires an UntrustedMessage.
//
// No errors are expected during normal operation.
// TODO: investigate how to eliminate this workaround in both ghost/rpc.go and corruptnet/message_processor.go
func InternalToMessage(event any) (UntrustedMessage, error) {
	switch internal := event.(type) {
	case *flow.Proposal:
		return (*Proposal)(internal), nil
	case *cluster.Proposal:
		return (*ClusterProposal)(internal), nil
	case *flow.EntityRequest:
		return (*EntityRequest)(internal), nil
	case *flow.EntityResponse:
		return (*EntityResponse)(internal), nil
	case *flow.TransactionBody:
		return (*TransactionBody)(internal), nil
	case *flow.CollectionGuarantee:
		return (*CollectionGuarantee)(internal), nil
	case *flow.SyncRequest:
		return (*SyncRequest)(internal), nil
	case *flow.SyncResponse:
		return (*SyncResponse)(internal), nil
	case *flow.BatchRequest:
		return (*BatchRequest)(internal), nil
	case *flow.ChunkDataRequest:
		return (*ChunkDataRequest)(internal), nil
	case *flow.ChunkDataResponse:
		return &ChunkDataResponse{
			ChunkDataPack: flow.UntrustedChunkDataPack(internal.ChunkDataPack),
			Nonce:         internal.Nonce,
		}, nil
	case *flow.RangeRequest:
		return (*RangeRequest)(internal), nil
	case *flow.ExecutionReceipt:
		return (*ExecutionReceipt)(internal), nil
	case *flow.ResultApproval:
		return (*ResultApproval)(internal), nil
	case *flow.TestMessage:
		return (*message.TestMessage)(internal), nil
	default:
		return nil, fmt.Errorf("cannot convert unsupported type %T", event)
	}
}
