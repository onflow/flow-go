package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertSporkID inserts the spork ID for the present spork.
// This values is inserted exactly once when bootstrapping the state and
// should always be present for a properly bootstrapped node.
// CAUTION: OVERWRITES existing data (potential for data corruption).
//
// No errors are expected during normal operation.
func InsertSporkID(w storage.Writer, sporkID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeSporkID), sporkID)
}

// RetrieveSporkID retrieves the spork ID for the present spork.
// This values should always be present for a properly bootstrapped node.
// No errors are expected during normal operation.
func RetrieveSporkID(r storage.Reader, sporkID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeSporkID), sporkID)
}

// InsertSporkRootBlockHeight inserts the spork root block height for the present spork.
// This values is inserted exactly once when bootstrapping the state and
// should always be present for a properly bootstrapped node.
// CAUTION: OVERWRITES existing data (potential for data corruption).
//
// No errors are expected during normal operation.
func InsertSporkRootBlockHeight(w storage.Writer, height uint64) error {
	return UpsertByKey(w, MakePrefix(codeSporkRootBlockHeight), height)
}

// RetrieveSporkRootBlockHeight retrieves the spork root block height for the present spork.
// This values should always be present for a properly bootstrapped node.
// No errors are expected during normal operation.
func RetrieveSporkRootBlockHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeSporkRootBlockHeight), height)
}
