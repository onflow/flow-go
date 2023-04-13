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

type ProgramInvalidator TableInvalidator[
	common.AddressLocation,
	*Program,
]

type MeterParamOverridesInvalidator TableInvalidator[
	struct{},
	MeterParamOverrides,
]

type TransactionInvalidator interface {
	ProgramInvalidator() ProgramInvalidator
	MeterParamOverridesInvalidator() MeterParamOverridesInvalidator
}
