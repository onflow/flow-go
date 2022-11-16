package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

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

type ProgramInvalidator DerivedDataInvalidator[
	common.AddressLocation,
	*interpreter.Program,
]

type TransactionInvalidator interface {
	ProgramInvalidator() ProgramInvalidator
}

type ModifiedSetsInvalidator struct {
	ContractUpdateKeys []ContractUpdateKey
	FrozenAccounts     []common.Address
}

func (sets ModifiedSetsInvalidator) ProgramInvalidator() ProgramInvalidator {
	return ModifiedSetsProgramInvalidator{sets}
}

type ModifiedSetsProgramInvalidator struct {
	ModifiedSetsInvalidator
}

var _ ProgramInvalidator = ModifiedSetsProgramInvalidator{}

func (sets ModifiedSetsProgramInvalidator) ShouldInvalidateEntries() bool {
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
