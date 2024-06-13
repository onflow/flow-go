package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
)

// InsertComputationResult addes given instance of ComputationResult into local BadgerDB.
func InsertComputationResultUploadStatus(blockID flow.Identifier,
	wasUploadCompleted bool) func(pebble.Writer) error {
	return insert(makePrefix(codeComputationResults, blockID), wasUploadCompleted)
}

// UpdateComputationResult updates given existing instance of ComputationResult in local BadgerDB.
func UpdateComputationResultUploadStatus(blockID flow.Identifier,
	wasUploadCompleted bool) func(PebbleReaderWriter) error {
	return update(makePrefix(codeComputationResults, blockID), wasUploadCompleted)
}

// UpsertComputationResult upserts given existing instance of ComputationResult in local BadgerDB.
func UpsertComputationResultUploadStatus(blockID flow.Identifier,
	wasUploadCompleted bool) func(pebble.Writer) error {
	return insert(makePrefix(codeComputationResults, blockID), wasUploadCompleted)
}

// RemoveComputationResult removes an instance of ComputationResult with given ID.
func RemoveComputationResultUploadStatus(
	blockID flow.Identifier) func(pebble.Writer) error {
	return remove(makePrefix(codeComputationResults, blockID))
}

// GetComputationResult returns stored ComputationResult instance with given ID.
func GetComputationResultUploadStatus(blockID flow.Identifier,
	wasUploadCompleted *bool) func(pebble.Reader) error {
	return retrieve(makePrefix(codeComputationResults, blockID), wasUploadCompleted)
}

// GetBlockIDsByStatus returns all IDs of stored ComputationResult instances.
func GetBlockIDsByStatus(blockIDs *[]flow.Identifier,
	targetUploadStatus bool) func(pebble.Reader) error {
	return traverse(makePrefix(codeComputationResults), func() (checkFunc, createFunc, handleFunc) {
		var currKey flow.Identifier
		check := func(key []byte) bool {
			currKey = flow.HashToID(key[1:])
			return true
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
	})
}
