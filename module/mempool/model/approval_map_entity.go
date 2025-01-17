package model

import (
	"github.com/onflow/flow-go/model/flow"
)

// ApprovalMapEntity is an internal data structure for the approval mempool.
// It implements a key-value entry where a chunk is associated with a map of
// approvals indexed by approver ID.
type ApprovalMapEntity struct {
	ChunkKey   flow.Identifier
	ResultID   flow.Identifier
	ChunkIndex uint64
	Approvals  map[flow.Identifier]*flow.ResultApproval // [approver_id] => approval
}

// ID implements flow.Entity.ID for ApprovalMapEntity to make it capable of
// being stored directly in mempools and storage.
func (a *ApprovalMapEntity) ID() flow.Identifier {
	return a.ChunkKey
}

func (a *ApprovalMapEntity) Hash() flow.Identifier {
	return flow.MakeID(a)
}
