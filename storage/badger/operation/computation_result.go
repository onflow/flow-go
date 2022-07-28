package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertComputationResult addes given instance of ComputationResult into local BadgerDB.
func InsertComputationResultUploadStatus(computationResultID flow.Identifier,
	wasUploadCompleted bool) func(*badger.Txn) error {
	return insert(makePrefix(codeComputationResults, computationResultID), wasUploadCompleted)
}

// UpdateComputationResult updates given existing instance of ComputationResult in local BadgerDB.
func UpdateComputationResultUploadStatus(computationResultID flow.Identifier,
	wasUploadCompleted bool) func(*badger.Txn) error {
	return update(makePrefix(codeComputationResults, computationResultID), wasUploadCompleted)
}

// RemoveComputationResult removes an instance of ComputationResult with given ID.
func RemoveComputationResultUploadStatus(
	computationResultID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeComputationResults, computationResultID))
}

// GetComputationResult returns stored ComputationResult instance with given ID.
func GetComputationResultUploadStatus(computationResultID flow.Identifier,
	wasUploadCompleted *bool) func(*badger.Txn) error {
	return retrieve(makePrefix(codeComputationResults, computationResultID), wasUploadCompleted)
}

// GetAllComputationResultIDs returns all IDs of stored ComputationResult instances.
func GetAllComputationResultIDs(computationResultIDs *[]flow.Identifier) func(*badger.Txn) error {
	return traverse(makePrefix(codeComputationResults), func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			*computationResultIDs = append(*computationResultIDs, flow.HashToID(key[1:]))
			return true
		}

		var wasUploadCompleted bool
		create := func() interface{} {
			return &wasUploadCompleted
		}

		handle := func() error {
			return nil
		}
		return check, create, handle
	})
}
