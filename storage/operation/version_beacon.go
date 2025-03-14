package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IndexVersionBeaconByHeight stores a sealed version beacon indexed by
// flow.SealedVersionBeacon.SealHeight.
//
// No errors are expected during normal operation.
func IndexVersionBeaconByHeight(
	w storage.Writer,
	beacon *flow.SealedVersionBeacon,
) error {
	return UpsertByKey(w, MakePrefix(codeVersionBeacon, beacon.SealHeight), beacon)
}

// LookupLastVersionBeaconByHeight finds the highest flow.VersionBeacon but no higher
// than maxHeight.

// Returns storage.ErrNotFound if no version beacon exists at or below
// the given height.
func LookupLastVersionBeaconByHeight(
	r storage.Reader,
	maxHeight uint64,
	versionBeacon *flow.SealedVersionBeacon,
) error {
	return FindHighestAtOrBelowByPrefix(
		r,
		MakePrefix(codeVersionBeacon),
		maxHeight,
		versionBeacon,
	)
}
