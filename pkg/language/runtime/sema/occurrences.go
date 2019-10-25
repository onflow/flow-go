package sema

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common/intervalst"
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

func (pos Position) Compare(other intervalst.Position) int {
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

type Origin struct {
	Type            Type
	DeclarationKind common.DeclarationKind
	StartPos        *ast.Position
	EndPos          *ast.Position
}

type Occurrences struct {
	T *intervalst.IntervalST
}

func NewOccurrences() *Occurrences {
	return &Occurrences{
		T: &intervalst.IntervalST{},
	}
}

func ToPosition(position ast.Position) Position {
	return Position{
		Line:   position.Line,
		Column: position.Column,
	}
}

func (o *Occurrences) Put(startPos, endPos ast.Position, origin *Origin) {
	occurrence := Occurrence{
		StartPos: ToPosition(startPos),
		EndPos:   ToPosition(endPos),
		Origin:   origin,
	}
	interval := intervalst.NewInterval(
		occurrence.StartPos,
		occurrence.EndPos,
	)
	o.T.Put(interval, occurrence)
}

type Occurrence struct {
	StartPos Position
	EndPos   Position
	Origin   *Origin
}

func (o *Occurrences) All() []Occurrence {
	values := o.T.Values()
	occurrences := make([]Occurrence, len(values))
	for i, value := range values {
		occurrences[i] = value.(Occurrence)
	}
	return occurrences
}

func (o *Occurrences) Find(pos Position) *Occurrence {
	interval, value := o.T.Search(pos)
	if interval == nil {
		return nil
	}
	occurrence := value.(Occurrence)
	return &occurrence
}
