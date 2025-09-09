package operation

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// EncodableInstanceParams is the consolidated, serializable form of protocol instance parameters
// that are constant throughout the lifetime of a node.
// These values are inserted exactly once when bootstrapping the state and should always be present for a
// properly bootstrapped node.
type EncodableInstanceParams struct {
	// FinalizedRootID is the ID of the finalized root block.
	FinalizedRootID flow.Identifier
	// SealedRootID is the ID of the sealed root block.
	SealedRootID flow.Identifier
	// SporkRootBlockID is the root block's ID for the present spork, which this node is
	// participating in. A Node's state is strictly tied to a specific spork, and should never change during
	// the lifetime of the node.
	SporkRootBlockID flow.Identifier
}

// InsertInstanceParams stores the consolidated instance params under a single key.
// If the key already exists, the value will be overwritten.
// Error returns:
//   - [storage.ErrAlreadyExists] if instance params is already stored to prevent potential for data corruption
//   - generic error in case of unexpected failure from the database layer or
//     encoding failure.
func InsertInstanceParams(rw storage.ReaderBatchWriter, params EncodableInstanceParams) error {
	key := MakePrefix(codeInstanceParams)
	exist, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return err
	}
	if exist {
		return fmt.Errorf("instance params is already stored: %w", storage.ErrAlreadyExists)
	}
	return UpsertByKey(rw.Writer(), key, params)
}

// RetrieveInstanceParams reads the consolidated instance params from storage.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value
func RetrieveInstanceParams(r storage.Reader, params *EncodableInstanceParams) error {
	return RetrieveByKey(r, MakePrefix(codeInstanceParams), params)
}
