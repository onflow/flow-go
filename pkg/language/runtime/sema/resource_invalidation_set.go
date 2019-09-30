package sema

import "github.com/raviqqe/hamt"

type ResourceInvalidationSet struct {
	invalidations hamt.Set
}

func (ris ResourceInvalidationSet) All() (result []ResourceInvalidation) {
	s := ris.invalidations
	for s.Size() != 0 {
		var e hamt.Entry
		e, s = s.FirstRest()
		invalidation := e.(ResourceInvalidationEntry).ResourceInvalidation
		result = append(result, invalidation)
	}
	return
}

func (ris ResourceInvalidationSet) Include(invalidation ResourceInvalidation) bool {
	return ris.invalidations.Include(ResourceInvalidationEntry{
		ResourceInvalidation: invalidation,
	})
}

func (ris ResourceInvalidationSet) Insert(invalidation ResourceInvalidation) ResourceInvalidationSet {
	entry := ResourceInvalidationEntry{invalidation}
	newInvalidations := ris.invalidations.Insert(entry)
	return ResourceInvalidationSet{newInvalidations}
}

func (ris ResourceInvalidationSet) Merge(other ResourceInvalidationSet) ResourceInvalidationSet {
	newInvalidations := ris.invalidations.Merge(other.invalidations)
	return ResourceInvalidationSet{newInvalidations}
}

func (ris ResourceInvalidationSet) Size() int {
	return ris.invalidations.Size()
}
