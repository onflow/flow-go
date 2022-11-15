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

var _ DerivedDataInvalidator[
	common.AddressLocation,
	*interpreter.Program,
] = ModifiedSetsInvalidator{}

type ModifiedSetsInvalidator struct {
	ContractUpdateKeys []ContractUpdateKey
	FrozenAccounts     []common.Address
}

func (sets ModifiedSetsInvalidator) ShouldInvalidateEntries() bool {
	return len(sets.ContractUpdateKeys) > 0 || len(sets.FrozenAccounts) > 0
}

func (sets ModifiedSetsInvalidator) ShouldInvalidateEntry(
	location common.AddressLocation,
	program *interpreter.Program,
	state *state.State,
) bool {
	// TODO(rbtz): switch to fine grain invalidation.
	return sets.ShouldInvalidateEntries()
}
