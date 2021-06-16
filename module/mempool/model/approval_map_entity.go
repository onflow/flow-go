package model

import (
	"os"
	"strconv"
	"sync"

	"github.com/aristanetworks/goarista/monotime"
	"github.com/onflow/flow-go/model/flow"
)

var once sync.Once

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

var logfile_approvalmap *os.File

// CheckSum implements flow.Entity.CheckSum for ApprovalMapEntity to make it
// capable of being stored directly in mempools and storage. It makes the id of
// the entire ApprovalMapEntity.
func (a *ApprovalMapEntity) Checksum() flow.Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/tmp/makeid-investigation/module/mempool/model/approval_map_entity.log")
		logfile_approvalmap = newfile
	})
	ts := monotime.Now()
	defer logfile_approvalmap.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return flow.MakeID(a)
}
