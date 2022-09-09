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

type Invalidator interface {
	// This returns true if the this invalidates at least one program
	ShouldInvalidatePrograms() bool

	// This returns true if the program entry should be invalidated.
	ShouldInvalidateEntry(ProgramEntry) bool
}

type ContractUpdateKey struct {
	Address flow.Address
	Name    string
}

type ContractUpdate struct {
	ContractUpdateKey
	Code []byte
}

var _ Invalidator = ModifiedSetsInvalidator{}

type ModifiedSetsInvalidator struct {
	ContractUpdateKeys []ContractUpdateKey
	FrozenAccounts     []common.Address
}

func (sets ModifiedSetsInvalidator) ShouldInvalidatePrograms() bool {
	return len(sets.ContractUpdateKeys) > 0 || len(sets.FrozenAccounts) > 0
}

func (sets ModifiedSetsInvalidator) ShouldInvalidateEntry(
	entry ProgramEntry,
) bool {
	// TODO(rbtz): switch to fine grain invalidation.
	return sets.ShouldInvalidatePrograms()
}

type invalidatorAtTime struct {
	Invalidator

	executionTime LogicalTime
}

// NOTE: chainedInvalidator assumes that the entries are order by non-decreasing
// execution time.
type chainedInvalidators []invalidatorAtTime

func (chained chainedInvalidators) ApplicableInvalidators(
	snapshotTime LogicalTime,
) chainedInvalidators {
	// NOTE: switch to bisection search (or reverse iteration) if the list
	// is long.
	for idx, entry := range chained {
		if snapshotTime <= entry.executionTime {
			return chained[idx:]
		}
	}

	return nil
}

func (chained chainedInvalidators) ShouldInvalidatePrograms() bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidatePrograms() {
			return true
		}
	}

	return false
}

func (chained chainedInvalidators) ShouldInvalidateEntry(
	entry ProgramEntry,
) bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateEntry(entry) {
			return true
		}
	}

	return false
}
