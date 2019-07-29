package sema

import (
	"fmt"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
)

// RedeclarationError

// TODO: show previous declaration

type RedeclarationError struct {
	Name string
	Pos  *ast.Position
}

func (e *RedeclarationError) Error() string {
	return fmt.Sprintf("cannot redeclare already declared identifier: %s", e.Name)
}

func (e *RedeclarationError) StartPosition() *ast.Position {
	return e.Pos
}

func (e *RedeclarationError) EndPosition() *ast.Position {
	return e.Pos
}
