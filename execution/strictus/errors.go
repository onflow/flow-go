package strictus

import (
	"bamboo-runtime/execution/strictus/ast"
)

// JuxtaposedUnaryOperatorsError

type JuxtaposedUnaryOperatorsError struct {
	Position ast.Position
}

func (e *JuxtaposedUnaryOperatorsError) Error() string {
	return "unary operators must not be juxtaposed; parenthesize inner expression"
}
