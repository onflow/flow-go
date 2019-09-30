package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	. "github.com/onsi/gomega"
	"testing"
)

func TestVariablePositions_Add(t *testing.T) {
	RegisterTestingT(t)

	positions := NewVariablePositions()

	varX := &Variable{
		Type: &IntType{},
	}

	varY := &Variable{
		Type: &IntType{},
	}

	varZ := &Variable{
		Type: &IntType{},
	}

	Expect(positions.Get(varX)).
		To(BeNil())
	Expect(positions.Get(varY)).
		To(BeNil())
	Expect(positions.Get(varZ)).
		To(BeNil())

	positions.Add(varX, ast.Position{Line: 1, Column: 1})

	Expect(positions.Get(varX)).
		To(ConsistOf(
			ast.Position{Line: 1, Column: 1},
		))
	Expect(positions.Get(varY)).
		To(BeNil())
	Expect(positions.Get(varZ)).
		To(BeNil())

	positions.Add(varX, ast.Position{Line: 2, Column: 2})

	Expect(positions.Get(varX)).
		To(ConsistOf(
			ast.Position{Line: 1, Column: 1},
			ast.Position{Line: 2, Column: 2},
		))
	Expect(positions.Get(varY)).
		To(BeNil())
	Expect(positions.Get(varZ)).
		To(BeNil())

	positions.Add(varY, ast.Position{Line: 3, Column: 3})

	Expect(positions.Get(varX)).
		To(ConsistOf(
			ast.Position{Line: 1, Column: 1},
			ast.Position{Line: 2, Column: 2},
		))
	Expect(positions.Get(varY)).
		To(ConsistOf(
			ast.Position{Line: 3, Column: 3},
		))
	Expect(positions.Get(varZ)).
		To(BeNil())

}

func TestVariablePositions_Merge(t *testing.T) {
	RegisterTestingT(t)

	positions1 := NewVariablePositions()
	positions2 := NewVariablePositions()

	varX := &Variable{
		Type: &IntType{},
	}

	varY := &Variable{
		Type: &IntType{},
	}

	varZ := &Variable{
		Type: &IntType{},
	}

	positions1.Add(varX, ast.Position{Line: 1, Column: 1})
	positions1.Add(varY, ast.Position{Line: 2, Column: 2})

	positions2.Add(varX, ast.Position{Line: 3, Column: 3})
	positions2.Add(varZ, ast.Position{Line: 4, Column: 4})

	positions1.Merge(positions2)

	Expect(positions1.Get(varX)).
		To(ConsistOf(
			ast.Position{Line: 1, Column: 1},
			ast.Position{Line: 3, Column: 3},
		))

	Expect(positions1.Get(varY)).
		To(ConsistOf(
			ast.Position{Line: 2, Column: 2},
		))

	Expect(positions1.Get(varZ)).
		To(ConsistOf(
			ast.Position{Line: 4, Column: 4},
		))
}
