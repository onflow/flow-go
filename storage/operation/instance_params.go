package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// EncodableInstanceParams is the consolidated, serializable form of protocol instance parameters
// that are constant throughout the lifetime of a node.
//
// Fields:
//   - FinalizedRootID: ID of the finalized root block used to bootstrap
//   - SealedRootID: ID of the sealed root block
type EncodableInstanceParams struct {
	FinalizedRootID flow.Identifier
	SealedRootID    flow.Identifier
}

// InsertInstanceParams stores the consolidated instance params under a single key.
// If the key already exists, the value will be overwritten.
// Error returns:
//   - generic error in case of unexpected failure from the database layer or
//     encoding failure.
func InsertInstanceParams(w storage.Writer, params EncodableInstanceParams) error {
	return UpsertByKey(w, MakePrefix(codeInstanceParams), params)
}

// RetrieveInstanceParams reads the consolidated instance params from storage.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value
func RetrieveInstanceParams(r storage.Reader, params *EncodableInstanceParams) error {
	return RetrieveByKey(r, MakePrefix(codeInstanceParams), params)
}
