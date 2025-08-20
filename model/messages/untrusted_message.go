package messages

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// UntrustedMessage represents the set of allowed decode target types for messages received over the network.
// Conceptually, an UntrustedMessage implementation makes no guarantees whatsoever about its contents.
// UntrustedMessage's must implement a ToInternal method, which converts the network message to a corresponding internal type (typically in the model/flow package)
// This conversion provides an opportunity to:
//    - perform basic structural validity checks (required fields are non-nil, fields reference one another in a consistent way)
//    - attach auxiliary information (for example, caching the hash of a model)
// Internal models abide by basic structural validity requirements, but are not trusted.
// They may still represent invalid or Byzantine inputs in the context of the broader application state.
// It is the responsibility of engines operating on these models to fully validate them.
type UntrustedMessage interface {

	// ToInternal returns the internal type (from flow.* constructors) representation.
	// All errors indicate that the decode target contains a structurally invalid representation of the internal model.
	ToInternal() (any, error)
}

var (
	_ UntrustedMessage = (*flow.UntrustedProposal)(nil)
	_ UntrustedMessage = (*BlockVote)(nil)
	_ UntrustedMessage = (*TimeoutObject)(nil)

	_ UntrustedMessage = (*cluster.UntrustedProposal)(nil)
	_ UntrustedMessage = (*ClusterBlockVote)(nil)
	_ UntrustedMessage = (*ClusterBlockResponse)(nil)
	_ UntrustedMessage = (*ClusterTimeoutObject)(nil)

	_ UntrustedMessage = (*SyncRequest)(nil)
	_ UntrustedMessage = (*SyncResponse)(nil)
	_ UntrustedMessage = (*RangeRequest)(nil)
	_ UntrustedMessage = (*BatchRequest)(nil)
	_ UntrustedMessage = (*BlockResponse)(nil)

	_ UntrustedMessage = (*flow.CollectionGuarantee)(nil)
	_ UntrustedMessage = (*flow.TransactionBody)(nil)
	_ UntrustedMessage = (*flow.Transaction)(nil)

	_ UntrustedMessage = (*flow.ExecutionReceipt)(nil)
	_ UntrustedMessage = (*flow.ResultApproval)(nil)

	_ UntrustedMessage = (*ChunkDataRequest)(nil)
	_ UntrustedMessage = (*ChunkDataResponse)(nil)

	_ UntrustedMessage = (*ApprovalRequest)(nil)
	_ UntrustedMessage = (*ApprovalResponse)(nil)

	_ UntrustedMessage = (*EntityRequest)(nil)
	_ UntrustedMessage = (*EntityResponse)(nil)

	_ UntrustedMessage = (*DKGMessage)(nil)
)
