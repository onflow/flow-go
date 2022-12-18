package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// IndexVersionBeaconByHeight stores version beacon by height it went into effect.
// See handleServiceEvents documentation for in-depth explanation of what height and why it's being used.
// This index allows us to answer questions such as what is the latest VersionBeacon service event up to
// block height 1000.
//
// why height?
// because height allows us scan through the index and then find the result for highest height.
// see LookupLastExecutionResultForServiceEventType
//
// No errors are expected during normal operation.
func IndexVersionBeaconByHeight(beacon *flow.VersionBeacon, blockHeight uint64) func(*badger.Txn) error {
	return upsert(makePrefix(codeVersionBeacon, blockHeight), beacon)
}

// LookupLastVersionBeaconByHeight finds the highest flow.VersionBeacon but no higher than maxHeight.
// Returns storage.ErrNotFound if version beacon exists at or below the given height.
func LookupLastVersionBeaconByHeight(maxHeight uint64, versionBeacon *flow.VersionBeacon, vbHeight *uint64) func(*badger.Txn) error {
	return findOneHighestButNoHigher(makePrefix(codeVersionBeacon), maxHeight, versionBeacon, vbHeight)
}
