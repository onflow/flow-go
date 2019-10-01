package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/raviqqe/hamt"
)

type PositionEntry struct {
	ast.Position
}

func (e PositionEntry) Equal(other hamt.Entry) bool {
	return e.Position == other.(PositionEntry).Position
}

type Positions struct {
	positions hamt.Set
}

func (p Positions) All() (result []ast.Position) {
	s := p.positions
	for s.Size() != 0 {
		var e hamt.Entry
		e, s = s.FirstRest()
		position := e.(PositionEntry).Position
		result = append(result, position)
	}
	return
}

func (p Positions) Include(position ast.Position) bool {
	return p.positions.Include(PositionEntry{
		Position: position,
	})
}

func (p Positions) Insert(position ast.Position) Positions {
	entry := PositionEntry{position}
	newPositions := p.positions.Insert(entry)
	return Positions{newPositions}
}

func (p Positions) Merge(other Positions) Positions {
	newPositions := p.positions.Merge(other.positions)
	return Positions{newPositions}
}

func (p Positions) Size() int {
	return p.positions.Size()
}
