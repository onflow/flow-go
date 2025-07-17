package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// UpsertComputationResult upserts given existing instance of ComputationResult in local BadgerDB.
func UpsertComputationResultUploadStatus(w storage.Writer, blockID flow.Identifier,
	wasUploadCompleted bool) error {
	return UpsertByKey(w, MakePrefix(codeComputationResults, blockID), wasUploadCompleted)
}

// RemoveComputationResult removes an instance of ComputationResult with given ID.
func RemoveComputationResultUploadStatus(
	w storage.Writer,
	blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeComputationResults, blockID))
}

// GetComputationResult returns stored ComputationResult instance with given ID.
func GetComputationResultUploadStatus(r storage.Reader, blockID flow.Identifier,
	wasUploadCompleted *bool) error {
	return RetrieveByKey(r, MakePrefix(codeComputationResults, blockID), wasUploadCompleted)
}

// GetBlockIDsByStatus returns all IDs of stored ComputationResult instances.
func GetBlockIDsByStatus(r storage.Reader, blockIDs *[]flow.Identifier,
	targetUploadStatus bool) error {
	iterationFunc := func(keyCopy []byte, getValue func(destVal any) error) (bail bool, err error) {
		var wasUploadCompleted bool
		err = getValue(&wasUploadCompleted)
		if err != nil {
			return true, err
		}

		if wasUploadCompleted == targetUploadStatus {
			*blockIDs = append(*blockIDs, flow.HashToID(keyCopy[1:]))
		}
		return false, nil
	}

	return TraverseByPrefix(r, MakePrefix(codeComputationResults), iterationFunc, storage.DefaultIteratorOptions())
}
