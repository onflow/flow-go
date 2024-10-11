package environment

import (
	"context"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type CurrentVersionBoundary interface {
	CurrentVersionBoundary() (flow.VersionBoundary, error)
}

type ParseRestrictedCurrentVersionBoundary struct {
	txnState state.NestedTransactionPreparer
	impl     CurrentVersionBoundary
}

func NewParseRestrictedCurrentVersionBoundary(
	txnState state.NestedTransactionPreparer,
	impl CurrentVersionBoundary,
) CurrentVersionBoundary {
	return ParseRestrictedCurrentVersionBoundary{
		txnState: txnState,
		impl:     impl,
	}
}

func (p ParseRestrictedCurrentVersionBoundary) CurrentVersionBoundary() (flow.VersionBoundary, error) {
	return parseRestrict1Ret(
		p.txnState,
		trace.FVMEnvRandom,
		p.impl.CurrentVersionBoundary)
}

type currentVersionBoundary struct {
	tracer    tracing.TracerSpan
	meter     Meter
	txnState  storage.TransactionPreparer
	envParams EnvironmentParams
}

func NewCurrentVersionBoundary(
	tracer tracing.TracerSpan,
	meter Meter,
	txnState storage.TransactionPreparer,
	envParams EnvironmentParams,
) CurrentVersionBoundary {
	return currentVersionBoundary{
		tracer:    tracer,
		meter:     meter,
		txnState:  txnState,
		envParams: envParams,
	}
}

func (c currentVersionBoundary) CurrentVersionBoundary() (flow.VersionBoundary, error) {
	tracerSpan := c.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvGetCurrentVersionBoundary)
	defer tracerSpan.End()

	err := c.meter.MeterComputation(ComputationKindGetCurrentVersionBoundary, 1)
	if err != nil {
		// TODO: return a better error
		return flow.VersionBoundary{}, err
	}

	value, _, err := c.txnState.GetCurrentVersionBoundary(
		c.txnState,
		NewCurrentVersionBoundaryComputer(
			tracerSpan,
			c.envParams,
			c.txnState,
		),
	)
	if err != nil {
		// TODO: return a better error
		return flow.VersionBoundary{}, err
	}

	return value, nil
}

var _ CurrentVersionBoundary = (*currentVersionBoundary)(nil)

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
		// TODO: return a better error
		return flow.VersionBoundary{}, err
	}

	boundary, err := convert.VersionBoundary(value)

	if err != nil {
		// TODO: return a better error
		return flow.VersionBoundary{}, err
	}

	return boundary, nil
}
