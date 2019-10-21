package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common/interface_entry"

	"github.com/raviqqe/hamt"
)

// ResourceInfo is the info for a resource.
//
type ResourceInfo struct {
	// DefinitivelyInvalidated is true if the invalidation of the resource
	// can be considered definitive
	DefinitivelyInvalidated bool
	// Invalidations is the set of invalidations of the resource
	Invalidations ResourceInvalidations
	// UsePositions is the set of uses of the resource
	UsePositions ResourceUses
}

// Resources is a map which contains invalidation info for resources.
//
type Resources struct {
	resources hamt.Map
	Returns   bool
}

// entry returns a `hamt` entry for the given resource.
//
func (ris *Resources) entry(resource interface{}) hamt.Entry {
	return interface_entry.InterfaceEntry{Interface: resource}
}

// Get returns the invalidation info for the given resource.
//
func (ris *Resources) Get(resource interface{}) ResourceInfo {
	entry := ris.entry(resource)
	existing := ris.resources.Find(entry)
	if existing == nil {
		return ResourceInfo{}
	}
	return existing.(ResourceInfo)
}

// AddInvalidation adds the given invalidation to the set of invalidations for the given resource.
// Marks the resource to be definitely invalidated.
//
func (ris *Resources) AddInvalidation(resource interface{}, invalidation ResourceInvalidation) {
	info := ris.Get(resource)
	info.DefinitivelyInvalidated = true
	info.Invalidations = info.Invalidations.Insert(invalidation)
	entry := ris.entry(resource)
	ris.resources = ris.resources.Insert(entry, info)
}

// AddUse adds the given use position to the set of use positions for the given resource.
//
func (ris *Resources) AddUse(resource interface{}, use ast.Position) {
	info := ris.Get(resource)
	info.UsePositions = info.UsePositions.Insert(use)
	entry := ris.entry(resource)
	ris.resources = ris.resources.Insert(entry, info)
}

func (ris *Resources) MarkUseAfterInvalidationReported(resource interface{}, pos ast.Position) {
	info := ris.Get(resource)
	info.UsePositions = info.UsePositions.MarkUseAfterInvalidationReported(pos)
	entry := ris.entry(resource)
	ris.resources = ris.resources.Insert(entry, info)
}

func (ris *Resources) IsUseAfterInvalidationReported(resource interface{}, pos ast.Position) bool {
	info := ris.Get(resource)
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

func (ris *Resources) FirstRest() (interface{}, ResourceInfo, *Resources) {
	entry, value, rest := ris.resources.FirstRest()
	resource := entry.(interface_entry.InterfaceEntry).Interface
	info := value.(ResourceInfo)
	resources := &Resources{resources: rest}
	return resource, info, resources
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

	for resource, infoTuple := range infoTuples {
		info := ris.Get(resource)

		// The resource can be considered definitely invalidated in both branches
		// if in both branches, there were invalidations or the branch returned.
		//
		// The assumption that a returning branch results in a definitive invalidation
		// can be made, because we check at the point of the return if the resource
		// was invalidated.

		definitelyInvalidatedInBranches :=
			(!infoTuple.thenInfo.Invalidations.IsEmpty() || thenResources.Returns) &&
				(!infoTuple.elseInfo.Invalidations.IsEmpty() || elseReturns)

		// The resource can be considered definitively invalidated if it was already invalidated,
		// or the resource was invalidated in both branches

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

		entry := ris.entry(resource)
		ris.resources = ris.resources.Insert(entry, info)
	}

	ris.Returns = ris.Returns ||
		(thenResources.Returns && elseReturns)
}

type BranchesResourceInfo struct {
	thenInfo ResourceInfo
	elseInfo ResourceInfo
}

type BranchesResourceInfos map[interface{}]BranchesResourceInfo

func (infos BranchesResourceInfos) Add(
	resources *Resources,
	setValue func(*BranchesResourceInfo, ResourceInfo),
) {
	var resource interface{}
	var resourceInfo ResourceInfo

	for resources.Size() != 0 {
		resource, resourceInfo, resources = resources.FirstRest()
		branchesResourceInfo := infos[resource]
		setValue(&branchesResourceInfo, resourceInfo)
		infos[resource] = branchesResourceInfo
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
