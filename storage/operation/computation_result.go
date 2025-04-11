package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpsertComputationResultUploadStatus upserts given existing instance of ComputationResult in local BadgerDB.
func UpsertComputationResultUploadStatus(w storage.Writer, blockID flow.Identifier,
	wasUploadCompleted bool) error {
	return UpsertByKey(w, MakePrefix(codeComputationResults, blockID), wasUploadCompleted)
}

// RemoveComputationResultUploadStatus removes an instance of ComputationResult with given ID.
func RemoveComputationResultUploadStatus(
	w storage.Writer,
	blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeComputationResults, blockID))
}

// GetComputationResultUploadStatus returns stored ComputationResult instance with given ID.
func GetComputationResultUploadStatus(r storage.Reader, blockID flow.Identifier,
	wasUploadCompleted *bool) error {
	return RetrieveByKey(r, MakePrefix(codeComputationResults, blockID), wasUploadCompleted)
}

// GetBlockIDsByStatus returns all IDs of stored ComputationResult instances.
func GetBlockIDsByStatus(r storage.Reader, blockIDs *[]flow.Identifier,
	targetUploadStatus bool) error {
	return TraverseByPrefix(r, MakePrefix(codeComputationResults), func() (CheckFunc, CreateFunc, HandleFunc) {
		var currKey flow.Identifier
		check := func(key []byte) (bool, error) {
			currKey = flow.HashToID(key[1:])
			return true, nil
		}

		var wasUploadCompleted bool
		create := func() interface{} {
			return &wasUploadCompleted
		}

		handle := func() error {
			if blockIDs != nil && wasUploadCompleted == targetUploadStatus {
				*blockIDs = append(*blockIDs, currKey)
			}
			return nil
		}
		return check, create, handle
	}, storage.DefaultIteratorOptions())
}
