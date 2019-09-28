package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

type ValueDeclaration interface {
	ValueDeclarationType() Type
	ValueDeclarationKind() common.DeclarationKind
	ValueDeclarationPosition() ast.Position
	ValueDeclarationIsConstant() bool
	ValueDeclarationArgumentLabels() []string
}

type TypeDeclaration interface {
	TypeDeclarationType() Type
	TypeDeclarationKind() common.DeclarationKind
	TypeDeclarationPosition() ast.Position
}
