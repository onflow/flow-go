package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertSporkID inserts the spork ID for the present spork. A single database
// and protocol state instance spans at most one spork, so this is inserted
// exactly once, when bootstrapping the state.
func InsertSporkID(sporkID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeSporkID), sporkID)
}

// RetrieveSporkID retrieves the spork ID for the present spork.
func RetrieveSporkID(sporkID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSporkID), sporkID)
}

// InsertSporkRootBlockHeight inserts the spork root block height for the present spork.
// A single database and protocol state instance spans at most one spork, so this is inserted
// exactly once, when bootstrapping the state.
func InsertSporkRootBlockHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeSporkRootBlockHeight), height)
}

// RetrieveSporkRootBlockHeight retrieves the spork root block height for the present spork.
func RetrieveSporkRootBlockHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSporkRootBlockHeight), height)
}

// InsertProtocolVersion inserts the protocol version for the present spork.
// A single database and protocol state instance spans at most one spork, and
// a spork has exactly one protocol version for its duration, so this is
// inserted exactly once, when bootstrapping the state.
func InsertProtocolVersion(version uint) func(*badger.Txn) error {
	return insert(makePrefix(codeProtocolVersion), version)
}

// RetrieveProtocolVersion retrieves the protocol version for the present spork.
func RetrieveProtocolVersion(version *uint) func(*badger.Txn) error {
	return retrieve(makePrefix(codeProtocolVersion), version)
}

// InsertEpochCommitSafetyThreshold inserts the epoch commit safety threshold
// for the present spork.
// A single database and protocol state instance spans at most one spork, and
// a spork has exactly one protocol version for its duration, so this is
// inserted exactly once, when bootstrapping the state.
func InsertEpochCommitSafetyThreshold(threshold uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeEpochCommitSafetyThreshold), threshold)
}

// RetrieveEpochCommitSafetyThreshold retrieves the epoch commit safety threshold
// for the present spork.
func RetrieveEpochCommitSafetyThreshold(threshold *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeEpochCommitSafetyThreshold), threshold)
}
