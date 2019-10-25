package ast

import (
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/segmentio/fasthash/fnv1"
)

// Position defines a row/column within a Cadence script.
type Position struct {
	// offset, starting at 0
	Offset int
	// line number, starting at 1
	Line int
	// column number, starting at 0 (byte count)
	Column int
}

func (position Position) Shifted(length int) Position {
	return Position{
		Line:   position.Line,
		Column: position.Column + length,
		Offset: position.Offset + length,
	}
}

func (position Position) Hash() (result uint32) {
	result = fnv1.Init32
	result = fnv1.AddUint32(result, uint32(position.Offset))
	result = fnv1.AddUint32(result, uint32(position.Line))
	result = fnv1.AddUint32(result, uint32(position.Column))
	return
}

func (position Position) Compare(other Position) int {
	switch {
	case position.Offset < other.Offset:
		return -1
	case position.Offset > other.Offset:
		return 1
	default:
		return 0
	}
}

func PositionFromToken(token antlr.Token) Position {
	return Position{
		Offset: token.GetStart(),
		Line:   token.GetLine(),
		Column: token.GetColumn(),
	}
}

func PositionRangeFromContext(ctx antlr.ParserRuleContext) (start, end Position) {
	start = PositionFromToken(ctx.GetStart())
	end = PositionFromToken(ctx.GetStop())
	return start, end
}

func EndPosition(startPosition Position, end int) Position {
	length := end - startPosition.Offset
	return startPosition.Shifted(length)
}

// HasPosition

type HasPosition interface {
	StartPosition() Position
	EndPosition() Position
}

// Range

type Range struct {
	StartPos Position
	EndPos   Position
}

func (e *Range) StartPosition() Position {
	return e.StartPos
}

func (e *Range) EndPosition() Position {
	return e.EndPos
}
