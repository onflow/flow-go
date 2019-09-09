package ast

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
)

// Identifier

type Identifier struct {
	Identifier string
	Pos        Position
}

func (i Identifier) String() string {
	return i.Identifier
}

func (i Identifier) StartPosition() Position {
	return i.Pos
}

func (i Identifier) EndPosition() Position {
	length := len(i.Identifier)
	return i.Pos.Shifted(length - 1)
}

// ImportDeclaration

type ImportDeclaration struct {
	Identifiers []Identifier
	Location    ImportLocation
	StartPos    Position
	EndPos      Position
	LocationPos Position
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

func (l AddressImportLocation) String() string {
	return fmt.Sprintf("%#x", []byte(l))
}

// HasImportLocation

type HasImportLocation interface {
	ImportLocation() ImportLocation
}
