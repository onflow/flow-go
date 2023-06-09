package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// IndexVersionBeaconByHeight stores a sealed version beacon indexed by
// flow.SealedVersionBeacon.SealHeight.
//
// No errors are expected during normal operation.
func IndexVersionBeaconByHeight(
	beacon *flow.SealedVersionBeacon,
) func(*badger.Txn) error {
	return upsert(makePrefix(codeVersionBeacon, beacon.SealHeight), beacon)
}

// LookupLastVersionBeaconByHeight finds the highest flow.VersionBeacon but no higher
// than maxHeight. Returns storage.ErrNotFound if no version beacon exists at or below
// the given height.
func LookupLastVersionBeaconByHeight(
	maxHeight uint64,
	versionBeacon *flow.SealedVersionBeacon,
) func(*badger.Txn) error {
	return findHighestAtOrBelow(
		makePrefix(codeVersionBeacon),
		maxHeight,
		versionBeacon,
	)
}
