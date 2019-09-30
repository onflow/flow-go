package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	. "github.com/onsi/gomega"
	"testing"
)

func TestResourceInvalidations_Add(t *testing.T) {
	RegisterTestingT(t)

	invalidations := &ResourceInvalidations{}

	varX := &Variable{
		Type: &IntType{},
	}

	varY := &Variable{
		Type: &IntType{},
	}

	varZ := &Variable{
		Type: &IntType{},
	}

	Expect(invalidations.Get(varX).InvalidationSet.All()).
		To(BeEmpty())
	Expect(invalidations.Get(varY).InvalidationSet.All()).
		To(BeEmpty())
	Expect(invalidations.Get(varZ).InvalidationSet.All()).
		To(BeEmpty())

	// add invalidation for X

	invalidations.Add(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 1, Column: 1},
	})

	Expect(invalidations.Get(varX).InvalidationSet.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
		))
	Expect(invalidations.Get(varY).InvalidationSet.All()).
		To(BeEmpty())
	Expect(invalidations.Get(varZ).InvalidationSet.All()).
		To(BeEmpty())

	// add invalidation for X

	invalidations.Add(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 2, Column: 2},
	})

	Expect(invalidations.Get(varX).InvalidationSet.All()).
		To(ConsistOf(
			ResourceInvalidation{

				Pos: ast.Position{Line: 1, Column: 1},
			},
			ResourceInvalidation{

				Pos: ast.Position{Line: 2, Column: 2},
			},
		))
	Expect(invalidations.Get(varY).InvalidationSet.All()).
		To(BeEmpty())
	Expect(invalidations.Get(varZ).InvalidationSet.All()).
		To(BeEmpty())

	// add invalidation for Y

	invalidations.Add(varY, ResourceInvalidation{
		Pos: ast.Position{Line: 3, Column: 3},
	})

	Expect(invalidations.Get(varX).InvalidationSet.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
			ResourceInvalidation{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		))
	Expect(invalidations.Get(varY).InvalidationSet.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		))
	Expect(invalidations.Get(varZ).InvalidationSet.All()).
		To(BeEmpty())
}

func TestResourceInvalidations_FirstRest(t *testing.T) {
	RegisterTestingT(t)

	invalidations := &ResourceInvalidations{}

	varX := &Variable{
		Type: &IntType{},
	}

	varY := &Variable{
		Type: &IntType{},
	}

	// add invalidations for X and Y

	invalidations.Add(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 1, Column: 1},
	})

	invalidations.Add(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 2, Column: 2},
	})

	invalidations.Add(varY, ResourceInvalidation{
		Pos: ast.Position{Line: 3, Column: 3},
	})

	result := map[*Variable][]ResourceInvalidation{}

	var variable *Variable
	var invalidationInfo ResourceInvalidationInfo
	for invalidations.Size() != 0 {
		variable, invalidationInfo, invalidations = invalidations.FirstRest()
		result[variable] = invalidationInfo.InvalidationSet.All()
	}

	Expect(result).
		To(HaveLen(2))

	Expect(result[varX]).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
			ResourceInvalidation{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		))

	Expect(result[varY]).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		))
}

func TestResourceInvalidations_MergeBranches(t *testing.T) {
	RegisterTestingT(t)

	positions1 := &ResourceInvalidations{}
	positions2 := &ResourceInvalidations{}

	varX := &Variable{
		Type: &IntType{},
	}

	varY := &Variable{
		Type: &IntType{},
	}

	varZ := &Variable{
		Type: &IntType{},
	}

	positions1.Add(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 1, Column: 1},
	})
	positions1.Add(varY, ResourceInvalidation{
		Pos: ast.Position{Line: 2, Column: 2},
	})

	positions2.Add(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 3, Column: 3},
	})
	positions2.Add(varZ, ResourceInvalidation{
		Pos: ast.Position{Line: 4, Column: 4},
	})

	positions1.MergeBranches(positions1, positions2)

	varXInfo := positions1.Get(varX)
	Expect(varXInfo.DefinitivelyInvalidated).
		To(BeTrue())
	Expect(varXInfo.InvalidationSet.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
			ResourceInvalidation{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		))

	varYInfo := positions1.Get(varY)
	Expect(varYInfo.DefinitivelyInvalidated).
		To(BeFalse())
	Expect(varYInfo.InvalidationSet.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		))

	varZInfo := positions1.Get(varZ)
	Expect(varZInfo.DefinitivelyInvalidated).
		To(BeFalse())
	Expect(varZInfo.InvalidationSet.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 4, Column: 4},
			},
		))
}
