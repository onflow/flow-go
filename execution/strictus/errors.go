package strictus

import (
	"bamboo-runtime/execution/strictus/ast"
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
