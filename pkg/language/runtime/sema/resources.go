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
	Invalidations ResourceInvalidations
	// UsePositions is the set of uses
	UsePositions ResourceUses
}

// Resources is a map which contains invalidation info for resource variables.
//
type Resources struct {
	resources hamt.Map
	Returns   bool
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
	info.DefinitivelyInvalidated = true
	info.Invalidations = info.Invalidations.Insert(invalidation)
	ris.resources = ris.resources.Insert(key, info)
}

// AddUse adds the given use position to the set of use positions for the given resource variable.
//
func (ris *Resources) AddUse(variable *Variable, use ast.Position) {
	info := ris.Get(variable)
	info.UsePositions = info.UsePositions.Insert(use)
	key := VariableEntry{variable: variable}
	ris.resources = ris.resources.Insert(key, info)
}

func (ris *Resources) MarkUseAfterInvalidationReported(variable *Variable, pos ast.Position) {
	info := ris.Get(variable)
	info.UsePositions = info.UsePositions.MarkUseAfterInvalidationReported(pos)
	key := VariableEntry{variable: variable}
	ris.resources = ris.resources.Insert(key, info)
}

func (ris *Resources) IsUseAfterInvalidationReported(variable *Variable, pos ast.Position) bool {
	info := ris.Get(variable)
	return info.UsePositions.IsUseAfterInvalidationReported(pos)
}

func (ris *Resources) Clone() *Resources {
	return &Resources{
		resources: ris.resources,
		Returns:   ris.Returns,
	}
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

	infoTuples := NewBranchesResourceInfos(thenResources, elseResources)

	elseReturns := false
	if elseResources != nil {
		elseReturns = elseResources.Returns
	}

	for variable, infoTuple := range infoTuples {
		info := ris.Get(variable)

		// The resource variable can be considered definitely invalidated in both branches
		// if in both branches, there were invalidations or the branch returned.
		//
		// The assumption that a returning branch results in a definitive invalidation
		// can be made, because we check at the point of the return if the variable
		// was invalidated.

		definitelyInvalidatedInBranches :=
			(!infoTuple.thenInfo.Invalidations.IsEmpty() || thenResources.Returns) &&
				(!infoTuple.elseInfo.Invalidations.IsEmpty() || elseReturns)

		// The resource variable can be considered definitively invalidated
		// if it was already invalidated,
		// or the variable was invalidated in both branches

		info.DefinitivelyInvalidated =
			info.DefinitivelyInvalidated ||
				definitelyInvalidatedInBranches

		// If the a branch returns, the invalidations and uses won't have occurred in the outer scope,
		// so only merge invalidations and uses if the branch did not return

		if !thenResources.Returns {
			info.Invalidations = info.Invalidations.
				Merge(infoTuple.thenInfo.Invalidations)

			info.UsePositions = info.UsePositions.
				Merge(infoTuple.thenInfo.UsePositions)
		}

		if !elseReturns {
			info.Invalidations = info.Invalidations.
				Merge(infoTuple.elseInfo.Invalidations)

			info.UsePositions = info.UsePositions.
				Merge(infoTuple.elseInfo.UsePositions)
		}

		key := VariableEntry{variable: variable}
		ris.resources = ris.resources.Insert(key, info)
	}

	ris.Returns = ris.Returns ||
		(thenResources.Returns && elseReturns)
}

type BranchesResourceInfo struct {
	thenInfo ResourceInfo
	elseInfo ResourceInfo
}

type BranchesResourceInfos map[*Variable]BranchesResourceInfo

func (infos BranchesResourceInfos) Add(
	resources *Resources,
	setValue func(*BranchesResourceInfo, ResourceInfo),
) {
	var variable *Variable
	var resourceInfo ResourceInfo

	for resources.Size() != 0 {
		variable, resourceInfo, resources = resources.FirstRest()
		branchesResourceInfo := infos[variable]
		setValue(&branchesResourceInfo, resourceInfo)
		infos[variable] = branchesResourceInfo
	}
}

func NewBranchesResourceInfos(thenResources *Resources, elseResources *Resources) BranchesResourceInfos {
	infoTuples := make(BranchesResourceInfos)
	infoTuples.Add(
		thenResources,
		func(branchesResourceInfo *BranchesResourceInfo, resourceInfo ResourceInfo) {
			branchesResourceInfo.thenInfo = resourceInfo
		},
	)
	if elseResources != nil {
		infoTuples.Add(
			elseResources,
			func(branchesResourceInfo *BranchesResourceInfo, resourceInfo ResourceInfo) {
				branchesResourceInfo.elseInfo = resourceInfo
			},
		)
	}
	return infoTuples
}
