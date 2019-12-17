package ast

import "github.com/dapperlabs/flow-go/language/runtime/common"

type Declaration interface {
	Element
	isDeclaration()
	DeclarationIdentifier() *Identifier
	DeclarationKind() common.DeclarationKind
	DeclarationAccess() Access
}
