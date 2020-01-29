package parser

import (
	"github.com/antlr/antlr4/runtime/Go/antlr"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

func PositionFromToken(token antlr.Token) ast.Position {
	return ast.Position{
		Offset: token.GetStart(),
		Line:   token.GetLine(),
		Column: token.GetColumn(),
	}
}

func PositionRangeFromContext(ctx antlr.ParserRuleContext) (start, end ast.Position) {
	start = PositionFromToken(ctx.GetStart())
	end = PositionFromToken(ctx.GetStop())
	return start, end
}
