package environment

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
)

type ContractUpdate struct {
	Location common.AddressLocation
	Code     []byte
}

type ContractUpdates struct {
	Updates   []common.AddressLocation
	Deploys   []common.AddressLocation
	Deletions []common.AddressLocation
}

func (u ContractUpdates) Any() bool {
	return len(u.Updates) > 0 || len(u.Deploys) > 0 || len(u.Deletions) > 0
}

type DerivedDataInvalidator struct {
	ContractUpdates

	MeterParamOverridesUpdated bool
}

var _ derived.TransactionInvalidator = DerivedDataInvalidator{}

func NewDerivedDataInvalidator(
	contractUpdates ContractUpdates,
	executionSnapshot *snapshot.ExecutionSnapshot,
	meterStateRead *snapshot.ExecutionSnapshot,
) DerivedDataInvalidator {
	return DerivedDataInvalidator{
		ContractUpdates: contractUpdates,
		MeterParamOverridesUpdated: meterParamOverridesUpdated(
			executionSnapshot,
			meterStateRead),
	}
}

// meterParamOverridesUpdated returns true if the meter param overrides have been updated
// this is done by checking if the registers needed to compute the meter param overrides
// have been touched in the execution snapshot
func meterParamOverridesUpdated(
	executionSnapshot *snapshot.ExecutionSnapshot,
	meterStateRead *snapshot.ExecutionSnapshot,
) bool {
	if len(executionSnapshot.WriteSet) > len(meterStateRead.ReadSet) {
		for registerId := range meterStateRead.ReadSet {
			_, ok := executionSnapshot.WriteSet[registerId]
			if ok {
				return true
			}
		}
	} else {
		for registerId := range executionSnapshot.WriteSet {
			_, ok := meterStateRead.ReadSet[registerId]
			if ok {
				return true
			}
		}
	}

	return false
}

func (invalidator DerivedDataInvalidator) ProgramInvalidator() derived.ProgramInvalidator {
	return ProgramInvalidator{invalidator}
}

func (invalidator DerivedDataInvalidator) MeterParamOverridesInvalidator() derived.MeterParamOverridesInvalidator {
	return MeterParamOverridesInvalidator{invalidator}
}

type ProgramInvalidator struct {
	DerivedDataInvalidator
}

func (invalidator ProgramInvalidator) ShouldInvalidateEntries() bool {
	return invalidator.MeterParamOverridesUpdated ||
		invalidator.ContractUpdates.Any()
}

func (invalidator ProgramInvalidator) ShouldInvalidateEntry(
	_ common.AddressLocation,
	program *derived.Program,
	_ *snapshot.ExecutionSnapshot,
) bool {
	if invalidator.MeterParamOverridesUpdated {
		// if meter parameters changed we need to invalidate all programs
		return true
	}

	// invalidate all programs depending on any of the contracts that were
	// updated. A program has itself listed as a dependency, so that this
	// simpler.
	for _, loc := range invalidator.ContractUpdates.Updates {
		ok := program.Dependencies.ContainsLocation(loc)
		if ok {
			return true
		}
	}

	// In case a contract was deployed or removed from an address,
	// we need to invalidate all programs depending on that address.
	for _, loc := range invalidator.ContractUpdates.Deploys {
		ok := program.Dependencies.ContainsAddress(loc.Address)
		if ok {
			return true
		}
	}
	for _, loc := range invalidator.ContractUpdates.Deletions {
		ok := program.Dependencies.ContainsAddress(loc.Address)
		if ok {
			return true
		}
	}

	return false
}

type MeterParamOverridesInvalidator struct {
	DerivedDataInvalidator
}

func (invalidator MeterParamOverridesInvalidator) ShouldInvalidateEntries() bool {
	return invalidator.MeterParamOverridesUpdated
}

func (invalidator MeterParamOverridesInvalidator) ShouldInvalidateEntry(
	_ struct{},
	_ derived.MeterParamOverrides,
	_ *snapshot.ExecutionSnapshot,
) bool {
	return invalidator.MeterParamOverridesUpdated
}
