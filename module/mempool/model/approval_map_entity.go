package model

import (
	"github.com/dapperlabs/flow-go/model/flow"
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
func (a ApprovalMapEntity) ID() flow.Identifier {
	return a.ChunkKey
}

// CheckSum implements flow.Entity.CheckSum for ApprovalMapEntity to make it
// capable of being stored directly in mempools and storage. It makes the id of
// the entire ApprovalMapEntity.
func (a ApprovalMapEntity) Checksum() flow.Identifier {
	return flow.MakeID(a)
}
