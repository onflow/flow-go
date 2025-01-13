package derived

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/meter"
)

type MeterParamOverrides struct {
	ComputationWeights meter.ExecutionEffortWeights // nil indicates no override
	MemoryWeights      meter.ExecutionMemoryWeights // nil indicates no override
	MemoryLimit        *uint64                      // nil indicates no override
}

// StateExecutionParameters are parameters needed for execution defined in the execution state.
type StateExecutionParameters struct {
	MeterParamOverrides
}

type ProgramInvalidator TableInvalidator[
	common.AddressLocation,
	*Program,
]

type ExecutionParametersInvalidator TableInvalidator[
	struct{},
	StateExecutionParameters,
]

type TransactionInvalidator interface {
	ProgramInvalidator() ProgramInvalidator
	ExecutionParametersInvalidator() ExecutionParametersInvalidator
}
