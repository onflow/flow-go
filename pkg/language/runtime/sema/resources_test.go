package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	. "github.com/onsi/gomega"
	"testing"
)

func TestResources_Add(t *testing.T) {
	RegisterTestingT(t)

	resources := &Resources{}

	varX := &Variable{
		Identifier: "x",
		Type:       &IntType{},
	}

	varY := &Variable{
		Identifier: "y",
		Type:       &IntType{},
	}

	varZ := &Variable{
		Identifier: "z",
		Type:       &IntType{},
	}

	Expect(resources.Get(varX).Invalidations.All()).
		To(BeEmpty())
	Expect(resources.Get(varY).Invalidations.All()).
		To(BeEmpty())
	Expect(resources.Get(varZ).Invalidations.All()).
		To(BeEmpty())

	// add invalidation for X

	resources.AddInvalidation(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 1, Column: 1},
	})

	Expect(resources.Get(varX).Invalidations.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
		))
	Expect(resources.Get(varY).Invalidations.All()).
		To(BeEmpty())
	Expect(resources.Get(varZ).Invalidations.All()).
		To(BeEmpty())

	// add invalidation for X

	resources.AddInvalidation(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 2, Column: 2},
	})

	Expect(resources.Get(varX).Invalidations.All()).
		To(ConsistOf(
			ResourceInvalidation{

				Pos: ast.Position{Line: 1, Column: 1},
			},
			ResourceInvalidation{

				Pos: ast.Position{Line: 2, Column: 2},
			},
		))
	Expect(resources.Get(varY).Invalidations.All()).
		To(BeEmpty())
	Expect(resources.Get(varZ).Invalidations.All()).
		To(BeEmpty())

	// add invalidation for Y

	resources.AddInvalidation(varY, ResourceInvalidation{
		Pos: ast.Position{Line: 3, Column: 3},
	})

	Expect(resources.Get(varX).Invalidations.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
			ResourceInvalidation{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		))
	Expect(resources.Get(varY).Invalidations.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		))
	Expect(resources.Get(varZ).Invalidations.All()).
		To(BeEmpty())
}

func TestResourceResources_FirstRest(t *testing.T) {
	RegisterTestingT(t)

	resources := &Resources{}

	varX := &Variable{
		Identifier: "x",
		Type:       &IntType{},
	}

	varY := &Variable{
		Identifier: "y",
		Type:       &IntType{},
	}

	// add resources for X and Y

	resources.AddInvalidation(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 1, Column: 1},
	})

	resources.AddInvalidation(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 2, Column: 2},
	})

	resources.AddInvalidation(varY, ResourceInvalidation{
		Pos: ast.Position{Line: 3, Column: 3},
	})

	result := map[*Variable][]ResourceInvalidation{}

	var variable *Variable
	var resourceInfo ResourceInfo
	for resources.Size() != 0 {
		variable, resourceInfo, resources = resources.FirstRest()
		result[variable] = resourceInfo.Invalidations.All()
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

func TestResources_MergeBranches(t *testing.T) {
	RegisterTestingT(t)

	resourcesThen := &Resources{}
	resourcesElse := &Resources{}

	varX := &Variable{
		Identifier: "x",
		Type:       &IntType{},
	}

	varY := &Variable{
		Identifier: "y",
		Type:       &IntType{},
	}

	varZ := &Variable{
		Identifier: "z",
		Type:       &IntType{},
	}

	// invalidate X and Y in then branch

	resourcesThen.AddInvalidation(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 1, Column: 1},
	})
	resourcesThen.AddInvalidation(varY, ResourceInvalidation{
		Pos: ast.Position{Line: 2, Column: 2},
	})

	// invalidate Y and Z in else branch

	resourcesElse.AddInvalidation(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 3, Column: 3},
	})
	resourcesElse.AddInvalidation(varZ, ResourceInvalidation{
		Pos: ast.Position{Line: 4, Column: 4},
	})

	// treat var Y already invalidated in main
	resources := &Resources{}
	resources.AddInvalidation(varY, ResourceInvalidation{
		Pos: ast.Position{Line: 0, Column: 0},
	})

	resources.MergeBranches(
		resourcesThen,
		resourcesElse,
	)

	varXInfo := resources.Get(varX)
	Expect(varXInfo.DefinitivelyInvalidated).
		To(BeTrue())
	Expect(varXInfo.Invalidations.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
			ResourceInvalidation{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		))

	varYInfo := resources.Get(varY)
	Expect(varYInfo.DefinitivelyInvalidated).
		To(BeTrue())
	Expect(varYInfo.Invalidations.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 0, Column: 0},
			},
			ResourceInvalidation{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		))

	varZInfo := resources.Get(varZ)
	Expect(varZInfo.DefinitivelyInvalidated).
		To(BeFalse())
	Expect(varZInfo.Invalidations.All()).
		To(ConsistOf(
			ResourceInvalidation{
				Pos: ast.Position{Line: 4, Column: 4},
			},
		))
}
