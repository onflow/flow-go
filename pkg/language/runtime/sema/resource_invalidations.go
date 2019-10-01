package sema

import (
	"github.com/raviqqe/hamt"
)

// ResourceInvalidationInfo is the invalidation info for a resource variable.
//
type ResourceInvalidationInfo struct {
	// DefinitivelyInvalidated is true if the invalidation of the variable
	// can be considered definitive
	DefinitivelyInvalidated bool
	// InvalidationSet the set of invalidations
	InvalidationSet ResourceInvalidationSet
}

func (i ResourceInvalidationInfo) IsEmpty() bool {
	return i.InvalidationSet.Size() == 0
}

// ResourceInvalidations is a map which contains invalidation info for resource variables.
//
type ResourceInvalidations struct {
	invalidations hamt.Map
}

// Get returns the invalidation info for the given variable.
//
func (ris *ResourceInvalidations) Get(variable *Variable) ResourceInvalidationInfo {
	key := VariableEntry{variable: variable}
	existing := ris.invalidations.Find(key)
	if existing == nil {
		return ResourceInvalidationInfo{}
	}
	return existing.(ResourceInvalidationInfo)
}

// Add adds the given invalidation to the set of invalidations for the given variable.
// Marks the variable to be definitely invalidated.
//
func (ris *ResourceInvalidations) Add(variable *Variable, invalidation ResourceInvalidation) {
	key := VariableEntry{variable: variable}
	invalidationInfo := ris.Get(variable)
	invalidationInfo = ResourceInvalidationInfo{
		DefinitivelyInvalidated: true,
		InvalidationSet:         invalidationInfo.InvalidationSet.Insert(invalidation),
	}
	ris.invalidations = ris.invalidations.Insert(key, invalidationInfo)
}

func (ris *ResourceInvalidations) Clone() *ResourceInvalidations {
	return &ResourceInvalidations{ris.invalidations}
}

func (ris *ResourceInvalidations) Size() int {
	return ris.invalidations.Size()
}

func (ris *ResourceInvalidations) FirstRest() (*Variable, ResourceInvalidationInfo, *ResourceInvalidations) {
	entry, value, rest := ris.invalidations.FirstRest()
	variable := entry.(VariableEntry).variable
	invalidationInfo := value.(ResourceInvalidationInfo)
	invalidations := &ResourceInvalidations{invalidations: rest}
	return variable, invalidationInfo, invalidations
}

// MergeBranches merges the given invalidations from two branches into these invalidations.
// Invalidations occurring in both branches are considered definitive,
// other new invalidations are only considered potential.
// The else invalidations are optional.
//
func (ris *ResourceInvalidations) MergeBranches(
	thenInvalidations *ResourceInvalidations,
	elseInvalidations *ResourceInvalidations,
) {
	var variable *Variable
	var info ResourceInvalidationInfo

	infoTuples := map[*Variable]struct {
		thenInfo, elseInfo ResourceInvalidationInfo
	}{}

	for thenInvalidations.Size() != 0 {
		variable, info, thenInvalidations = thenInvalidations.FirstRest()
		infoTuples[variable] = struct{ thenInfo, elseInfo ResourceInvalidationInfo }{
			thenInfo: info,
		}
	}

	if elseInvalidations != nil {
		for elseInvalidations.Size() != 0 {
			variable, info, elseInvalidations = elseInvalidations.FirstRest()
			infoTuple := infoTuples[variable]
			infoTuple.elseInfo = info
			infoTuples[variable] = infoTuple
		}
	}

	for variable, infoTuple := range infoTuples {
		info := ris.Get(variable)

		// the invalidation of the variable can be considered definitive
		// iff the variable was invalidated in both branches

		definitelyInvalidatedInBranches :=
			!infoTuple.thenInfo.IsEmpty() &&
				!infoTuple.elseInfo.IsEmpty()

		definitelyInvalidated :=
			info.DefinitivelyInvalidated ||
				definitelyInvalidatedInBranches

		invalidationSet := info.InvalidationSet.
			Merge(infoTuple.thenInfo.InvalidationSet).
			Merge(infoTuple.elseInfo.InvalidationSet)

		key := VariableEntry{variable: variable}

		ris.invalidations = ris.invalidations.Insert(key,
			ResourceInvalidationInfo{
				DefinitivelyInvalidated: definitelyInvalidated,
				InvalidationSet:         invalidationSet,
			},
		)
	}
}
