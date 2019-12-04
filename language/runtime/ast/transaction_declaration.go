package ast

import "github.com/dapperlabs/flow-go/language/runtime/common"

type TransactionDeclaration struct {
	Fields         []*FieldDeclaration
	Prepare        *SpecialFunctionDeclaration
	PreConditions  []*Condition
	PostConditions []*Condition
	Execute        *SpecialFunctionDeclaration
	Range
}

func (d *TransactionDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitTransactionDeclaration(d)
}

func (*TransactionDeclaration) isDeclaration() {}
func (*TransactionDeclaration) isStatement()   {}

func (d *TransactionDeclaration) DeclarationIdentifier() Identifier {
	return Identifier{}
}

func (d *TransactionDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindTransaction
}

func (d *TransactionDeclaration) DeclarationAccess() Access {
	return AccessNotSpecified
}
