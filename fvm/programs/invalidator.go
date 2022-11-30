package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type ContractUpdateKey struct {
	Address flow.Address
	Name    string
}

type ContractUpdate struct {
	ContractUpdateKey
	Code []byte
}

type MeterParamOverrides struct {
	ComputationWeights meter.ExecutionEffortWeights // nil indicates no override
	MemoryWeights      meter.ExecutionMemoryWeights // nil indicates no override
	MemoryLimit        *uint64                      // nil indicates no override
}

type ProgramInvalidator TableInvalidator[
	common.AddressLocation,
	*interpreter.Program,
]

type MeterParamOverridesInvalidator TableInvalidator[
	struct{},
	MeterParamOverrides,
]

type TransactionInvalidator interface {
	ProgramInvalidator() ProgramInvalidator
	MeterParamOverridesInvalidator() MeterParamOverridesInvalidator
}

type ModifiedSetsInvalidator struct {
	ContractUpdateKeys []ContractUpdateKey
	FrozenAccounts     []common.Address
}

func (sets ModifiedSetsInvalidator) ProgramInvalidator() ProgramInvalidator {
	return ModifiedSetsProgramInvalidator{sets}
}

func (sets ModifiedSetsInvalidator) MeterParamOverridesInvalidator() MeterParamOverridesInvalidator {
	return ModifiedSetsMeterParamOverridesInvalidator{sets}
}

type ModifiedSetsProgramInvalidator struct {
	ModifiedSetsInvalidator
}

var _ ProgramInvalidator = ModifiedSetsProgramInvalidator{}

func (sets ModifiedSetsProgramInvalidator) ShouldInvalidateEntries() bool {
	// TODO(patrick): invalidate on meter param updates
	return len(sets.ContractUpdateKeys) > 0 || len(sets.FrozenAccounts) > 0
}

func (sets ModifiedSetsProgramInvalidator) ShouldInvalidateEntry(
	location common.AddressLocation,
	program *interpreter.Program,
	state *state.State,
) bool {
	// TODO(rbtz): switch to fine grain invalidation.
	return sets.ShouldInvalidateEntries()
}

type ModifiedSetsMeterParamOverridesInvalidator struct {
	ModifiedSetsInvalidator
}

func (sets ModifiedSetsMeterParamOverridesInvalidator) ShouldInvalidateEntries() bool {
	// TODO(patrick): invalidate on meter param updates instead of contract
	// updates
	return len(sets.ContractUpdateKeys) > 0 || len(sets.FrozenAccounts) > 0
}

func (sets ModifiedSetsMeterParamOverridesInvalidator) ShouldInvalidateEntry(
	_ struct{},
	_ MeterParamOverrides,
	_ *state.State,
) bool {
	return sets.ShouldInvalidateEntries()
}
