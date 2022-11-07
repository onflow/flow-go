package programs

type DerivedDataInvalidator[TVal any] interface {
	// This returns true if the this invalidates any data
	ShouldInvalidateEntries() bool

	// This returns true if the data entry should be invalidated.
	ShouldInvalidateEntry(TVal) bool
}

type derivedDataInvalidatorAtTime[TVal any] struct {
	DerivedDataInvalidator[TVal]

	executionTime LogicalTime
}

// NOTE: chainedInvalidator assumes that the entries are order by non-decreasing
// execution time.
type chainedDerivedDataInvalidators[TVal any] []derivedDataInvalidatorAtTime[TVal]

func (chained chainedDerivedDataInvalidators[TVal]) ApplicableInvalidators(
	snapshotTime LogicalTime,
) chainedDerivedDataInvalidators[TVal] {
	// NOTE: switch to bisection search (or reverse iteration) if the list
	// is long.
	for idx, entry := range chained {
		if snapshotTime <= entry.executionTime {
			return chained[idx:]
		}
	}

	return nil
}

func (chained chainedDerivedDataInvalidators[TVal]) ShouldInvalidateEntries() bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateEntries() {
			return true
		}
	}

	return false
}

func (chained chainedDerivedDataInvalidators[TVal]) ShouldInvalidateEntry(
	entry TVal,
) bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateEntry(entry) {
			return true
		}
	}

	return false
}
