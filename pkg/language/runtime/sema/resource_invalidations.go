package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/raviqqe/hamt"
)

type ResourceInvalidation struct {
	Kind ResourceInvalidationKind
	Pos  ast.Position
}

type ResourceInvalidations struct {
	invalidations hamt.Map
}

func NewResourceInvalidations() *ResourceInvalidations {
	return &ResourceInvalidations{
		invalidations: hamt.NewMap(),
	}
}

// Get returns all invalidations for the given variable.
//
func (p *ResourceInvalidations) Get(variable *Variable) []*ResourceInvalidation {
	key := VariableKey{variable: variable}
	existing := p.invalidations.Find(key)
	if existing == nil {
		return nil
	}
	return existing.([]*ResourceInvalidation)
}

func (p *ResourceInvalidations) Add(variable *Variable, invalidation *ResourceInvalidation) {
	invalidations := p.Get(variable)
	invalidations = append(invalidations, invalidation)
	key := VariableKey{variable: variable}
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

		variable := entry.(VariableKey).variable

		mergedInvalidations := append(p.Get(variable), value.([]*ResourceInvalidation)...)
		p.invalidations = p.invalidations.Insert(entry, mergedInvalidations)
	}
}
