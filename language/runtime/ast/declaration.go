package ast

import "github.com/dapperlabs/flow-go/language/runtime/common"

type Declaration interface {
	Element
	isDeclaration()
	DeclarationName() string
	DeclarationKind() common.DeclarationKind
}
