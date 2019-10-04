package sema

import "github.com/raviqqe/hamt"

type ResourceInvalidations struct {
	invalidations hamt.Set
}

func (ris ResourceInvalidations) All() (result []ResourceInvalidation) {
	s := ris.invalidations
	for s.Size() != 0 {
		var e hamt.Entry
		e, s = s.FirstRest()
		invalidation := e.(ResourceInvalidationEntry).ResourceInvalidation
		result = append(result, invalidation)
	}
	return
}

func (ris ResourceInvalidations) Include(invalidation ResourceInvalidation) bool {
	return ris.invalidations.Include(ResourceInvalidationEntry{
		ResourceInvalidation: invalidation,
	})
}

func (ris ResourceInvalidations) Insert(invalidation ResourceInvalidation) ResourceInvalidations {
	entry := ResourceInvalidationEntry{invalidation}
	newInvalidations := ris.invalidations.Insert(entry)
	return ResourceInvalidations{newInvalidations}
}

func (ris ResourceInvalidations) Merge(other ResourceInvalidations) ResourceInvalidations {
	newInvalidations := ris.invalidations.Merge(other.invalidations)
	return ResourceInvalidations{newInvalidations}
}

func (ris ResourceInvalidations) Size() int {
	return ris.invalidations.Size()
}

func (ris ResourceInvalidations) IsEmpty() bool {
	return ris.Size() == 0
}
