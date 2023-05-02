package flow

import "golang.org/x/mod/semver"

// VersionBoundary represents a boundary between semver versions.
// BlockHeight is the first block height which must be run by the given Version (inclusive).
// Version is semver string.
type VersionBoundary struct {
	BlockHeight uint64
	Version     string
}

// VersionBeacon represents a service event which specifies required software versions
// for upcoming blocks.
//
// It contains VersionBoundaries field which is an ordered list of VersionBoundary
// (ordered by VersionBoundary.BlockHeight). While heights are strictly
// increasing, versions must be equal or greater, compared by semver semantics.
// It must contain at least one entry. The first entry is for a past block height.
// The rest of the entries are for all future block heights. Future version boundaries
// can be removed, in which case the event emitted will not contain the removed version
// boundaries.
// VersionBeacon is produced by NodeVersionBeacon smart contract.
//
// Sequence is event sequence number, which can be used to verify that no event has been
// skipped by the follower. Everytime the smart contract emits a new event, it increments
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
		if semver.Compare(v.Version, other.Version) != 0 {
			return false
		}
	}

	return true
}
