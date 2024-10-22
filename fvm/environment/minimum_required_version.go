package environment

import (
	"context"
	"fmt"

	"github.com/coreos/go-semver/semver"

	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// MinimumRequiredVersion returns the minimum required cadence version for the current environment
// in semver format.
type MinimumRequiredVersion interface {
	MinimumRequiredVersion() (string, error)
}

type ParseRestrictedMinimumRequiredVersion struct {
	txnState state.NestedTransactionPreparer
	impl     MinimumRequiredVersion
}

func NewParseRestrictedMinimumRequiredVersion(
	txnState state.NestedTransactionPreparer,
	impl MinimumRequiredVersion,
) MinimumRequiredVersion {
	return ParseRestrictedMinimumRequiredVersion{
		txnState: txnState,
		impl:     impl,
	}
}

func (p ParseRestrictedMinimumRequiredVersion) MinimumRequiredVersion() (string, error) {
	return parseRestrict1Ret(
		p.txnState,
		trace.FVMEnvRandom,
		p.impl.MinimumRequiredVersion)
}

type minimumRequiredVersion struct {
	tracer tracing.TracerSpan
	meter  Meter

	txnState  storage.TransactionPreparer
	envParams EnvironmentParams
}

func NewMinimumRequiredVersion(
	tracer tracing.TracerSpan,
	meter Meter,
	txnState storage.TransactionPreparer,
	envParams EnvironmentParams,
) MinimumRequiredVersion {
	return minimumRequiredVersion{
		tracer: tracer,
		meter:  meter,

		txnState:  txnState,
		envParams: envParams,
	}
}

func (c minimumRequiredVersion) MinimumRequiredVersion() (string, error) {
	tracerSpan := c.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvMinimumRequiredVersion)
	defer tracerSpan.End()

	err := c.meter.MeterComputation(ComputationKindMinimumRequiredVersion, 1)
	if err != nil {
		return "", fmt.Errorf("get MinimumRequiredVersion failed: %w", err)
	}

	value, err := c.txnState.GetCurrentVersionBoundary(
		c.txnState,
		NewCurrentVersionBoundaryComputer(
			tracerSpan,
			c.envParams,
			c.txnState,
		),
	)
	if err != nil {
		return "", fmt.Errorf("get MinimumRequiredVersion failed: %w", err)
	}

	version, err := c.mapToCadenceVersion(value.Version)

	if err != nil {
		return "", fmt.Errorf("get MinimumRequiredVersion failed: %w", err)
	}

	return version, nil
}

func (c minimumRequiredVersion) mapToCadenceVersion(version string) (string, error) {
	semVer, err := semver.NewVersion(version)
	if err != nil {
		// TODO: return a better error
		return "", fmt.Errorf("failed to map FVM version to cadence version: %w", err)
	}

	// return 0.0.0 if there is no mapping for the version
	var cadenceVersion = semver.Version{}

	greaterThanOrEqualTo := func(version semver.Version, versionToCompare semver.Version) bool {
		return version.Compare(versionToCompare) >= 0
	}

	for _, entry := range fvmToCadenceVersionMapping {
		if greaterThanOrEqualTo(*semVer, entry.FlowGoVersion) {
			cadenceVersion = entry.CadenceVersion
		} else {
			break
		}
	}

	return cadenceVersion.String(), nil
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
	{
		FlowGoVersion:  *semver.New("0.37.0"),
		CadenceVersion: *semver.New("1.0.0"),
	},
}

func SetFVMToCadenceVersionMappingForTestingOnly(mapping []VersionMapEntry) {
	fvmToCadenceVersionMapping = mapping
}

var _ MinimumRequiredVersion = (*minimumRequiredVersion)(nil)

type CurrentVersionBoundaryComputer struct {
	tracerSpan tracing.TracerSpan
	envParams  EnvironmentParams
	txnState   storage.TransactionPreparer
}

func NewCurrentVersionBoundaryComputer(
	tracerSpan tracing.TracerSpan,
	envParams EnvironmentParams,
	txnState storage.TransactionPreparer,
) CurrentVersionBoundaryComputer {
	return CurrentVersionBoundaryComputer{
		tracerSpan: tracerSpan,
		envParams:  envParams,
		txnState:   txnState,
	}
}

func (computer CurrentVersionBoundaryComputer) Compute(
	_ state.NestedTransactionPreparer,
	_ struct{},
) (
	flow.VersionBoundary,
	error,
) {
	env := NewScriptEnv(
		context.Background(),
		computer.tracerSpan,
		computer.envParams,
		computer.txnState)

	value, err := env.GetCurrentVersionBoundary()

	if err != nil {
		return flow.VersionBoundary{}, fmt.Errorf("could not get current version boundary: %w", err)
	}

	boundary, err := convert.VersionBoundary(value)

	if err != nil {
		return flow.VersionBoundary{}, fmt.Errorf("could not parse current version boundary: %w", err)
	}

	return boundary, nil
}
