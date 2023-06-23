package storage

import "github.com/onflow/flow-go/model/flow"

// VersionBeacons represents persistent storage for Version Beacons.
type VersionBeacons interface {

	// Highest finds the highest flow.SealedVersionBeacon but no higher than
	// belowOrEqualTo
	// Returns nil if no version beacon has been sealed below or equal to the
	// input height.
	Highest(belowOrEqualTo uint64) (*flow.SealedVersionBeacon, error)
}
