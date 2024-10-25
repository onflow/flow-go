package environment

import (
	"github.com/coreos/go-semver/semver"

	"github.com/onflow/flow-go/fvm/storage/state"
)

// MinimumRequiredVersion returns the minimum required cadence version for the current environment
// in semver format.
type MinimumRequiredVersion interface {
	MinimumRequiredVersion() (string, error)
}

type minimumRequiredVersion struct {
	txnPreparer state.NestedTransactionPreparer
}

func NewMinimumRequiredVersion(
	txnPreparer state.NestedTransactionPreparer,
) MinimumRequiredVersion {
	return minimumRequiredVersion{
		txnPreparer: txnPreparer,
	}
}

func (c minimumRequiredVersion) MinimumRequiredVersion() (string, error) {
	executionParameters := c.txnPreparer.ExecutionParameters()

	// map the minimum required flow-go version to a minimum required cadence version
	cadenceVersion := mapToCadenceVersion(executionParameters.ExecutionVersion, minimumFvmToMinimumCadenceVersionMapping)

	return cadenceVersion.String(), nil
}

// mapToCadenceVersion finds the entry in the versionMapping with the flow version that is closest to,
// but not higher the given flowGoVersion than returns the cadence version for that entry.
// the versionMapping is assumed to be sorted by flowGoVersion.
func mapToCadenceVersion(flowGoVersion semver.Version, versionMapping []VersionMapEntry) semver.Version {
	// return 0.0.0 if there is no mapping for the version
	var closestEntry = VersionMapEntry{}

	// example setup: flowGoVersion = 2.1
	// versionMapping = [
	// 	{FlowGoVersion: 1.0, CadenceVersion: 1.1},
	// 	{FlowGoVersion: 2.0, CadenceVersion: 2.1},
	// 	{FlowGoVersion: 2.1, CadenceVersion: 2.2},
	// 	{FlowGoVersion: 3.0, CadenceVersion: 3.1},
	// 	{FlowGoVersion: 4.0, CadenceVersion: 4.1},
	// ]
	for _, entry := range versionMapping {
		// loop 1: 2.1 >= 1.0 closest entry is 0
		// loop 2: 2.1 >= 2.0 closest entry is 1
		// loop 3: 2.1 >= 2.1 closest entry is 2
		// loop 4: 2.1 < 3.0 we went too high: closest entry is 2 break
		if versionGreaterThanOrEqualTo(flowGoVersion, entry.FlowGoVersion) {
			closestEntry = entry
		} else {
			break
		}
	}

	// return the cadence version for the closest entry (2): 2.2
	return closestEntry.CadenceVersion
}

func versionGreaterThanOrEqualTo(version semver.Version, other semver.Version) bool {
	return version.Compare(other) >= 0
}

type VersionMapEntry struct {
	FlowGoVersion  semver.Version
	CadenceVersion semver.Version
}

// FVMToCadenceVersionMapping is a hardcoded mapping between FVM versions and Cadence versions.
// Entries are only needed for cadence versions where cadence intends to switch behaviour
// based on the version.
// This should be ordered in ascending order by FlowGoVersion.
var minimumFvmToMinimumCadenceVersionMapping = []VersionMapEntry{
	// Leaving this example in, so it's easier to understand
	//{
	//	FlowGoVersion:  *semver.New("0.37.0"),
	//	CadenceVersion: *semver.New("1.0.0"),
	//},
}

func SetFVMToCadenceVersionMappingForTestingOnly(mapping []VersionMapEntry) {
	minimumFvmToMinimumCadenceVersionMapping = mapping
}

var _ MinimumRequiredVersion = (*minimumRequiredVersion)(nil)
