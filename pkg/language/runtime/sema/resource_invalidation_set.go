package sema

import "github.com/raviqqe/hamt"

type ResourceInvalidationSet struct {
	invalidations hamt.Set
}

func (rs ResourceInvalidationSet) All() (result []ResourceInvalidation) {
	s := rs.invalidations
	for s.Size() != 0 {
		var e hamt.Entry
		e, s = s.FirstRest()
		invalidation := e.(ResourceInvalidationEntry).ResourceInvalidation
		result = append(result, invalidation)
	}
	return
}

func (rs ResourceInvalidationSet) Include(invalidation ResourceInvalidation) bool {
	return rs.invalidations.Include(ResourceInvalidationEntry{
		ResourceInvalidation: invalidation,
	})
}

func (rs ResourceInvalidationSet) Insert(invalidation ResourceInvalidation) ResourceInvalidationSet {
	entry := ResourceInvalidationEntry{invalidation}
	newInvalidations := rs.invalidations.Insert(entry)
	return ResourceInvalidationSet{newInvalidations}
}

func (rs ResourceInvalidationSet) Merge(other ResourceInvalidationSet) ResourceInvalidationSet {
	newInvalidations := rs.invalidations.Merge(other.invalidations)
	return ResourceInvalidationSet{newInvalidations}
}
