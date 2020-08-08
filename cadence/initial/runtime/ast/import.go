/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ast

import (
	"encoding/hex"
	"fmt"
	"strings"

	"example.com/cadence-initial/runtime/common"
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

func LocationFromTypeID(typeID string) Location {
	pieces := strings.Split(typeID, ".")

	switch len(pieces) {
	case 3:
		if pieces[0] == AddressPrefix {
			address, err := hex.DecodeString(pieces[1])
			if err != nil {
				return nil
			}

			return AddressLocation(address)
		}

		return nil
	case 2:
		return StringLocation(pieces[0])
	default:
		return nil
	}
}

// LocationID

type LocationID string

// StringLocation

type StringLocation string

func (l StringLocation) ID() LocationID {
	return LocationID(l)
}

// AddressLocation

const AddressPrefix = "A"

type AddressLocation []byte

func (l AddressLocation) String() string {
	return l.ToAddress().String()
}

func (l AddressLocation) ID() LocationID {
	return LocationID(fmt.Sprintf(
		"%s.%s",
		AddressPrefix,
		l.ToAddress().Hex(),
	))
}

func (l AddressLocation) ToAddress() common.Address {
	return common.BytesToAddress(l)
}

// HasImportLocation

type HasImportLocation interface {
	ImportLocation() Location
}
