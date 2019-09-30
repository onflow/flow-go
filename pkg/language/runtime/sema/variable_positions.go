package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/raviqqe/hamt"
)

type VariablePositions struct {
	positions hamt.Map
}

func NewVariablePositions() *VariablePositions {
	return &VariablePositions{
		positions: hamt.NewMap(),
	}
}

// Get returns all positions for the given variable.
//
func (p *VariablePositions) Get(variable *Variable) []ast.Position {
	key := VariableKey{variable: variable}
	existing := p.positions.Find(key)
	var moves []ast.Position
	if existing == nil {
		return moves
	}
	return existing.([]ast.Position)
}

func (p *VariablePositions) Add(variable *Variable, pos ast.Position) {
	moves := p.Get(variable)
	moves = append(moves, pos)
	key := VariableKey{variable: variable}
	p.positions = p.positions.Insert(key, moves)
}

func (p *VariablePositions) Clone() *VariablePositions {
	return &VariablePositions{p.positions}
}

// Merge merges the given positions into these positions.
// The positions for all variables are concatenated.
//
func (p *VariablePositions) Merge(other *VariablePositions) {
	otherPositions := other.positions
	for otherPositions.Size() != 0 {
		var entry hamt.Entry
		var value interface{}
		entry, value, otherPositions = otherPositions.FirstRest()

		variable := entry.(VariableKey).variable

		mergedPositions := append(p.Get(variable), value.([]ast.Position)...)
		p.positions = p.positions.Insert(entry, mergedPositions)
	}
}
