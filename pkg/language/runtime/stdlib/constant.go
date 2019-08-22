package stdlib

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
)

type StandardLibraryValue struct {
	Name       string
	Type       sema.Type
	Kind       common.DeclarationKind
	IsConstant bool
}

func (v StandardLibraryValue) ValueDeclarationName() string {
	return v.Name
}

func (v StandardLibraryValue) ValueDeclarationType() sema.Type {
	return v.Type
}

func (v StandardLibraryValue) ValueDeclarationKind() common.DeclarationKind {
	if v.IsConstant {
		return common.DeclarationKindConstant
	} else {
		return common.DeclarationKindVariable
	}
}

func (StandardLibraryValue) ValueDeclarationPosition() ast.Position {
	return ast.Position{}
}

func (v StandardLibraryValue) ValueDeclarationIsConstant() bool {
	return v.IsConstant
}

func (StandardLibraryValue) ValueDeclarationArgumentLabels() []string {
	return nil
}
