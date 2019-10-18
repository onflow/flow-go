package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestResources_Add(t *testing.T) {
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

	assert.Empty(t, resources.Get(varX).Invalidations.All())
	assert.Empty(t, resources.Get(varY).Invalidations.All())
	assert.Empty(t, resources.Get(varZ).Invalidations.All())

	// add invalidation for X

	resources.AddInvalidation(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 1, Column: 1},
	})

	assert.ElementsMatch(t,
		resources.Get(varX).Invalidations.All(),
		[]ResourceInvalidation{
			{
				Pos: ast.Position{Line: 1, Column: 1},
			},
		},
	)
	assert.Empty(t, resources.Get(varY).Invalidations.All())
	assert.Empty(t, resources.Get(varZ).Invalidations.All())

	// add invalidation for X

	resources.AddInvalidation(varX, ResourceInvalidation{
		Pos: ast.Position{Line: 2, Column: 2},
	})

	assert.ElementsMatch(t,
		resources.Get(varX).Invalidations.All(),
		[]ResourceInvalidation{
			{

				Pos: ast.Position{Line: 1, Column: 1},
			},
			{

				Pos: ast.Position{Line: 2, Column: 2},
			},
		},
	)
	assert.Empty(t, resources.Get(varY).Invalidations.All())
	assert.Empty(t, resources.Get(varZ).Invalidations.All())

	// add invalidation for Y

	resources.AddInvalidation(varY, ResourceInvalidation{
		Pos: ast.Position{Line: 3, Column: 3},
	})

	assert.ElementsMatch(t,
		resources.Get(varX).Invalidations.All(),
		[]ResourceInvalidation{
			{
				Pos: ast.Position{Line: 1, Column: 1},
			},

			{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		},
	)
	assert.ElementsMatch(t,
		resources.Get(varY).Invalidations.All(),
		[]ResourceInvalidation{
			{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		},
	)
	assert.Empty(t, resources.Get(varZ).Invalidations.All())
}

func TestResourceResources_FirstRest(t *testing.T) {

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

	var resource interface{}
	var resourceInfo ResourceInfo
	for resources.Size() != 0 {
		resource, resourceInfo, resources = resources.FirstRest()
		variable := resource.(*Variable)
		result[variable] = resourceInfo.Invalidations.All()
	}

	assert.Len(t, result, 2)

	assert.ElementsMatch(t,
		result[varX],
		[]ResourceInvalidation{
			{
				Pos: ast.Position{Line: 1, Column: 1},
			},
			{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		},
	)

	assert.ElementsMatch(t,
		result[varY],
		[]ResourceInvalidation{
			{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		},
	)
}

func TestResources_MergeBranches(t *testing.T) {

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
	assert.True(t, varXInfo.DefinitivelyInvalidated)
	assert.ElementsMatch(t,
		varXInfo.Invalidations.All(),
		[]ResourceInvalidation{
			{
				Pos: ast.Position{Line: 1, Column: 1},
			},
			{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		},
	)

	varYInfo := resources.Get(varY)
	assert.True(t, varYInfo.DefinitivelyInvalidated)
	assert.ElementsMatch(t,
		varYInfo.Invalidations.All(),
		[]ResourceInvalidation{
			{
				Pos: ast.Position{Line: 0, Column: 0},
			},
			{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		},
	)

	varZInfo := resources.Get(varZ)
	assert.False(t, varZInfo.DefinitivelyInvalidated)
	assert.ElementsMatch(t,
		varZInfo.Invalidations.All(),
		[]ResourceInvalidation{
			{
				Pos: ast.Position{Line: 4, Column: 4},
			},
		},
	)
}
