package ast

import (
	"encoding/gob"
	"encoding/hex"

	"github.com/dapperlabs/flow-go/language/runtime/common"
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
	LocationPos Position
	Range
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
	// ID returns the canonical ID for this import location.
	ID() LocationID
}

// LocationID

type LocationID string

// StringImportLocation

type StringImportLocation string

func (StringImportLocation) isImportLocation() {}

func (l StringImportLocation) ID() LocationID {
	return LocationID(l)
}

func init() {
	gob.Register(StringImportLocation(""))
}

// AddressImportLocation

type AddressImportLocation []byte

func (AddressImportLocation) isImportLocation() {}

func (l AddressImportLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l AddressImportLocation) String() string {
	return hex.EncodeToString([]byte(l))
}

// TransactionImportLocation

type TransactionImportLocation []byte

func (TransactionImportLocation) isImportLocation() {}

func (l TransactionImportLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l TransactionImportLocation) String() string {
	return hex.EncodeToString([]byte(l))
}

// ScriptImportLocation

type ScriptImportLocation []byte

func (ScriptImportLocation) isImportLocation() {}

func (l ScriptImportLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l ScriptImportLocation) String() string {
	return hex.EncodeToString([]byte(l))
}

func init() {
	gob.Register(AddressImportLocation([]byte{}))
}

// HasImportLocation

type HasImportLocation interface {
	ImportLocation() ImportLocation
}
