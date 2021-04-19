package requester

import (
	"github.com/onflow/flow-go/model/verification"
)

type ChunkDataRequestQualifier interface {
	CanTry(status verification.ChunkRequestStatus) bool
	UpdateStatus(status *verification.ChunkDataPackRequest) bool
}
