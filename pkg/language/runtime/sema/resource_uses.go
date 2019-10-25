package sema

import (
	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

type ResourceUse struct {
	UseAfterInvalidationReported bool
}

type ResourceUseEntry struct {
	ast.Position
}

func (e ResourceUseEntry) Equal(other hamt.Entry) bool {
	return e.Position == other.(ResourceUseEntry).Position
}

////

type ResourceUses struct {
	positions hamt.Map
}

func (p ResourceUses) AllPositions() (result []ast.Position) {
	s := p.positions
	for s.Size() != 0 {
		var e hamt.Entry
		e, _, s = s.FirstRest()
		position := e.(ResourceUseEntry).Position
		result = append(result, position)
	}
	return
}

func (p ResourceUses) Include(pos ast.Position) bool {
	return p.positions.Include(ResourceUseEntry{pos})
}

func (p ResourceUses) Insert(pos ast.Position) ResourceUses {
	if p.Include(pos) {
		return p
	}
	entry := ResourceUseEntry{pos}
	newPositions := p.positions.Insert(entry, ResourceUse{})
	return ResourceUses{newPositions}
}

func (p ResourceUses) MarkUseAfterInvalidationReported(pos ast.Position) ResourceUses {
	entry := ResourceUseEntry{pos}
	value := p.positions.Find(entry)
	use := value.(ResourceUse)
	use.UseAfterInvalidationReported = true
	newPositions := p.positions.Insert(entry, use)
	return ResourceUses{newPositions}
}

func (p ResourceUses) IsUseAfterInvalidationReported(pos ast.Position) bool {
	entry := ResourceUseEntry{pos}
	value := p.positions.Find(entry)
	use := value.(ResourceUse)
	return use.UseAfterInvalidationReported
}

func (p ResourceUses) Merge(other ResourceUses) ResourceUses {
	newPositions := p.positions.Merge(other.positions)
	return ResourceUses{newPositions}
}

func (p ResourceUses) Size() int {
	return p.positions.Size()
}
