package ast

import "github.com/antlr/antlr4/runtime/Go/antlr"

type Position struct {
	// offset, starting at 0
	Offset int
	// line number, starting at 1
	Line int
	// column number, starting at 1 (byte count)
	Column int
}

func PositionFromToken(token antlr.Token) Position {
	return Position{
		Offset: token.GetStart(),
		Line:   token.GetLine(),
		Column: token.GetColumn(),
	}
}
