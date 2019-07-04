package parser

import (
	"github.com/dapperlabs/bamboo-node/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/language/runtime/errors"
	"fmt"
)

// ParserError

type ParseError interface {
	error
	ast.HasPosition
	isParseError()
}

// SyntaxError

type SyntaxError struct {
	Pos     *ast.Position
	Message string
}

func (*SyntaxError) isParseError() {}

func (e *SyntaxError) StartPosition() *ast.Position {
	return e.Pos
}

func (e *SyntaxError) EndPosition() *ast.Position {
	return e.Pos
}

func (e *SyntaxError) Error() string {
	return e.Message
}

// JuxtaposedUnaryOperatorsError

type JuxtaposedUnaryOperatorsError struct {
	Pos *ast.Position
}

func (*JuxtaposedUnaryOperatorsError) isParseError() {}

func (e *JuxtaposedUnaryOperatorsError) StartPosition() *ast.Position {
	return e.Pos
}

func (e *JuxtaposedUnaryOperatorsError) EndPosition() *ast.Position {
	return e.Pos
}

func (e *JuxtaposedUnaryOperatorsError) Error() string {
	return "unary operators must not be juxtaposed; parenthesize inner expression"
}

// InvalidIntegerLiteralError

type InvalidIntegerLiteralError struct {
	Literal                   string
	IntegerLiteralKind        IntegerLiteralKind
	InvalidIntegerLiteralKind InvalidIntegerLiteralKind
	StartPos                  *ast.Position
	EndPos                    *ast.Position
}

func (*InvalidIntegerLiteralError) isParseError() {}

func (e *InvalidIntegerLiteralError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *InvalidIntegerLiteralError) EndPosition() *ast.Position {
	return e.EndPos
}

func (e *InvalidIntegerLiteralError) Error() string {
	if e.IntegerLiteralKind == IntegerLiteralKindUnknown {
		return fmt.Sprintf(
			"invalid integer literal `%s`: %s",
			e.Literal,
			e.InvalidIntegerLiteralKind.Description(),
		)
	}

	return fmt.Sprintf(
		"invalid %s integer literal `%s`: %s",
		e.IntegerLiteralKind.Name(),
		e.Literal,
		e.InvalidIntegerLiteralKind.Description(),
	)
}

func (e *InvalidIntegerLiteralError) SecondaryError() string {
	switch e.InvalidIntegerLiteralKind {
	case InvalidIntegerLiteralKindLeadingUnderscore:
		return "remove the leading underscore"
	case InvalidIntegerLiteralKindTrailingUnderscore:
		return "remove the trailing underscore"
	case InvalidIntegerLiteralKindUnknownPrefix:
		return "did you mean `0x` (hexadecimal), `0b` (binary), or `0o` (octal)?"
	}

	panic(&errors.UnreachableError{})
}
