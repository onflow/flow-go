package ast

import "github.com/antlr/antlr4/runtime/Go/antlr"

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

type HasPosition interface {
	StartPosition() Position
	EndPosition() Position
}
