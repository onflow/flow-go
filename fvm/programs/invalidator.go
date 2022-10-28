package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/meter"
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

// For Programs
var _ OCCInvalidator[ProgramEntry] = OCCProgramsInvalidator{}

type OCCProgramsInvalidator struct {
	ContractUpdateKeys []ContractUpdateKey
	FrozenAccounts     []common.Address
}

func (sets OCCProgramsInvalidator) ShouldInvalidateItems() bool {
	return len(sets.ContractUpdateKeys) > 0 || len(sets.FrozenAccounts) > 0
}

func (sets OCCProgramsInvalidator) ShouldInvalidateEntry(
	_ ProgramEntry,
) bool {
	// TODO(rbtz): switch to fine grain invalidation.
	return sets.ShouldInvalidateItems()
}

// NOTE: chainedInvalidator assumes that the entries are order by non-decreasing
// execution time.
type chainedProgramInvalidators []occInvalidatorAtTime[ProgramEntry]

func (chained chainedProgramInvalidators) ApplicableInvalidators(
	snapshotTime LogicalTime,
) chainedProgramInvalidators {
	// NOTE: switch to bisection search (or reverse iteration) if the list
	// is long.
	for idx, entry := range chained {
		if snapshotTime <= entry.executionTime {
			return chained[idx:]
		}
	}

	return nil
}

func (chained chainedProgramInvalidators) ShouldInvalidateItems() bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateItems() {
			return true
		}
	}

	return false
}

func (chained chainedProgramInvalidators) ShouldInvalidateEntry(
	entry ProgramEntry,
) bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateEntry(entry) {
			return true
		}
	}

	return false
}

// For Meter Settings
var _ OCCInvalidator[meter.MeterParameters] = OCCMeterSettingsInvalidator{}

type OCCMeterSettingsInvalidator struct {
	MeterSettingMutated bool
}

func (sets OCCMeterSettingsInvalidator) ShouldInvalidateItems() bool {
	return sets.MeterSettingMutated
}

func (sets OCCMeterSettingsInvalidator) ShouldInvalidateEntry(
	_ meter.MeterParameters,
) bool {
	return sets.MeterSettingMutated
}
