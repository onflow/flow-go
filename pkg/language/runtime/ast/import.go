package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"

// ImportDeclaration

type ImportDeclaration struct {
	Location ImportLocation
	StartPos Position
	EndPos   Position
}

func (v *ImportDeclaration) StartPosition() Position {
	return v.StartPos
}

func (v *ImportDeclaration) EndPosition() Position {
	return v.EndPos
}

func (*ImportDeclaration) isDeclaration() {}

func (*ImportDeclaration) isStatement() {}

func (v *ImportDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitImportDeclaration(v)
}

func (v *ImportDeclaration) DeclarationName() string {
	return ""
}

func (v *ImportDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindImport
}

// ImportLocation

type ImportLocation interface {
	isImportLocation()
}

// StringImportLocation

type StringImportLocation string

func (StringImportLocation) isImportLocation() {}

// AddressImportLocation

type AddressImportLocation string

func (AddressImportLocation) isImportLocation() {}
