package flow

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
)

// VersionBoundary represents a boundary between semver versions.
// BlockHeight is the first block height that must be run by the given Version (inclusive).
// Version is a semver string.
type VersionBoundary struct {
	BlockHeight uint64
	Version     string
}

func (v VersionBoundary) Semver() (*semver.Version, error) {
	return semver.NewVersion(v.Version)
}

// VersionBeacon represents a service event specifying the required software versions
// for upcoming blocks.
//
// It contains a VersionBoundaries field, which is an ordered list of VersionBoundary
// (sorted by VersionBoundary.BlockHeight). While heights are strictly
// increasing, versions must be equal or greater when compared using semver semantics.
// It must contain at least one entry. The first entry is for a past block height.
// The remaining entries are for all future block heights. Future version boundaries
// can be removed, in which case the emitted event will not contain the removed version
// boundaries.
// VersionBeacon is produced by the NodeVersionBeacon smart contract.
//
// Sequence is the event sequence number, which can be used to verify that no event has been
// skipped by the follower. Every time the smart contract emits a new event, it increments
// the sequence number by one.
type VersionBeacon struct {
	VersionBoundaries []VersionBoundary
	Sequence          uint64
}

// SealedVersionBeacon is a VersionBeacon with a SealHeight field.
// Version beacons are effective only after they are sealed.
type SealedVersionBeacon struct {
	*VersionBeacon
	SealHeight uint64
}

func (v *VersionBeacon) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventVersionBeacon,
		Event: v,
	}
}

// EqualTo returns true if two VersionBeacons are equal.
// If any of the VersionBeacons has a malformed version, it will return false.
func (v *VersionBeacon) EqualTo(other *VersionBeacon) bool {

	if v.Sequence != other.Sequence {
		return false
	}

	if len(v.VersionBoundaries) != len(other.VersionBoundaries) {
		return false
	}

	for i, v := range v.VersionBoundaries {
		other := other.VersionBoundaries[i]

		if v.BlockHeight != other.BlockHeight {
			return false
		}

		v1, err := v.Semver()
		if err != nil {
			return false
		}
		v2, err := other.Semver()
		if err != nil {
			return false
		}
		if !v1.Equal(*v2) {
			return false
		}
	}

	return true
}

// Validate validates the internal structure of a flow.VersionBeacon.
// An error with an appropriate message is returned
// if any validation fails.
func (v *VersionBeacon) Validate() error {
	eventError := func(format string, args ...interface{}) error {
		args = append([]interface{}{v.Sequence}, args...)
		return fmt.Errorf(
			"version beacon (sequence=%d) error: "+format,
			args...,
		)
	}

	if len(v.VersionBoundaries) == 0 {
		return eventError("required version boundaries empty")
	}

	var previousHeight uint64
	var previousVersion *semver.Version
	for i, boundary := range v.VersionBoundaries {
		version, err := boundary.Semver()
		if err != nil {
			return eventError(
				"invalid semver %s for version boundary (height=%d) (index=%d): %w",
				boundary.Version,
				boundary.BlockHeight,
				i,
				err,
			)
		}

		if i != 0 && previousHeight >= boundary.BlockHeight {
			return eventError(
				"higher requirement (index=%d) height %d "+
					"at or below previous height (index=%d) %d",
				i,
				boundary.BlockHeight,
				i-1,
				previousHeight,
			)
		}

		if i != 0 && version.LessThan(*previousVersion) {
			return eventError(
				"higher requirement (index=%d) semver %s "+
					"lower than previous (index=%d) %s",
				i,
				version,
				i-1,
				previousVersion,
			)
		}

		previousVersion = version
		previousHeight = boundary.BlockHeight
	}

	return nil
}
