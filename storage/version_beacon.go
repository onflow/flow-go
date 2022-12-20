package storage

import "github.com/onflow/flow-go/model/flow"

// VersionBeacons represents persistent storage for Version Beacons.
type VersionBeacons interface {

	// Highest finds the highest flow.VersionBeacon but no higher than maxHeight and the corresponding height it has been finalized.
	// Returns storage.ErrNotFound if no version beacon exists at or below the given height.
	Highest(maxHeight uint64) (*flow.VersionBeacon, uint64, error)
}
