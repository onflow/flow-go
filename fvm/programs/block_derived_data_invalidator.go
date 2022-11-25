package programs

import (
	"github.com/onflow/flow-go/fvm/state"
)

type DerivedDataInvalidator[TKey comparable, TVal any] interface {
	// This returns true if the this invalidates any data
	ShouldInvalidateEntries() bool

	// This returns true if the data entry should be invalidated.
	ShouldInvalidateEntry(TKey, TVal, *state.State) bool
}

type derivedDataInvalidatorAtTime[TKey comparable, TVal any] struct {
	DerivedDataInvalidator[TKey, TVal]

	executionTime LogicalTime
}

// NOTE: chainedInvalidator assumes that the entries are order by non-decreasing
// execution time.
type chainedDerivedDataInvalidators[TKey comparable, TVal any] []derivedDataInvalidatorAtTime[TKey, TVal]

func (chained chainedDerivedDataInvalidators[TKey, TVal]) ApplicableInvalidators(
	snapshotTime LogicalTime,
) chainedDerivedDataInvalidators[TKey, TVal] {
	// NOTE: switch to bisection search (or reverse iteration) if the list
	// is long.
	for idx, entry := range chained {
		if snapshotTime <= entry.executionTime {
			return chained[idx:]
		}
	}

	return nil
}

func (chained chainedDerivedDataInvalidators[TKey, TVal]) ShouldInvalidateEntries() bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateEntries() {
			return true
		}
	}

	return false
}

func (chained chainedDerivedDataInvalidators[TKey, TVal]) ShouldInvalidateEntry(
	key TKey,
	value TVal,
	state *state.State,
) bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateEntry(key, value, state) {
			return true
		}
	}

	return false
}
