package flow

import "golang.org/x/mod/semver"

// VersionControlRequirement describe a single version requirement - height and a corresponding version
// Height is the first height which must be run by a given Version.
// Version is semver string. String type is used by golang.org/x/mod/semver, and it must be valid semver identifier,
// as defined by golang.org/x/mod/semver.isValid version
type VersionControlRequirement struct {
	Height  uint64
	Version string
}

// VersionThreshold is a minimum number of blocks from the current one when the new version can be introduced.
// This prevents the version ambiguity as we don't expect forks to run for that amount of blocks.
// The version to process blocks at any given height must be unambiguous. This threshold has been chosen so
// it's longer than any possible fork we anticipate.
// Note - this is only a distance between blocks the VersionBeacon is marked as in-effect and lowest version change,
// not the distance between versions. For example, while in-effect at block 1000, VB can introduce new version at block 2000
// 2100, 2200 etc. But not if in-effect height is at 2000 and next new version is set at 2200.
// IMPORTANT - This value must match its counterpart in NodeVersionBeacon contract.
const VersionThreshold = 1000

// VersionBeacon represents a service event which specifies required software versions for upcoming blocks.
// It contains RequiredVersions field which is an ordered list of VersionControlRequirement.
// RequiredVersions is an ordered table of VersionControlRequirement. It informs about upcoming required versions by
// height. Table is ordered by height, ascending. While heights are strictly increasing, versions are equal or greater,
// compared by semver semantics. It must contain at least one entry. If no version's requirement are present then
// no VersionBeacon should be emitted at all.
// There are two cases for RequiredVersions:
// Single-entry - can only happen when no previous version beacon exists:
//   Its height must be greater than current block by at least VersionThreshold distance.
// Multi-entry:
//   - no previous versions were ever set - first entry must be at least VersionThreshold blocks from the current block's height
//   - there were previous version set - first entry's height must be below the current block's height and it's version must be
//     the current running versions. Next one, so first upcoming version must be at least VersionThreshold blocks in the future.
//
// In other words, current active version, if ever set,  must be the first entry in the table. Then, any upcoming change must be at least
// VersionThreshold blocks in the future regardless.

// testy maksio kurwa

// Sequence is event sequence number, which can be used to verify that no event has been skipped by the follower
type VersionBeacon struct {
	RequiredVersions []VersionControlRequirement
	Sequence         uint64
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

	if len(v.RequiredVersions) != len(other.RequiredVersions) {
		return false
	}

	for i, v := range v.RequiredVersions {
		other := other.RequiredVersions[i]

		if v.Height != other.Height {
			return false
		}
		if semver.Compare(v.Version, other.Version) != 0 {
			return false
		}
	}

	return true
}
