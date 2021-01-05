package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type ResultApprovals interface {

	// Store stores a ResultApproval
	Store(result *flow.ResultApproval) error

	// Index indexes a ResultApproval by result ID and chunk index
	Index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error

	// ByID retrieves a ResultApproval by its ID
	ByID(approvalID flow.Identifier) (*flow.ResultApproval, error)

	// ByChunk retrieves a ResultApproval by result ID and chunk index
	ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error)
}
