package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type ProgramEntry struct {
	Location common.AddressLocation
	Program  *interpreter.Program
	State    *state.State
}
type ContractUpdateKey struct {
	Address flow.Address
	Name    string
}

type ContractUpdate struct {
	ContractUpdateKey
	Code []byte
}

var _ DerivedDataInvalidator[ProgramEntry] = ModifiedSetsInvalidator{}

type ModifiedSetsInvalidator struct {
	ContractUpdateKeys []ContractUpdateKey
	FrozenAccounts     []common.Address
}

func (sets ModifiedSetsInvalidator) ShouldInvalidateEntries() bool {
	return len(sets.ContractUpdateKeys) > 0 || len(sets.FrozenAccounts) > 0
}

func (sets ModifiedSetsInvalidator) ShouldInvalidateEntry(
	entry ProgramEntry,
) bool {
	// TODO(rbtz): switch to fine grain invalidation.
	return sets.ShouldInvalidateEntries()
}
