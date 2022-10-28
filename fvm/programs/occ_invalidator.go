package programs

type OCCInvalidator[TVal any] interface {
	// This returns true if the this invalidates at least one program
	ShouldInvalidateItems() bool

	// This returns true if the program entry should be invalidated.
	ShouldInvalidateEntry(TVal) bool
}

type occInvalidatorAtTime[TVal any] struct {
	OCCInvalidator[TVal]

	executionTime LogicalTime
}

// NOTE: chainedInvalidator assumes that the entries are order by non-decreasing
// execution time.
type chainedOCCInvalidators[TVal any] []occInvalidatorAtTime[TVal]

func (chained chainedOCCInvalidators[TVal]) ApplicableInvalidators(
	snapshotTime LogicalTime,
) chainedOCCInvalidators[TVal] {
	// NOTE: switch to bisection search (or reverse iteration) if the list
	// is long.
	for idx, entry := range chained {
		if snapshotTime <= entry.executionTime {
			return chained[idx:]
		}
	}

	return nil
}

func (chained chainedOCCInvalidators[TVal]) ShouldInvalidateItems() bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateItems() {
			return true
		}
	}

	return false
}

func (chained chainedOCCInvalidators[TVal]) ShouldInvalidateEntry(
	entry TVal,
) bool {
	for _, invalidator := range chained {
		if invalidator.ShouldInvalidateEntry(entry) {
			return true
		}
	}

	return false
}
