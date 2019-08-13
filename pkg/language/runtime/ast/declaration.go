package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"

type Declaration interface {
	Element
	isDeclaration()
	DeclarationName() string
	IdentifierPosition() *Position
	DeclarationKind() common.DeclarationKind
}
