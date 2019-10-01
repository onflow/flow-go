package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/raviqqe/hamt"
)

// ResourceInfo is the info for a resource variable.
//
type ResourceInfo struct {
	// DefinitivelyInvalidated is true if the invalidation of the variable
	// can be considered definitive
	DefinitivelyInvalidated bool
	// Invalidations is the set of invalidations
	Invalidations ResourceInvalidationSet
	// UsePositions is the set of uses
	UsePositions Positions
}

// Resources is a map which contains invalidation info for resource variables.
//
type Resources struct {
	resources hamt.Map
}

// Get returns the invalidation info for the given variable.
//
func (ris *Resources) Get(variable *Variable) ResourceInfo {
	key := VariableEntry{variable: variable}
	existing := ris.resources.Find(key)
	if existing == nil {
		return ResourceInfo{}
	}
	return existing.(ResourceInfo)
}

// AddInvalidation adds the given invalidation to the set of invalidations for the given resource variable.
// Marks the variable to be definitely invalidated.
//
func (ris *Resources) AddInvalidation(variable *Variable, invalidation ResourceInvalidation) {
	key := VariableEntry{variable: variable}
	info := ris.Get(variable)
	invalidations := info.Invalidations.Insert(invalidation)
	info = ResourceInfo{
		DefinitivelyInvalidated: true,
		Invalidations:           invalidations,
		UsePositions:            info.UsePositions,
	}
	ris.resources = ris.resources.Insert(key, info)
}

// AddUse adds the given use position to the set of use positions for the given resource variable.
//
func (ris *Resources) AddUse(variable *Variable, use ast.Position) {
	key := VariableEntry{variable: variable}
	info := ris.Get(variable)
	uses := info.UsePositions.Insert(use)
	info = ResourceInfo{
		UsePositions:            uses,
		DefinitivelyInvalidated: info.DefinitivelyInvalidated,
		Invalidations:           info.Invalidations,
	}
	ris.resources = ris.resources.Insert(key, info)
}

func (ris *Resources) Clone() *Resources {
	return &Resources{ris.resources}
}

func (ris *Resources) Size() int {
	return ris.resources.Size()
}

func (ris *Resources) FirstRest() (*Variable, ResourceInfo, *Resources) {
	entry, value, rest := ris.resources.FirstRest()
	variable := entry.(VariableEntry).variable
	info := value.(ResourceInfo)
	resources := &Resources{resources: rest}
	return variable, info, resources
}

// MergeBranches merges the given resources from two branches into these resources.
// Invalidations occurring in both branches are considered definitive,
// other new invalidations are only considered potential.
// The else resources are optional.
//
func (ris *Resources) MergeBranches(thenResources *Resources, elseResources *Resources) {
	var variable *Variable
	var info ResourceInfo

	infoTuples := map[*Variable]struct {
		thenInfo, elseInfo ResourceInfo
	}{}

	for thenResources.Size() != 0 {
		variable, info, thenResources = thenResources.FirstRest()
		infoTuples[variable] = struct{ thenInfo, elseInfo ResourceInfo }{
			thenInfo: info,
		}
	}

	if elseResources != nil {
		for elseResources.Size() != 0 {
			variable, info, elseResources = elseResources.FirstRest()
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
			!infoTuple.thenInfo.Invalidations.IsEmpty() &&
				!infoTuple.elseInfo.Invalidations.IsEmpty()

		definitelyInvalidated :=
			info.DefinitivelyInvalidated ||
				definitelyInvalidatedInBranches

		invalidations := info.Invalidations.
			Merge(infoTuple.thenInfo.Invalidations).
			Merge(infoTuple.elseInfo.Invalidations)

		uses := info.UsePositions.
			Merge(infoTuple.thenInfo.UsePositions).
			Merge(infoTuple.elseInfo.UsePositions)

		key := VariableEntry{variable: variable}

		ris.resources = ris.resources.Insert(key,
			ResourceInfo{
				DefinitivelyInvalidated: definitelyInvalidated,
				Invalidations:           invalidations,
				UsePositions:            uses,
			},
		)
	}
}
