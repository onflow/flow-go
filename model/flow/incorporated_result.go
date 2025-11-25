package flow

import "fmt"

// IncorporatedResult is a wrapper around an ExecutionResult which contains the
// ID of the first block on its fork in which it was incorporated.
//
//structwrite:immutable - mutations allowed only within the constructor
type IncorporatedResult struct {
	// IncorporatedBlockID is the ID of the first block on its fork where a
	// receipt for this result was incorporated. Within a fork, multiple blocks
	// may contain receipts for the same result; only the first one is used to
	// compute the random beacon of the result's chunk assignment.
	IncorporatedBlockID Identifier

	// Result is the ExecutionResult contained in the ExecutionReceipt that was
	// incorporated in the payload of IncorporatedBlockID.
	Result *ExecutionResult
}

// UntrustedIncorporatedResult is an untrusted input-only representation of an IncorporatedResult,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedIncorporatedResult should be validated and converted into
// a trusted IncorporatedResult using NewIncorporatedResult constructor.
type UntrustedIncorporatedResult IncorporatedResult

// NewIncorporatedResult creates a new instance of IncorporatedResult.
// Construction IncorporatedResult allowed only within the constructor
//
// All errors indicate a valid IncorporatedResult cannot be constructed from the input.
func NewIncorporatedResult(untrusted UntrustedIncorporatedResult) (*IncorporatedResult, error) {
	if untrusted.IncorporatedBlockID == ZeroID {
		return nil, fmt.Errorf("IncorporatedBlockID must not be empty")
	}

	if untrusted.Result == nil {
		return nil, fmt.Errorf("Result must not be empty")
	}

	return &IncorporatedResult{
		IncorporatedBlockID: untrusted.IncorporatedBlockID,
		Result:              untrusted.Result,
	}, nil
}

// ID returns a collision-resistant hash for the [IncorporatedResult] structure.
func (ir *IncorporatedResult) ID() Identifier {
	return MakeID(ir)
}

/*******************************************************************************
GROUPING allows to split a list incorporated results by some property
*******************************************************************************/

// IncorporatedResultList is a slice of IncorporatedResults with the additional
// functionality to group them by various properties
type IncorporatedResultList []*IncorporatedResult

// IncorporatedResultGroupedList is a partition of an IncorporatedResultList
type IncorporatedResultGroupedList map[Identifier]IncorporatedResultList

// IncorporatedResultGroupingFunction is a function that assigns an identifier to each IncorporatedResult
type IncorporatedResultGroupingFunction func(*IncorporatedResult) Identifier

// GroupBy partitions the IncorporatedResultList. All IncorporatedResults that are
// mapped by the grouping function to the same identifier are placed in the same group.
// Within each group, the order and multiplicity of the IncorporatedResults is preserved.
func (l IncorporatedResultList) GroupBy(grouper IncorporatedResultGroupingFunction) IncorporatedResultGroupedList {
	groups := make(map[Identifier]IncorporatedResultList)
	for _, ir := range l {
		groupID := grouper(ir)
		groups[groupID] = append(groups[groupID], ir)
	}
	return groups
}

// GroupByIncorporatedBlockID partitions the IncorporatedResultList by the ID of the block that
// incorporates the result. Within each group, the order and multiplicity of the
// IncorporatedResults is preserved.
func (l IncorporatedResultList) GroupByIncorporatedBlockID() IncorporatedResultGroupedList {
	grouper := func(ir *IncorporatedResult) Identifier { return ir.IncorporatedBlockID }
	return l.GroupBy(grouper)
}

// GroupByResultID partitions the IncorporatedResultList by the Results' IDs.
// Within each group, the order and multiplicity of the IncorporatedResults is preserved.
func (l IncorporatedResultList) GroupByResultID() IncorporatedResultGroupedList {
	grouper := func(ir *IncorporatedResult) Identifier { return ir.Result.ID() }
	return l.GroupBy(grouper)
}

// GroupByExecutedBlockID partitions the IncorporatedResultList by the IDs of the executed blocks.
// Within each group, the order and multiplicity of the IncorporatedResults is preserved.
func (l IncorporatedResultList) GroupByExecutedBlockID() IncorporatedResultGroupedList {
	grouper := func(ir *IncorporatedResult) Identifier { return ir.Result.BlockID }
	return l.GroupBy(grouper)
}

// Size returns the number of IncorporatedResults in the list
func (l IncorporatedResultList) Size() int {
	return len(l)
}

// GetGroup returns the IncorporatedResults that were mapped to the same identifier by the
// grouping function. Returns an empty (nil) IncorporatedResultList if groupID does not exist.
func (g IncorporatedResultGroupedList) GetGroup(groupID Identifier) IncorporatedResultList {
	return g[groupID]
}

// NumberGroups returns the number of groups
func (g IncorporatedResultGroupedList) NumberGroups() int {
	return len(g)
}
