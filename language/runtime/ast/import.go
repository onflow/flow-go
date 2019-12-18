package ast

import (
	"encoding/gob"
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
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

func (v *ImportDeclaration) DeclarationIdentifier() *Identifier {
	return nil
}

func (v *ImportDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindImport
}

func (v *ImportDeclaration) DeclarationAccess() Access {
	return AccessNotSpecified
}

// Location describes the origin of a Cadence script.
// This could be a file, a transaction, or a smart contract.
//
type Location interface {
	// ID returns the canonical ID for this import location.
	ID() LocationID
}

func LocationsMatch(first, second Location) bool {
	if first == nil && second == nil {
		return true
	}

	if (first == nil && second != nil) || (first != nil && second == nil) {
		return false
	}

	return first.ID() == second.ID()
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
	return LocationID(fmt.Sprintf("A.%s", l.ToAddress().Hex()))
}

func (l AddressLocation) ToAddress() values.Address {
	return values.BytesToAddress(l)
}

func init() {
	gob.Register(AddressLocation([]byte{}))
}

// HasImportLocation

type HasImportLocation interface {
	ImportLocation() Location
}
