package storage

import "github.com/onflow/flow-go/model/flow"

// VersionBeacons represents persistent storage for Version Beacons.
type VersionBeacons interface {

	// Highest finds the highest flow.SealedVersionBeacon but no higher than
	// belowOrEqualTo
	// Returns nil.
	Highest(belowOrEqualTo uint64) (*flow.SealedVersionBeacon, error)
}
