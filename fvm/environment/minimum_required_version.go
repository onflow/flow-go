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

	cadenceVersion := mapToCadenceVersion(executionParameters.ExecutionVersion)

	return cadenceVersion.String(), nil
}

func mapToCadenceVersion(version semver.Version) semver.Version {
	// return 0.0.0 if there is no mapping for the version
	var cadenceVersion = semver.Version{}

	greaterThanOrEqualTo := func(version semver.Version, versionToCompare semver.Version) bool {
		return version.Compare(versionToCompare) >= 0
	}

	for _, entry := range fvmToCadenceVersionMapping {
		if greaterThanOrEqualTo(version, entry.FlowGoVersion) {
			cadenceVersion = entry.CadenceVersion
		} else {
			break
		}
	}

	return cadenceVersion
}

type VersionMapEntry struct {
	FlowGoVersion  semver.Version
	CadenceVersion semver.Version
}

// FVMToCadenceVersionMapping is a hardcoded mapping between FVM versions and Cadence versions.
// Entries are only needed for cadence versions where cadence intends to switch behaviour
// based on the version.
// This should be ordered.

var fvmToCadenceVersionMapping = []VersionMapEntry{
	// Leaving this example in, so it's easier to understand
	//{
	//	FlowGoVersion:  *semver.New("0.37.0"),
	//	CadenceVersion: *semver.New("1.0.0"),
	//},
}

func SetFVMToCadenceVersionMappingForTestingOnly(mapping []VersionMapEntry) {
	fvmToCadenceVersionMapping = mapping
}

var _ MinimumRequiredVersion = (*minimumRequiredVersion)(nil)
