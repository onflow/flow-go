package environment

import (
	"github.com/coreos/go-semver/semver"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

type ExecutionVersionProvider interface {
	ExecutionVersion() (semver.Version, error)
}

type GetVersionBeaconFunc func() (*flow.SealedVersionBeacon, error)

type VersionBeaconExecutionVersionProvider struct {
	getVersionBeacon GetVersionBeaconFunc
}

func NewVersionBeaconExecutionVersionProvider(getVersionBeacon GetVersionBeaconFunc) VersionBeaconExecutionVersionProvider {
	return VersionBeaconExecutionVersionProvider{
		getVersionBeacon: getVersionBeacon,
	}
}

func (v VersionBeaconExecutionVersionProvider) ExecutionVersion() (semver.Version, error) {
	vb, err := v.getVersionBeacon()
	if err != nil {
		return semver.Version{}, err
	}
	// Special case. If there are no version boundaries, then the execution version is 0.0.0.
	if vb == nil || len(vb.VersionBoundaries) == 0 {
		return semver.Version{}, nil
	}

	// by definition zero boundary is the last most recent past boundary
	boundary := vb.VersionBoundaries[0]
	sv, err := boundary.Semver()
	if err != nil {
		return semver.Version{}, err
	}

	return *sv, nil
}

type ZeroExecutionVersionProvider struct{}

func (v ZeroExecutionVersionProvider) ExecutionVersion() (semver.Version, error) {
	return semver.Version{}, nil
}

// MinimumCadenceRequiredVersion returns the minimum required cadence version for the current environment
// in semver format.
type MinimumCadenceRequiredVersion interface {
	MinimumRequiredVersion() (string, error)
}

type minimumCadenceRequiredVersion struct {
	executionVersionProvider ExecutionVersionProvider
}

func NewMinimumCadenceRequiredVersion(
	executionVersionProvider ExecutionVersionProvider,
) MinimumCadenceRequiredVersion {
	return minimumCadenceRequiredVersion{
		executionVersionProvider: executionVersionProvider,
	}
}

// MinimumRequiredVersion The returned cadence version can be used by cadence runtime for supporting feature flag.
// The feature flag in cadence allows ENs to produce consistent results even if running with
// different cadence versions at the same height, which is useful for rolling out cadence
// upgrade without all ENs restarting all together.
// For instance, we would like to grade cadence from v1 to v3, where v3 has a new cadence feature.
// We first make a cadence v2 that has feature flag only turned on when the MinimumRequiredVersion()
// method returns v2 or above.
// So cadence v2 with the feature flag turned off will produce the same result as v1 which doesn't have the feature.
// And cadence v2 with the feature flag turned on will also produce the same result as v3 which has the feature.
// The feature flag allows us to roll out cadence v2 to all ENs which was running v1.
// And we use the MinimumRequiredVersion to control when the feature flag should be switched from off to on.
// And the switching should happen at the same height for all ENs.
//
// The height-based switch over can be done by using VersionBeacon, however, the VersionBeacon only
// defines the flow-go version, not cadence version.
// So we first read the current minimum required flow-go version from the VersionBeacon control,
// and map it to the cadence version to be used by cadence to decide feature flag status.
//
// For instance, let’s say all ENs are running flow-go v0.37.0 with cadence v1.
// We first create a version mapping entry for flow-go v0.37.1 to cadence v2, and roll out v0.37.1 to all ENs.Z
// v0.37.1 ENs will produce the same result as v0.37.0 ENs, because the current version beacon still returns v0.37.0,
// which maps zero cadence version, and cadence will keep the feature flag off.
//
// After all ENs have upgraded to v0.37.1, we send out a version beacon to switch to v0.37.1 at a future height,
// let’s say height 1000.
// Then what happens is that:
//  1. ENs running v0.37.0 will crash after height 999, until upgrade to higher version
//  2. ENs running v0.37.1 will execute with cadence v2 with feature flag off up until height 999, and from height 1000,
//     the feature flag will be on, which means all v0.37.1 ENs will again produce consistent results for blocks above 1000.
//
// After height 1000 have been sealed, we can roll out v0.37.2 to all ENs with cadence v3, and it will produce the consistent
// result as v0.37.1.
func (c minimumCadenceRequiredVersion) MinimumRequiredVersion() (string, error) {
	executionVersion, err := c.executionVersionProvider.ExecutionVersion()
	if err != nil {
		return "", errors.NewExecutionVersionProviderFailure(err)
	}

	// map the minimum required flow-go version to a minimum required cadence version
	cadenceVersion := mapToCadenceVersion(executionVersion, minimumFvmToMinimumCadenceVersionMapping)

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

func setFVMToCadenceVersionMappingForTestingOnly(mapping FlowGoToCadenceVersionMapping) {
	minimumFvmToMinimumCadenceVersionMapping = mapping
}

var _ MinimumCadenceRequiredVersion = (*minimumCadenceRequiredVersion)(nil)
