package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
)

// IndexVersionBeaconByHeight stores a sealed version beacon indexed by
// flow.SealedVersionBeacon.SealHeight.
//
// No errors are expected during normal operation.
func IndexVersionBeaconByHeight(
	beacon *flow.SealedVersionBeacon,
) func(pebble.Writer) error {
	return insert(makePrefix(codeVersionBeacon, beacon.SealHeight), beacon)
}

// LookupLastVersionBeaconByHeight finds the highest flow.VersionBeacon but no higher
// than maxHeight. Returns storage.ErrNotFound if no version beacon exists at or below
// the given height.
// TODO: fix it
func LookupLastVersionBeaconByHeight(
	maxHeight uint64,
	versionBeacon *flow.SealedVersionBeacon,
) func(pebble.Reader) error {
	return findHighestAtOrBelow(
		makePrefix(codeVersionBeacon),
		maxHeight,
		versionBeacon,
	)
}
