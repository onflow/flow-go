package strictus

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
)

// SyntaxError

type SyntaxError struct {
	Line    int
	Column  int
	Message string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("line %d:%d %s", e.Line, e.Column, e.Message)
}

// JuxtaposedUnaryOperatorsError

type JuxtaposedUnaryOperatorsError struct {
	Position *ast.Position
}

func (e *JuxtaposedUnaryOperatorsError) Error() string {
	return "unary operators must not be juxtaposed; parenthesize inner expression"
}
