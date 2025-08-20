package messages

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// UntrustedMessage is the decode target for anything that came over the wire.
// ToInternal must validate and construct the internal, trusted model.
// Implementations must validate and construct the trusted, internal model.
type UntrustedMessage interface {

	// ToInternal returns the validated internal type (from flow.* constructors)
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
