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

package interpreter

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/errors"
	"github.com/onflow/cadence-initial/runtime/sema"
)

// StaticType is a shallow representation of a static type (`sema.Type`)
// which doesn't contain the full information, but only refers
// to composite and interface types by ID.
//
// This allows static types to be efficiently serialized and deserialized,
// for example in the world state.
//
type StaticType interface {
	fmt.Stringer
	isStaticType()
}

// CompositeStaticType

type CompositeStaticType struct {
	Location ast.Location
	TypeID   sema.TypeID
}

func (CompositeStaticType) isStaticType() {}

func (t CompositeStaticType) String() string {
	return fmt.Sprintf(
		"CompositeStaticType(Location: %s, TypeID: %s)",
		t.Location,
		t.TypeID,
	)
}

// InterfaceStaticType

type InterfaceStaticType struct {
	Location ast.Location
	TypeID   sema.TypeID
}

func (InterfaceStaticType) isStaticType() {}

func (t InterfaceStaticType) String() string {
	return fmt.Sprintf(
		"InterfaceStaticType(Location: %s, TypeID: %s)",
		t.Location,
		t.TypeID,
	)
}

// VariableSizedStaticType

type VariableSizedStaticType struct {
	Type StaticType
}

func (VariableSizedStaticType) isStaticType() {}

func (t VariableSizedStaticType) String() string {
	return fmt.Sprintf("[%s]", t.Type)
}

// ConstantSizedStaticType

type ConstantSizedStaticType struct {
	Type StaticType
	Size int64
}

func (ConstantSizedStaticType) isStaticType() {}

func (t ConstantSizedStaticType) String() string {
	return fmt.Sprintf("[%s; %d]", t.Type, t.Size)
}

// DictionaryStaticType

type DictionaryStaticType struct {
	KeyType   StaticType
	ValueType StaticType
}

func (DictionaryStaticType) isStaticType() {}

func (t DictionaryStaticType) String() string {
	return fmt.Sprintf("{%s: %s}", t.KeyType, t.ValueType)
}

// OptionalStaticType

type OptionalStaticType struct {
	Type StaticType
}

func (OptionalStaticType) isStaticType() {}

func (t OptionalStaticType) String() string {
	return fmt.Sprintf("%s?", t.Type)
}

// RestrictedStaticType

type RestrictedStaticType struct {
	Type         StaticType
	Restrictions []InterfaceStaticType
}

func (RestrictedStaticType) isStaticType() {}

func (t RestrictedStaticType) String() string {
	restrictions := make([]string, len(t.Restrictions))

	for i, restriction := range t.Restrictions {
		restrictions[i] = restriction.String()
	}

	return fmt.Sprintf("%s{%s}", t.Type, strings.Join(restrictions, ", "))
}

// ReferenceStaticType

type ReferenceStaticType struct {
	Authorized bool
	Type       StaticType
}

func (ReferenceStaticType) isStaticType() {}

func (t ReferenceStaticType) String() string {
	auth := ""
	if t.Authorized {
		auth = "auth "
	}

	return fmt.Sprintf("%s&%s", auth, t.Type)
}

// Conversion

func ConvertSemaToStaticType(typ sema.Type) StaticType {
	switch t := typ.(type) {
	case *sema.CompositeType:
		return CompositeStaticType{
			Location: t.Location,
			TypeID:   t.ID(),
		}

	case *sema.InterfaceType:
		return convertToInterfaceStaticType(t)

	case *sema.VariableSizedType:
		return VariableSizedStaticType{
			Type: ConvertSemaToStaticType(t.Type),
		}

	case *sema.ConstantSizedType:
		return ConstantSizedStaticType{
			Type: ConvertSemaToStaticType(t.Type),
			Size: t.Size,
		}

	case *sema.DictionaryType:
		return DictionaryStaticType{
			KeyType:   ConvertSemaToStaticType(t.KeyType),
			ValueType: ConvertSemaToStaticType(t.ValueType),
		}

	case *sema.OptionalType:
		return OptionalStaticType{
			Type: ConvertSemaToStaticType(t.Type),
		}

	case *sema.RestrictedType:
		restrictions := make([]InterfaceStaticType, len(t.Restrictions))

		for i, restriction := range t.Restrictions {
			restrictions[i] = convertToInterfaceStaticType(restriction)
		}

		return RestrictedStaticType{
			Type:         ConvertSemaToStaticType(t.Type),
			Restrictions: restrictions,
		}

	case *sema.ReferenceType:
		return convertSemaReferenceToStaticReferenceType(t)

	default:
		primitiveStaticType := ConvertSemaToPrimitiveStaticType(t)
		if primitiveStaticType == PrimitiveStaticTypeUnknown {
			return nil
		}
		return primitiveStaticType
	}
}

func convertSemaReferenceToStaticReferenceType(t *sema.ReferenceType) ReferenceStaticType {
	return ReferenceStaticType{
		Authorized: t.Authorized,
		Type:       ConvertSemaToStaticType(t.Type),
	}
}

func convertToInterfaceStaticType(t *sema.InterfaceType) InterfaceStaticType {
	return InterfaceStaticType{
		Location: t.Location,
		TypeID:   t.ID(),
	}
}

func ConvertStaticToSemaType(
	typ StaticType,
	getInterface func(location ast.Location, id sema.TypeID) *sema.InterfaceType,
	getComposite func(location ast.Location, id sema.TypeID) *sema.CompositeType,
) sema.Type {
	switch t := typ.(type) {
	case CompositeStaticType:
		return getComposite(t.Location, t.TypeID)

	case InterfaceStaticType:
		return getInterface(t.Location, t.TypeID)

	case VariableSizedStaticType:
		return &sema.VariableSizedType{
			Type: ConvertStaticToSemaType(t.Type, getInterface, getComposite),
		}

	case ConstantSizedStaticType:
		return &sema.ConstantSizedType{
			Type: ConvertStaticToSemaType(t.Type, getInterface, getComposite),
			Size: t.Size,
		}

	case DictionaryStaticType:
		return &sema.DictionaryType{
			KeyType:   ConvertStaticToSemaType(t.KeyType, getInterface, getComposite),
			ValueType: ConvertStaticToSemaType(t.ValueType, getInterface, getComposite),
		}

	case OptionalStaticType:
		return &sema.OptionalType{
			Type: ConvertStaticToSemaType(t.Type, getInterface, getComposite),
		}

	case RestrictedStaticType:
		restrictions := make([]*sema.InterfaceType, len(t.Restrictions))

		for i, restriction := range t.Restrictions {
			restrictions[i] = getInterface(restriction.Location, restriction.TypeID)
		}

		return &sema.RestrictedType{
			Type:         ConvertStaticToSemaType(t.Type, getInterface, getComposite),
			Restrictions: restrictions,
		}

	case ReferenceStaticType:
		return &sema.ReferenceType{
			Authorized: t.Authorized,
			Type:       ConvertStaticToSemaType(t.Type, getInterface, getComposite),
		}

	case PrimitiveStaticType:
		return t.SemaType()

	default:
		panic(errors.NewUnreachableError())
	}
}
