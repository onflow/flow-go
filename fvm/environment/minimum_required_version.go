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

func mapToCadenceVersion(flowGoVersion semver.Version, versionMapping FlowGoToCadenceVersionMapping) semver.Version {
	if versionGreaterThanOrEqualTo(flowGoVersion, versionMapping.FlowGoVersion) {
		return versionMapping.CadenceVersion
	} else {
		return semver.Version{}
	}
}

func versionGreaterThanOrEqualTo(version semver.Version, other semver.Version) bool {
	return version.Compare(other) >= 0
}

type FlowGoToCadenceVersionMapping struct {
	FlowGoVersion  semver.Version
	CadenceVersion semver.Version
}

// This could also be a map, but ist not needed because we only expect one entry at a give time
// we won't be fixing 2 separate issues at 2 separate version with one deploy.
var minimumFvmToMinimumCadenceVersionMapping = FlowGoToCadenceVersionMapping{
	// Leaving this example in, so it's easier to understand
	//
	//	FlowGoVersion:  *semver.New("0.37.0"),
	//	CadenceVersion: *semver.New("1.0.0"),
	//
}

func SetFVMToCadenceVersionMappingForTestingOnly(mapping FlowGoToCadenceVersionMapping) {
	minimumFvmToMinimumCadenceVersionMapping = mapping
}

var _ MinimumRequiredVersion = (*minimumRequiredVersion)(nil)
