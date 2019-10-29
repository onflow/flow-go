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
	isLocation()
	// ID returns the canonical ID for this import location.
	ID() LocationID
}

// LocationID

type LocationID string

// StringLocation

type StringLocation string

func (StringLocation) isLocation() {}

func (l StringLocation) ID() LocationID {
	return LocationID(l)
}

func init() {
	gob.Register(StringLocation(""))
}

// AddressLocation

type AddressLocation []byte

func (AddressLocation) isLocation() {}

func (l AddressLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l AddressLocation) String() string {
	return hex.EncodeToString([]byte(l))
}

// TransactionLocation

type TransactionLocation []byte

func (TransactionLocation) isLocation() {}

func (l TransactionLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l TransactionLocation) String() string {
	return hex.EncodeToString([]byte(l))
}

// ScriptLocation

type ScriptLocation []byte

func (ScriptLocation) isLocation() {}

func (l ScriptLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l ScriptLocation) String() string {
	return hex.EncodeToString([]byte(l))
}

func init() {
	gob.Register(AddressLocation([]byte{}))
}

// FileLocation

type FileLocation string

func (FileLocation) isLocation() {}

func (l FileLocation) ID() LocationID {
	return LocationID(l.String())
}

func (l FileLocation) String() string {
	return string(l)
}

type REPLLocation struct{}

func (REPLLocation) isLocation() {}

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
