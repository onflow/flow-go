package id

import "github.com/onflow/flow-go/model/flow"

// Any is a flow.IdentifierFilter. It accepts all identifiers.
func Any(flow.Identifier) bool {
	return true
}

// False is a flow.IdentifierFilter. It accepts no identifier.
func False(flow.Identifier) bool {
	return false
}

// And combines two or more filters that all need to be true.
func And(filters ...flow.IdentifierFilter) flow.IdentifierFilter {
	return func(id flow.Identifier) bool {
		for _, filter := range filters {
			if !filter(id) {
				return false
			}
		}
		return true
	}
}

// Or combines two or more filters and only needs one of them to be true.
func Or(filters ...flow.IdentifierFilter) flow.IdentifierFilter {
	return func(id flow.Identifier) bool {
		for _, filter := range filters {
			if filter(id) {
				return true
			}
		}
		return false
	}
}

// Not returns a filter equivalent to the inverse of the input filter.
func Not(filter flow.IdentifierFilter) flow.IdentifierFilter {
	return func(id flow.Identifier) bool {
		return !filter(id)
	}
}

// In constructs a filter that returns true if and only if
// the Identifier is in the provided list.
func In(ids ...flow.Identifier) flow.IdentifierFilter {
	lookup := make(map[flow.Identifier]struct{})
	for _, nodeID := range ids {
		lookup[nodeID] = struct{}{}
	}
	return func(id flow.Identifier) bool {
		_, ok := lookup[id]
		return ok
	}
}

// Is constructs a filter that returns true if and only if
// the Identifier is identical to the expectedID.
func Is(expectedID flow.Identifier) flow.IdentifierFilter {
	return func(id flow.Identifier) bool {
		return id == expectedID
	}
}

// InSet constructs a filter that returns true if and only if
// the Identifier is in the provided map.
// CAUTION: input map is _not_ copied
func InSet(ids map[flow.Identifier]struct{}) flow.IdentifierFilter {
	return func(id flow.Identifier) bool {
		_, ok := ids[id]
		return ok
	}
}
