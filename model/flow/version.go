package flow

import "golang.org/x/mod/semver"

// VersionControlRequirement describe a single version requirement - height and a corresponding version
// Height is the first height which must be run by a given Version
// Version is semver string. String type is used by golang.org/x/mod/semver, and it must be valid semver identifier,
// as defined by golang.org/x/mod/semver.isValid version
type VersionControlRequirement struct {
	Height  uint64
	Version string
}

// VersionTable represents a service event which specified required software versions for upcoming blocks.
// It contains RequiredVersions field which is an ordered list of VersionControlRequirement. It informs about upcoming required versions by
// VersionTable is an ordered table of VersionControlRequirement. It informs about upcoming required versions by
// height. Table is ordered by height, ascending. While heights are strictly increasing, versions are equal or greater,
// compared by semver semantics
// Sequence is event sequence number, which can be used to verify that no event has been skipped by the follower
type VersionTable struct {
	RequiredVersions []VersionControlRequirement
	Sequence         uint64
}

func (v *VersionTable) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventVersionControl,
		Event: v,
	}
}

func (v *VersionTable) EqualTo(other *VersionTable) bool {

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
