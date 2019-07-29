package sema

import (
	"fmt"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
)

// astTypeConversionError

type astTypeConversionError struct {
	invalidASTType ast.Type
}

func (e *astTypeConversionError) Error() string {
	return fmt.Sprintf("cannot convert unsupported AST type: %#+v", e.invalidASTType)
}

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

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind common.DeclarationKind
	Name         string
	StartPos     *ast.Position
	EndPos       *ast.Position
}

func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf("cannot find %s `%s` in this scope", e.ExpectedKind.Name(), e.Name)
}

func (e *NotDeclaredError) SecondaryError() string {
	return "not found in this scope"
}

func (e *NotDeclaredError) StartPosition() *ast.Position {
	return e.StartPos
}

func (e *NotDeclaredError) EndPosition() *ast.Position {
	return e.EndPos
}
