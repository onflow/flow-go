package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	. "github.com/onsi/gomega"
	"testing"
)

func TestResourceInvalidations_Add(t *testing.T) {
	RegisterTestingT(t)

	invalidations := NewResourceInvalidations()

	varX := &Variable{
		Type: &IntType{},
	}

	varY := &Variable{
		Type: &IntType{},
	}

	varZ := &Variable{
		Type: &IntType{},
	}

	Expect(invalidations.Get(varX)).
		To(BeNil())
	Expect(invalidations.Get(varY)).
		To(BeNil())
	Expect(invalidations.Get(varZ)).
		To(BeNil())

	invalidations.Add(varX, &ResourceInvalidation{
		Pos: ast.Position{Line: 1, Column: 1},
	})

	Expect(invalidations.Get(varX)).
		To(ConsistOf(
			&ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
		))
	Expect(invalidations.Get(varY)).
		To(BeNil())
	Expect(invalidations.Get(varZ)).
		To(BeNil())

	invalidations.Add(varX, &ResourceInvalidation{
		Pos: ast.Position{Line: 2, Column: 2},
	})

	Expect(invalidations.Get(varX)).
		To(ConsistOf(
			&ResourceInvalidation{

				Pos: ast.Position{Line: 1, Column: 1},
			},
			&ResourceInvalidation{

				Pos: ast.Position{Line: 2, Column: 2},
			},
		))
	Expect(invalidations.Get(varY)).
		To(BeNil())
	Expect(invalidations.Get(varZ)).
		To(BeNil())

	invalidations.Add(varY, &ResourceInvalidation{
		Pos: ast.Position{Line: 3, Column: 3},
	})

	Expect(invalidations.Get(varX)).
		To(ConsistOf(
			&ResourceInvalidation{
				Pos: ast.Position{Line: 1, Column: 1},
			},
			&ResourceInvalidation{
				Pos: ast.Position{Line: 2, Column: 2},
			},
		))
	Expect(invalidations.Get(varY)).
		To(ConsistOf(
			&ResourceInvalidation{
				Pos: ast.Position{Line: 3, Column: 3},
			},
		))
	Expect(invalidations.Get(varZ)).
		To(BeNil())

}

//
//func TestResourceInvalidations_Merge(t *testing.T) {
//	RegisterTestingT(t)
//
//	positions1 := NewResourceInvalidations()
//	positions2 := NewResourceInvalidations()
//
//	varX := &Variable{
//		Type: &IntType{},
//	}
//
//	varY := &Variable{
//		Type: &IntType{},
//	}
//
//	varZ := &Variable{
//		Type: &IntType{},
//	}
//
//	positions1.Add(varX, ast.Position{Line: 1, Column: 1})
//	positions1.Add(varY, ast.Position{Line: 2, Column: 2})
//
//	positions2.Add(varX, ast.Position{Line: 3, Column: 3})
//	positions2.Add(varZ, ast.Position{Line: 4, Column: 4})
//
//	positions1.Merge(positions2)
//
//	Expect(positions1.Get(varX)).
//		To(ConsistOf(
//			ast.Position{Line: 1, Column: 1},
//			ast.Position{Line: 3, Column: 3},
//		))
//
//	Expect(positions1.Get(varY)).
//		To(ConsistOf(
//			ast.Position{Line: 2, Column: 2},
//		))
//
//	Expect(positions1.Get(varZ)).
//		To(ConsistOf(
//			ast.Position{Line: 4, Column: 4},
//		))
//}
