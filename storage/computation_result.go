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
	// Upsert inserts or updates ComputationResult into persistent storage with given ID.
	Upsert(computationResultID flow.Identifier, wasUploadCompleted bool) error

	// GetAllIDs returns all IDs of stored ComputationResult upload status.
	GetAllIDs() ([]flow.Identifier, error)

	// ByID returns the upload status of ComputationResult with given ID.
	ByID(computationResultID flow.Identifier) (bool, error)

	// Remove removes an instance of ComputationResult with given ID.
	Remove(computationResultID flow.Identifier) error
}
