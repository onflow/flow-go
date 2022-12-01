package environment

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/programs"
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

type DerivedDataInvalidator struct {
	ContractUpdateKeys []ContractUpdateKey
	FrozenAccounts     []common.Address
}

var _ programs.TransactionInvalidator = DerivedDataInvalidator{}

func (invalidator DerivedDataInvalidator) ProgramInvalidator() programs.ProgramInvalidator {
	return ProgramInvalidator{invalidator}
}

func (invalidator DerivedDataInvalidator) MeterParamOverridesInvalidator() programs.MeterParamOverridesInvalidator {
	return MeterParamOverridesInvalidator{invalidator}
}

type ProgramInvalidator struct {
	DerivedDataInvalidator
}

func (invalidator ProgramInvalidator) ShouldInvalidateEntries() bool {
	// TODO(patrick): invalidate on meter param updates
	return len(invalidator.ContractUpdateKeys) > 0 ||
		len(invalidator.FrozenAccounts) > 0
}

func (invalidator ProgramInvalidator) ShouldInvalidateEntry(
	location common.AddressLocation,
	program *interpreter.Program,
	state *state.State,
) bool {
	// TODO(rbtz): switch to fine grain invalidation.
	return invalidator.ShouldInvalidateEntries()
}

type MeterParamOverridesInvalidator struct {
	DerivedDataInvalidator
}

func (invalidator MeterParamOverridesInvalidator) ShouldInvalidateEntries() bool {
	// TODO(patrick): invalidate on meter param updates instead of contract
	// updates
	return len(invalidator.ContractUpdateKeys) > 0 ||
		len(invalidator.FrozenAccounts) > 0
}

func (invalidator MeterParamOverridesInvalidator) ShouldInvalidateEntry(
	_ struct{},
	_ programs.MeterParamOverrides,
	_ *state.State,
) bool {
	return invalidator.ShouldInvalidateEntries()
}
