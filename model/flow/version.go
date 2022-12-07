package flow

// VersionControlRequirement describe a single version requirement - height and a corresponding version
// Height is the first height which must be run by a given Version
// Version is semver string. String type is used by golang.org/x/mod/semver, and it must be valid semver identifier,
// as defined by golang.org/x/mod/semver.isValid version
type VersionControlRequirement struct {
	Height  uint64
	Version string
}

// VersionTable is an ordered table of VersionControlRequirement. It informs about upcoming required versions by
// height. Table is ordered by height, ascending. While heights are strictly increasing, versions are equal or greater,
// compared by semver semantics
type VersionTable = []VersionControlRequirement

func (v *VersionTable) ServiceEvent() {

}
