package ast

import "github.com/dapperlabs/flow-go/language/runtime/common"

type TransactionDeclaration struct {
	Fields         []*FieldDeclaration
	Prepare        *SpecialFunctionDeclaration
	PreConditions  []*Condition
	PostConditions []*Condition
	Execute        *Block
	Range
}

func (e *TransactionDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitTransactionDeclaration(e)
}

func (*TransactionDeclaration) isDeclaration() {}
func (*TransactionDeclaration) isStatement()   {}

func (e *TransactionDeclaration) DeclarationName() string {
	return ""
}

func (e *TransactionDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindTransaction
}
