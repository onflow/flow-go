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

// VersionBeacon represents a service event which specifies required software versions for upcoming blocks.
// It contains RequiredVersions field which is an ordered list of VersionControlRequirement.
// RequiredVersions is an ordered table of VersionControlRequirement. It informs about upcoming required versions by
// height. Table is ordered by height, ascending. While heights are strictly increasing, versions are equal or greater,
// compared by semver semantics. It must contain at least one entry. If no version's requirement are present then
// no VersionBeacon should be emitted at all.
// VersionBeacon is produced by NodeVersionBeacon smart contract, and it can enforce additional restrictions, however
// those must be compatible with requirements described above.
// One important limitation enforced by a contract is restricting introduction of new versions less than N amount of blocks in the future.
// This prevents the version ambiguity as we don't expect forks to run for that amount of blocks.
// The version to process blocks at any given height must be unambiguous. This threshold has been chosen, so it's longer than any
// possible fork we anticipate, its current value if part of a contract.
// It is important to note that length of a fork is theoretically unlimited and can extend any assumed threshold, however unlikely.
// To account for this situation: when the VersionBeacon takes effect (block sealing results containing event is finalized)
// node should check if it's possible to apply a new versions (for example, EN can check if it has not executed blocks higher than new
// requirement). If it's not possible, node should crash with appropriate message.
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
