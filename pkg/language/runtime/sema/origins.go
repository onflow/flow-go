package sema

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common/intervalst"
)

type Position struct {
	// line number, starting at 1
	Line int
	// column number, starting at 0 (byte count)
	Column int
}

func (pos Position) String() string {
	return fmt.Sprintf("Position{%d, %d}", pos.Line, pos.Column)
}

func (pos Position) CompareTo(other intervalst.Position) int {
	if _, ok := other.(intervalst.MinPosition); ok {
		return 1
	}

	otherL, ok := other.(Position)
	if !ok {
		panic(fmt.Sprintf("not a sema.Position: %#+v", other))
	}
	if pos.Line < otherL.Line {
		return -1
	}
	if pos.Line > otherL.Line {
		return 1
	}
	if pos.Column < otherL.Column {
		return -1
	}
	if pos.Column > otherL.Column {
		return 1
	}
	return 0
}

type Origins struct {
	T *intervalst.IntervalST
}

func NewOrigins() *Origins {
	return &Origins{
		T: &intervalst.IntervalST{},
	}
}

func ToPosition(position ast.Position) Position {
	return Position{
		Line:   position.Line,
		Column: position.Column,
	}
}

func (o *Origins) Put(startPos, endPos ast.Position, variable *Variable) {
	origin := Origin{
		StartPos: ToPosition(startPos),
		EndPos:   ToPosition(endPos),
		Variable: variable,
	}
	interval := intervalst.NewInterval(
		origin.StartPos,
		origin.EndPos,
	)
	o.T.Put(interval, origin)
}

type Origin struct {
	StartPos Position
	EndPos   Position
	Variable *Variable
}

func (o *Origins) All() []Origin {
	values := o.T.Values()
	origins := make([]Origin, len(values))
	for i, value := range values {
		origins[i] = value.(Origin)
	}
	return origins
}

func (o *Origins) Find(pos Position) *Origin {
	interval, value := o.T.Search(pos)
	if interval == nil {
		return nil
	}
	origin := value.(Origin)
	return &origin
}
