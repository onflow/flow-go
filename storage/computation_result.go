package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// ComputationResultUploadStatus interface defines storage operations for upload status
// of given ComputationResult instance:
//	- false as upload not completed
//  - true as upload completed
//
type ComputationResultUploadStatus interface {
	// Upsert upserts omputationResult into persistent storage with given BlockID.
	Upsert(blockID flow.Identifier, wasUploadCompleted bool) error

	// GetIDsByUploadStatus returns BlockIDs whose upload status matches with targetUploadStatus
	GetIDsByUploadStatus(targetUploadStatus bool) ([]flow.Identifier, error)

	// ByID returns the upload status of ComputationResult with given BlockID.
	ByID(blockID flow.Identifier) (bool, error)

	// Remove removes an instance of ComputationResult with given BlockID.
	Remove(blockID flow.Identifier) error
}
