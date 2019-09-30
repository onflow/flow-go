package sema

import (
	"github.com/raviqqe/hamt"
)

type ResourceInvalidations struct {
	invalidations hamt.Map
}

// Get returns all invalidations for the given variable.
//
func (p *ResourceInvalidations) Get(variable *Variable) ResourceInvalidationSet {
	key := VariableEntry{variable: variable}
	existing := p.invalidations.Find(key)
	if existing == nil {
		return ResourceInvalidationSet{}
	}
	return existing.(ResourceInvalidationSet)
}

func (p *ResourceInvalidations) Add(variable *Variable, invalidation ResourceInvalidation) {
	key := VariableEntry{variable: variable}
	invalidations := p.Get(variable).Insert(invalidation)
	p.invalidations = p.invalidations.Insert(key, invalidations)
}

func (p *ResourceInvalidations) Clone() *ResourceInvalidations {
	return &ResourceInvalidations{p.invalidations}
}

// Merge merges the given invalidations into these invalidations.
// The invalidations for all variables are concatenated.
//
func (p *ResourceInvalidations) Merge(other *ResourceInvalidations) {
	otherInvalidations := other.invalidations
	for otherInvalidations.Size() != 0 {
		var entry hamt.Entry
		var value interface{}
		entry, value, otherInvalidations = otherInvalidations.FirstRest()
		variable := entry.(VariableEntry).variable
		mergedInvalidations := p.Get(variable).Merge(value.(ResourceInvalidationSet))
		p.invalidations = p.invalidations.Insert(entry, mergedInvalidations)
	}
}
