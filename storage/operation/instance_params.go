package operation

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// EncodableInstanceParams is the consolidated, serializable form of protocol instance
// parameters that are constant throughout the lifetime of a node.
type EncodableInstanceParams struct {
	// FinalizedRootID is the ID of the finalized root block.
	FinalizedRootID flow.Identifier
	// SealedRootID is the ID of the sealed root block.
	SealedRootID flow.Identifier
	// SporkRootBlockID is the root block's ID for the present spork this node participates in.
	SporkRootBlockID flow.Identifier
}

// InsertInstanceParams stores the consolidated instance params under a single key.
//
// CAUTION:
//   - This function is intended to be called exactly once during bootstrapping.
//   - Overwrites are prevented by an explicit existence check; if data is already present, error is returned.
//
// Expected errors during normal operations:
//   - [storage.ErrAlreadyExists] if instance params have already been stored.
//   - Generic error for unexpected database or encoding failures.
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

// RetrieveInstanceParams retrieves the consolidated instance params from storage.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if the key does not exist (not bootstrapped).
//   - Generic error for unexpected database or decoding failures.
func RetrieveInstanceParams(r storage.Reader, params *EncodableInstanceParams) error {
	return RetrieveByKey(r, MakePrefix(codeInstanceParams), params)
}
