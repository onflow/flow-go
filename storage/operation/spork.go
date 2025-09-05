package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IndexSporkRootBlock indexes the spork root block ID for the present spork.
// A Node's state is strictly tied to a specific spork, and should never change during
// the lifetime of the node. This values is inserted exactly once when bootstrapping the
// state and should always be present for a properly bootstrapped node.
// CAUTION: OVERWRITES existing data (potential for data corruption).
// TODO: error if key already exists!
//
// No errors are expected during normal operation.
func IndexSporkRootBlock(w storage.Writer, blockID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeSporkRootBlockID), blockID)
}

// RetrieveSporkRootBlockID retrieves the spork root block's ID for the present spork, which this node is
// participating in. A Node's state is strictly tied to a specific spork, and should never change during
// the lifetime of the node.
// This values is inserted exactly once when bootstrapping the state and should always be present for a
// properly bootstrapped node.
// No errors are expected during normal operation.
func RetrieveSporkRootBlockID(r storage.Reader, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeSporkRootBlockID), blockID)
}
