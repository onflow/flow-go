package derived

import (
	"github.com/onflow/flow-go/fvm/state"
)

type TableInvalidator[TKey comparable, TVal any] interface {
	// This returns true if the this invalidates any data
	ShouldInvalidateEntries() bool

	// This returns true if the table entry should be invalidated.
	ShouldInvalidateEntry(TKey, TVal, *state.State) bool
}

type tableInvalidatorAtTime[TKey comparable, TVal any] struct {
	TableInvalidator[TKey, TVal]

	executionTime LogicalTime
}

// NOTE: chainedInvalidator assumes that the entries are order by non-decreasing
// execution time.
type chainedTableInvalidators[TKey comparable, TVal any] []tableInvalidatorAtTime[TKey, TVal]

func (chained chainedTableInvalidators[TKey, TVal]) ApplicableInvalidators(
	toValidateTime LogicalTime,
) chainedTableInvalidators[TKey, TVal] {
	// NOTE: switch to bisection search (or reverse iteration) if the list
	// is long.
	for idx, entry := range chained {
		if toValidateTime <= entry.executionTime {
			return chained[idx:]
		}
	}

	return nil
}

func (chained chainedTableInvalidators[TKey, TVal]) ShouldInvalidateEntries() bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateEntries() {
			return true
		}
	}

	return false
}

func (chained chainedTableInvalidators[TKey, TVal]) ShouldInvalidateEntry(
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
