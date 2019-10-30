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
	Location    Location
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

// Location describes the origin of a Cadence script.
// This could be a file, a transaction, or a smart contract.
//
type Location interface {
	// ID returns the canonical ID for this import location.
	ID() LocationID
}

// LocationID

type LocationID string

// StringLocation

type StringLocation string

func (l StringLocation) ID() LocationID {
	return LocationID(l)
}

func init() {
	gob.Register(StringLocation(""))
}

// AddressLocation

type AddressLocation []byte

func (l AddressLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l AddressLocation) String() string {
	return hex.EncodeToString([]byte(l))
}

func init() {
	gob.Register(AddressLocation([]byte{}))
}

// FileLocation

type FileLocation string

func (l FileLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l FileLocation) String() string {
	return string(l)
}

type REPLLocation struct{}

func (l REPLLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l REPLLocation) String() string {
	return "REPL"
}

// HasImportLocation

type HasImportLocation interface {
	ImportLocation() Location
}
