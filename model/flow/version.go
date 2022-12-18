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

// VersionBeacon represents a service event which specified required software versions for upcoming blocks.
// It contains RequiredVersions field which is an ordered list of VersionControlRequirement.
// RequiredVersions is an ordered table of VersionControlRequirement. It informs about upcoming required versions by
// height. Table is ordered by height, ascending. While heights are strictly increasing, versions are equal or greater,
// compared by semver semantics. First entry, if present, must indicate a current version - that is its height must be
// at or below a block is has been emitted in.
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
