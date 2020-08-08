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

package sema

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/errors"
)

func qualifiedIdentifier(identifier string, containerType Type) string {

	// Gather all identifiers: this, parent, grand-parent, etc.

	identifiers := []string{identifier}

	for containerType != nil {
		switch typedContainerType := containerType.(type) {
		case *InterfaceType:
			identifiers = append(identifiers, typedContainerType.Identifier)
			containerType = typedContainerType.ContainerType
		case *CompositeType:
			identifiers = append(identifiers, typedContainerType.Identifier)
			containerType = typedContainerType.ContainerType
		default:
			panic(errors.NewUnreachableError())
		}
	}

	// Append all identifiers, in reverse order

	var sb strings.Builder

	for i := len(identifiers) - 1; i >= 0; i-- {
		sb.WriteString(identifiers[i])
		if i != 0 {
			sb.WriteRune('.')
		}
	}

	return sb.String()
}

type TypeID string

type Type interface {
	IsType()
	ID() TypeID
	String() string
	QualifiedString() string
	Equal(other Type) bool

	// IsResourceType returns true if the type is itself a resource (a `CompositeType` with resource kind),
	// or it contains a resource type (e.g. for optionals, arrays, dictionaries, etc.)
	IsResourceType() bool

	// IsInvalidType returns true if the type is itself the invalid type (see `InvalidType`),
	// or it contains an invalid type (e.g. for optionals, arrays, dictionaries, etc.)
	IsInvalidType() bool

	TypeAnnotationState() TypeAnnotationState
	ContainsFirstLevelInterfaceType() bool

	// Unify attempts to unify the given type with this type, i.e., resolve type parameters
	// in generic types (see `GenericType`) using the given type parameters.
	//
	// For a generic type, unification assigns a given type with a type parameter.
	//
	// If the type parameter has not been previously unified with a type,
	// through an explicitly provided type argument in an invocation
	// or through a previous unification, the type parameter is assigned the given type.
	//
	// If the type parameter has already been previously unified with a type,
	// the type parameter's unified .
	//
	// The boolean return value indicates if a generic type was encountered during unification.
	// For primitives (e.g. `Int`, `String`, etc.) it would be false, as .
	// For types with nested types (e.g. optionals, arrays, and dictionaries)
	// the result is the successful unification of the inner types.
	//
	// The boolean return value does *not* indicate if unification succeeded or not.
	//
	Unify(
		other Type,
		typeParameters map[*TypeParameter]Type,
		report func(err error),
		outerRange ast.Range,
	) bool

	// Resolve returns a type that is free of generic types (see `GenericType`),
	// i.e. it resolves the type parameters in generic types given the type parameter
	// unifications of `typeParameters`.
	//
	// If resolution fails, it returns `nil`.
	//
	Resolve(typeParameters map[*TypeParameter]Type) Type
}

// ValueIndexableType is a type which can be indexed into using a value
//
type ValueIndexableType interface {
	Type
	isValueIndexableType() bool
	ElementType(isAssignment bool) Type
	IndexingType() Type
}

// MemberAccessibleType is a type which might have members
//
type MemberAccessibleType interface {
	Type
	CanHaveMembers() bool
	GetMember(identifier string, targetRange ast.Range, report func(error)) *Member
}

// ContainedType is a type which might have a container type
//
type ContainedType interface {
	Type
	GetContainerType() Type
}

// ContainerType is a type which might have nested types

type ContainerType interface {
	Type
	NestedTypes() map[string]Type
}

func VisitContainerAndNested(t ContainerType, visit func(ty Type)) {
	visit(t)

	for _, nestedType := range t.NestedTypes() {
		if nestedContainerType, ok := nestedType.(ContainerType); ok {
			VisitContainerAndNested(nestedContainerType, visit)
		} else {
			visit(nestedType)
		}
	}
}

// CompositeKindedType is a type which has a composite kind
//
type CompositeKindedType interface {
	Type
	GetCompositeKind() common.CompositeKind
}

// LocatedType is a type which has a location
type LocatedType interface {
	Type
	GetLocation() ast.Location
}

// TypeAnnotation

type TypeAnnotation struct {
	IsResource bool
	Type       Type
}

func (a *TypeAnnotation) TypeAnnotationState() TypeAnnotationState {
	if a.Type.IsInvalidType() {
		return TypeAnnotationStateValid
	}

	innerState := a.Type.TypeAnnotationState()
	if innerState != TypeAnnotationStateValid {
		return innerState
	}

	isResourceType := a.Type.IsResourceType()
	switch {
	case isResourceType && !a.IsResource:
		return TypeAnnotationStateMissingResourceAnnotation
	case !isResourceType && a.IsResource:
		return TypeAnnotationStateInvalidResourceAnnotation
	default:
		return TypeAnnotationStateValid
	}
}

func (a *TypeAnnotation) String() string {
	if a.IsResource {
		return fmt.Sprintf(
			"%s%s",
			common.CompositeKindResource.Annotation(),
			a.Type,
		)
	} else {
		return fmt.Sprint(a.Type)
	}
}

func (a *TypeAnnotation) QualifiedString() string {
	qualifiedString := a.Type.QualifiedString()
	if a.IsResource {
		return fmt.Sprintf(
			"%s%s",
			common.CompositeKindResource.Annotation(),
			qualifiedString,
		)
	} else {
		return fmt.Sprint(qualifiedString)
	}
}

func (a *TypeAnnotation) Equal(other *TypeAnnotation) bool {
	return a.IsResource == other.IsResource &&
		a.Type.Equal(other.Type)
}

func NewTypeAnnotation(ty Type) *TypeAnnotation {
	return &TypeAnnotation{
		IsResource: ty.IsResourceType(),
		Type:       ty,
	}
}

// toString

const ToStringFunctionName = "toString"

var toStringFunctionType = &FunctionType{
	ReturnTypeAnnotation: NewTypeAnnotation(
		&StringType{},
	),
}

var typeBuiltinFunctionTypes = map[TypeID]map[string]*FunctionType{}

func init() {
	for _, ty := range append(
		AllNumberTypes[:],
		&NumberType{},
		&SignedNumberType{},
		&FixedPointType{},
		&SignedFixedPointType{},
		&IntegerType{},
		&SignedIntegerType{},
		&AddressType{},
	) {
		functionTypes, ok := typeBuiltinFunctionTypes[ty.ID()]
		if !ok {
			functionTypes = map[string]*FunctionType{}
			typeBuiltinFunctionTypes[ty.ID()] = functionTypes
		}
		functionTypes[ToStringFunctionName] = toStringFunctionType
	}
}

// MetaType represents the type of a type.
type MetaType struct{}

func (*MetaType) IsType() {}

func (*MetaType) String() string {
	return "Type"
}

func (*MetaType) QualifiedString() string {
	return "Type"
}

func (*MetaType) ID() TypeID {
	return "Type"
}

func (*MetaType) Equal(other Type) bool {
	_, ok := other.(*MetaType)
	return ok
}

func (*MetaType) IsResourceType() bool {
	return false
}

func (*MetaType) IsInvalidType() bool {
	return false
}

func (*MetaType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*MetaType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*MetaType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *MetaType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*MetaType) CanHaveMembers() bool {
	return true
}

func (t *MetaType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {

	switch identifier {
	case "identifier":
		return NewPublicConstantFieldMember(t, identifier, &StringType{})

	default:
		return nil
	}
}

// AnyType represents the top type of all types.
// NOTE: This type is only used internally and not available in programs.
type AnyType struct{}

func (*AnyType) IsType() {}

func (*AnyType) String() string {
	return "Any"
}

func (*AnyType) QualifiedString() string {
	return "Any"
}

func (*AnyType) ID() TypeID {
	return "Any"
}

func (*AnyType) Equal(other Type) bool {
	_, ok := other.(*AnyType)
	return ok
}

func (*AnyType) IsResourceType() bool {
	return false
}

func (*AnyType) IsInvalidType() bool {
	return false
}

func (*AnyType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*AnyType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*AnyType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *AnyType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// AnyStructType represents the top type of all non-resource types
type AnyStructType struct{}

func (*AnyStructType) IsType() {}

func (*AnyStructType) String() string {
	return "AnyStruct"
}

func (*AnyStructType) QualifiedString() string {
	return "AnyStruct"
}

func (*AnyStructType) ID() TypeID {
	return "AnyStruct"
}

func (*AnyStructType) Equal(other Type) bool {
	_, ok := other.(*AnyStructType)
	return ok
}

func (*AnyStructType) IsResourceType() bool {
	return false
}

func (*AnyStructType) IsInvalidType() bool {
	return false
}

func (*AnyStructType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*AnyStructType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*AnyStructType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *AnyStructType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// AnyResourceType represents the top type of all resource types
type AnyResourceType struct{}

func (*AnyResourceType) IsType() {}

func (*AnyResourceType) String() string {
	return "AnyResource"
}

func (*AnyResourceType) QualifiedString() string {
	return "AnyResource"
}

func (*AnyResourceType) ID() TypeID {
	return "AnyResource"
}

func (*AnyResourceType) Equal(other Type) bool {
	_, ok := other.(*AnyResourceType)
	return ok
}

func (*AnyResourceType) IsResourceType() bool {
	return true
}

func (*AnyResourceType) IsInvalidType() bool {
	return false
}

func (*AnyResourceType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*AnyResourceType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*AnyResourceType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *AnyResourceType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// NeverType represents the bottom type
type NeverType struct{}

func (*NeverType) IsType() {}

func (*NeverType) String() string {
	return "Never"
}

func (*NeverType) QualifiedString() string {
	return "Never"
}

func (*NeverType) ID() TypeID {
	return "Never"
}

func (*NeverType) Equal(other Type) bool {
	_, ok := other.(*NeverType)
	return ok
}

func (*NeverType) IsResourceType() bool {
	return false
}

func (*NeverType) IsInvalidType() bool {
	return false
}

func (*NeverType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*NeverType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*NeverType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *NeverType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// VoidType represents the void type
type VoidType struct{}

func (*VoidType) IsType() {}

func (*VoidType) String() string {
	return "Void"
}

func (*VoidType) QualifiedString() string {
	return "Void"
}

func (*VoidType) ID() TypeID {
	return "Void"
}

func (*VoidType) Equal(other Type) bool {
	_, ok := other.(*VoidType)
	return ok
}

func (*VoidType) IsResourceType() bool {
	return false
}

func (*VoidType) IsInvalidType() bool {
	return false
}

func (*VoidType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*VoidType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*VoidType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *VoidType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// InvalidType represents a type that is invalid.
// It is the result of type checking failing and
// can't be expressed in programs.
//
type InvalidType struct{}

func (*InvalidType) IsType() {}

func (*InvalidType) String() string {
	return "<<invalid>>"
}

func (*InvalidType) QualifiedString() string {
	return "<<invalid>>"
}

func (*InvalidType) ID() TypeID {
	return "<<invalid>>"
}

func (*InvalidType) Equal(other Type) bool {
	_, ok := other.(*InvalidType)
	return ok
}

func (*InvalidType) IsResourceType() bool {
	return false
}

func (*InvalidType) IsInvalidType() bool {
	return true
}

func (*InvalidType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*InvalidType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*InvalidType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *InvalidType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// OptionalType represents the optional variant of another type
type OptionalType struct {
	Type Type
}

func (*OptionalType) IsType() {}

func (t *OptionalType) String() string {
	if t.Type == nil {
		return "optional"
	}
	return fmt.Sprintf("%s?", t.Type)
}

func (t *OptionalType) QualifiedString() string {
	if t.Type == nil {
		return "optional"
	}
	return fmt.Sprintf("%s?", t.Type.QualifiedString())
}

func (t *OptionalType) ID() TypeID {
	var id string
	if t.Type != nil {
		id = string(t.Type.ID())
	}
	return TypeID(fmt.Sprintf("%s?", id))
}

func (t *OptionalType) Equal(other Type) bool {
	otherOptional, ok := other.(*OptionalType)
	if !ok {
		return false
	}
	return t.Type.Equal(otherOptional.Type)
}

func (t *OptionalType) IsResourceType() bool {
	return t.Type.IsResourceType()
}

func (t *OptionalType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

func (t *OptionalType) TypeAnnotationState() TypeAnnotationState {
	return t.Type.TypeAnnotationState()
}

func (t *OptionalType) ContainsFirstLevelInterfaceType() bool {
	return t.Type.ContainsFirstLevelInterfaceType()
}

func (t *OptionalType) Unify(other Type, typeParameters map[*TypeParameter]Type, report func(err error), outerRange ast.Range) bool {
	otherOptional, ok := other.(*OptionalType)
	if !ok {
		return false
	}

	return t.Type.Unify(otherOptional.Type, typeParameters, report, outerRange)
}

func (t *OptionalType) Resolve(typeParameters map[*TypeParameter]Type) Type {

	newInnerType := t.Type.Resolve(typeParameters)
	if newInnerType == nil {
		return nil
	}

	return &OptionalType{
		Type: newInnerType,
	}
}

// GenericType
//
type GenericType struct {
	TypeParameter *TypeParameter
}

func (*GenericType) IsType() {}

func (t *GenericType) String() string {
	return t.TypeParameter.Name
}

func (t *GenericType) QualifiedString() string {
	return t.TypeParameter.Name
}

func (t *GenericType) ID() TypeID {
	return TypeID(t.TypeParameter.Name)
}

func (t *GenericType) Equal(other Type) bool {
	otherType, ok := other.(*GenericType)
	if !ok {
		return false
	}
	return t.TypeParameter == otherType.TypeParameter
}

func (t *GenericType) IsResourceType() bool {
	return false
}

func (t *GenericType) IsInvalidType() bool {
	return false
}

func (t *GenericType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (t *GenericType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (t *GenericType) Unify(
	other Type,
	typeParameters map[*TypeParameter]Type,
	report func(err error),
	outerRange ast.Range,
) bool {

	if unifiedType, ok := typeParameters[t.TypeParameter]; ok {

		// If the type parameter is already unified with a type argument
		// (either explicit by a type argument, or implicit through an argument's type),
		// check that this argument's type matches the unified type

		if !other.Equal(unifiedType) {
			report(
				&TypeParameterTypeMismatchError{
					TypeParameter: t.TypeParameter,
					ExpectedType:  unifiedType,
					ActualType:    other,
					Range:         outerRange,
				},
			)
		}

	} else {
		// If the type parameter is not yet unified to a type argument, unify it.

		typeParameters[t.TypeParameter] = other

		// If the type parameter corresponding to the type argument has a type bound,
		// then check that the argument's type is a subtype of the type bound.

		err := t.TypeParameter.checkTypeBound(other, outerRange)
		if err != nil {
			report(err)
		}
	}

	return true
}

func (t *GenericType) Resolve(typeParameters map[*TypeParameter]Type) Type {
	ty, ok := typeParameters[t.TypeParameter]
	if !ok {
		return nil
	}
	return ty
}

// BoolType represents the boolean type
type BoolType struct{}

func (*BoolType) IsType() {}

func (*BoolType) String() string {
	return "Bool"
}

func (*BoolType) QualifiedString() string {
	return "Bool"
}

func (*BoolType) ID() TypeID {
	return "Bool"
}

func (*BoolType) Equal(other Type) bool {
	_, ok := other.(*BoolType)
	return ok
}

func (*BoolType) IsResourceType() bool {
	return false
}

func (*BoolType) IsInvalidType() bool {
	return false
}

func (*BoolType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*BoolType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*BoolType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *BoolType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// CharacterType represents the character type

type CharacterType struct{}

func (*CharacterType) IsType() {}

func (*CharacterType) String() string {
	return "Character"
}

func (*CharacterType) QualifiedString() string {
	return "Character"
}

func (*CharacterType) ID() TypeID {
	return "Character"
}

func (*CharacterType) Equal(other Type) bool {
	_, ok := other.(*CharacterType)
	return ok
}

func (*CharacterType) IsResourceType() bool {
	return false
}

func (*CharacterType) IsInvalidType() bool {
	return false
}

func (*CharacterType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*CharacterType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*CharacterType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *CharacterType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// StringType represents the string type
type StringType struct{}

func (*StringType) IsType() {}

func (*StringType) String() string {
	return "String"
}

func (*StringType) QualifiedString() string {
	return "String"
}

func (*StringType) ID() TypeID {
	return "String"
}

func (*StringType) Equal(other Type) bool {
	_, ok := other.(*StringType)
	return ok
}

func (*StringType) IsResourceType() bool {
	return false
}

func (*StringType) IsInvalidType() bool {
	return false
}

func (*StringType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*StringType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*StringType) CanHaveMembers() bool {
	return true
}

var stringTypeConcatFunctionType = &FunctionType{
	Parameters: []*Parameter{
		{
			Label:          ArgumentLabelNotRequired,
			Identifier:     "other",
			TypeAnnotation: NewTypeAnnotation(&StringType{}),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(
		&StringType{},
	),
}

var stringTypeSliceFunctionType = &FunctionType{
	Parameters: []*Parameter{
		{
			Identifier:     "from",
			TypeAnnotation: NewTypeAnnotation(&IntType{}),
		},
		{
			Identifier:     "upTo",
			TypeAnnotation: NewTypeAnnotation(&IntType{}),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(
		&StringType{},
	),
}

var stringTypeDecodeHexFunctionType = &FunctionType{
	ReturnTypeAnnotation: NewTypeAnnotation(
		&VariableSizedType{
			// TODO: change to UInt8
			Type: &IntType{},
		},
	),
}

func (t *StringType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	newFunction := func(functionType *FunctionType) *Member {
		return NewPublicFunctionMember(t, identifier, functionType)
	}

	switch identifier {
	case "concat":
		return newFunction(stringTypeConcatFunctionType)

	case "slice":
		return newFunction(stringTypeSliceFunctionType)

	case "decodeHex":
		return newFunction(stringTypeDecodeHexFunctionType)

	case "length":
		return NewPublicConstantFieldMember(t, identifier, &IntType{})

	default:
		return nil
	}
}

func (t *StringType) isValueIndexableType() bool {
	return true
}

func (t *StringType) ElementType(_ bool) Type {
	return &CharacterType{}
}

func (t *StringType) IndexingType() Type {
	return &IntegerType{}
}

func (*StringType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *StringType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// NumberType represents the super-type of all signed number types
type NumberType struct{}

func (*NumberType) IsType() {}

func (*NumberType) String() string {
	return "Number"
}

func (*NumberType) QualifiedString() string {
	return "Number"
}

func (*NumberType) ID() TypeID {
	return "Number"
}

func (*NumberType) Equal(other Type) bool {
	_, ok := other.(*NumberType)
	return ok
}

func (*NumberType) IsResourceType() bool {
	return false
}

func (*NumberType) IsInvalidType() bool {
	return false
}

func (*NumberType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*NumberType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*NumberType) MinInt() *big.Int {
	return nil
}

func (*NumberType) MaxInt() *big.Int {
	return nil
}

func (*NumberType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *NumberType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*NumberType) CanHaveMembers() bool {
	return true
}

func (t *NumberType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// SignedNumberType represents the super-type of all signed number types
type SignedNumberType struct{}

func (*SignedNumberType) IsType() {}

func (*SignedNumberType) String() string {
	return "SignedNumber"
}

func (*SignedNumberType) QualifiedString() string {
	return "SignedNumber"
}

func (*SignedNumberType) ID() TypeID {
	return "SignedNumber"
}

func (*SignedNumberType) Equal(other Type) bool {
	_, ok := other.(*SignedNumberType)
	return ok
}

func (*SignedNumberType) IsResourceType() bool {
	return false
}

func (*SignedNumberType) IsInvalidType() bool {
	return false
}

func (*SignedNumberType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*SignedNumberType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*SignedNumberType) MinInt() *big.Int {
	return nil
}

func (*SignedNumberType) MaxInt() *big.Int {
	return nil
}

func (*SignedNumberType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *SignedNumberType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*SignedNumberType) CanHaveMembers() bool {
	return true
}

func (t *SignedNumberType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// IntegerRangedType

type IntegerRangedType interface {
	Type
	MinInt() *big.Int
	MaxInt() *big.Int
}

type FractionalRangedType interface {
	IntegerRangedType
	Scale() uint
	MinFractional() *big.Int
	MaxFractional() *big.Int
}

// IntegerType represents the super-type of all integer types
type IntegerType struct{}

func (*IntegerType) IsType() {}

func (*IntegerType) String() string {
	return "Integer"
}

func (*IntegerType) QualifiedString() string {
	return "Integer"
}

func (*IntegerType) ID() TypeID {
	return "Integer"
}

func (*IntegerType) Equal(other Type) bool {
	_, ok := other.(*IntegerType)
	return ok
}

func (*IntegerType) IsResourceType() bool {
	return false
}

func (*IntegerType) IsInvalidType() bool {
	return false
}

func (*IntegerType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*IntegerType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*IntegerType) MinInt() *big.Int {
	return nil
}

func (*IntegerType) MaxInt() *big.Int {
	return nil
}

func (*IntegerType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *IntegerType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*IntegerType) CanHaveMembers() bool {
	return true
}

func (t *IntegerType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// SignedIntegerType represents the super-type of all signed integer types
type SignedIntegerType struct{}

func (*SignedIntegerType) IsType() {}

func (*SignedIntegerType) String() string {
	return "SignedInteger"
}

func (*SignedIntegerType) QualifiedString() string {
	return "SignedInteger"
}

func (*SignedIntegerType) ID() TypeID {
	return "SignedInteger"
}

func (*SignedIntegerType) Equal(other Type) bool {
	_, ok := other.(*SignedIntegerType)
	return ok
}

func (*SignedIntegerType) IsResourceType() bool {
	return false
}

func (*SignedIntegerType) IsInvalidType() bool {
	return false
}

func (*SignedIntegerType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*SignedIntegerType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*SignedIntegerType) MinInt() *big.Int {
	return nil
}

func (*SignedIntegerType) MaxInt() *big.Int {
	return nil
}

func (*SignedIntegerType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *SignedIntegerType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*SignedIntegerType) CanHaveMembers() bool {
	return true
}

func (t *SignedIntegerType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// IntType represents the arbitrary-precision integer type `Int`
type IntType struct{}

func (*IntType) IsType() {}

func (*IntType) String() string {
	return "Int"
}

func (*IntType) QualifiedString() string {
	return "Int"
}

func (*IntType) ID() TypeID {
	return "Int"
}

func (*IntType) Equal(other Type) bool {
	_, ok := other.(*IntType)
	return ok
}

func (*IntType) IsResourceType() bool {
	return false
}

func (*IntType) IsInvalidType() bool {
	return false
}

func (*IntType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*IntType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*IntType) MinInt() *big.Int {
	return nil
}

func (*IntType) MaxInt() *big.Int {
	return nil
}

func (*IntType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *IntType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*IntType) CanHaveMembers() bool {
	return true
}

func (t *IntType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Int8Type represents the 8-bit signed integer type `Int8`

type Int8Type struct{}

func (*Int8Type) IsType() {}

func (*Int8Type) String() string {
	return "Int8"
}

func (*Int8Type) QualifiedString() string {
	return "Int8"
}

func (*Int8Type) ID() TypeID {
	return "Int8"
}

func (*Int8Type) Equal(other Type) bool {
	_, ok := other.(*Int8Type)
	return ok
}

func (*Int8Type) IsResourceType() bool {
	return false
}

func (*Int8Type) IsInvalidType() bool {
	return false
}

func (*Int8Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Int8Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Int8TypeMinInt = new(big.Int).SetInt64(math.MinInt8)
var Int8TypeMaxInt = new(big.Int).SetInt64(math.MaxInt8)

func (*Int8Type) MinInt() *big.Int {
	return Int8TypeMinInt
}

func (*Int8Type) MaxInt() *big.Int {
	return Int8TypeMaxInt
}

func (*Int8Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Int8Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Int8Type) CanHaveMembers() bool {
	return true
}

func (t *Int8Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Int16Type represents the 16-bit signed integer type `Int16`
type Int16Type struct{}

func (*Int16Type) IsType() {}

func (*Int16Type) String() string {
	return "Int16"
}

func (*Int16Type) QualifiedString() string {
	return "Int16"
}

func (*Int16Type) ID() TypeID {
	return "Int16"
}

func (*Int16Type) Equal(other Type) bool {
	_, ok := other.(*Int16Type)
	return ok
}

func (*Int16Type) IsResourceType() bool {
	return false
}

func (*Int16Type) IsInvalidType() bool {
	return false
}

func (*Int16Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Int16Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Int16TypeMinInt = new(big.Int).SetInt64(math.MinInt16)
var Int16TypeMaxInt = new(big.Int).SetInt64(math.MaxInt16)

func (*Int16Type) MinInt() *big.Int {
	return Int16TypeMinInt
}

func (*Int16Type) MaxInt() *big.Int {
	return Int16TypeMaxInt
}

func (*Int16Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Int16Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Int16Type) CanHaveMembers() bool {
	return true
}

func (t *Int16Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Int32Type represents the 32-bit signed integer type `Int32`
type Int32Type struct{}

func (*Int32Type) IsType() {}

func (*Int32Type) String() string {
	return "Int32"
}

func (*Int32Type) QualifiedString() string {
	return "Int32"
}

func (*Int32Type) ID() TypeID {
	return "Int32"
}

func (*Int32Type) Equal(other Type) bool {
	_, ok := other.(*Int32Type)
	return ok
}

func (*Int32Type) IsResourceType() bool {
	return false
}

func (*Int32Type) IsInvalidType() bool {
	return false
}

func (*Int32Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Int32Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Int32TypeMinInt = new(big.Int).SetInt64(math.MinInt32)
var Int32TypeMaxInt = new(big.Int).SetInt64(math.MaxInt32)

func (*Int32Type) MinInt() *big.Int {
	return Int32TypeMinInt
}

func (*Int32Type) MaxInt() *big.Int {
	return Int32TypeMaxInt
}

func (*Int32Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Int32Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Int32Type) CanHaveMembers() bool {
	return true
}

func (t *Int32Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Int64Type represents the 64-bit signed integer type `Int64`
type Int64Type struct{}

func (*Int64Type) IsType() {}

func (*Int64Type) String() string {
	return "Int64"
}

func (*Int64Type) QualifiedString() string {
	return "Int64"
}

func (*Int64Type) ID() TypeID {
	return "Int64"
}

func (*Int64Type) Equal(other Type) bool {
	_, ok := other.(*Int64Type)
	return ok
}

func (*Int64Type) IsResourceType() bool {
	return false
}

func (*Int64Type) IsInvalidType() bool {
	return false
}

func (*Int64Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Int64Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Int64TypeMinInt = new(big.Int).SetInt64(math.MinInt64)
var Int64TypeMaxInt = new(big.Int).SetInt64(math.MaxInt64)

func (*Int64Type) MinInt() *big.Int {
	return Int64TypeMinInt
}

func (*Int64Type) MaxInt() *big.Int {
	return Int64TypeMaxInt
}

func (*Int64Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Int64Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Int64Type) CanHaveMembers() bool {
	return true
}

func (t *Int64Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Int128Type represents the 128-bit signed integer type `Int128`
type Int128Type struct{}

func (*Int128Type) IsType() {}

func (*Int128Type) String() string {
	return "Int128"
}

func (*Int128Type) QualifiedString() string {
	return "Int128"
}

func (*Int128Type) ID() TypeID {
	return "Int128"
}

func (*Int128Type) Equal(other Type) bool {
	_, ok := other.(*Int128Type)
	return ok
}

func (*Int128Type) IsResourceType() bool {
	return false
}

func (*Int128Type) IsInvalidType() bool {
	return false
}

func (*Int128Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Int128Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Int128TypeMinIntBig *big.Int

func init() {
	Int128TypeMinIntBig = big.NewInt(-1)
	Int128TypeMinIntBig.Lsh(Int128TypeMinIntBig, 127)
}

var Int128TypeMaxIntBig *big.Int

func init() {
	Int128TypeMaxIntBig = big.NewInt(1)
	Int128TypeMaxIntBig.Lsh(Int128TypeMaxIntBig, 127)
	Int128TypeMaxIntBig.Sub(Int128TypeMaxIntBig, big.NewInt(1))
}

func (*Int128Type) MinInt() *big.Int {
	return Int128TypeMinIntBig
}

func (*Int128Type) MaxInt() *big.Int {
	return Int128TypeMaxIntBig
}

func (*Int128Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Int128Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Int128Type) CanHaveMembers() bool {
	return true
}

func (t *Int128Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Int256Type represents the 256-bit signed integer type `Int256`
type Int256Type struct{}

func (*Int256Type) IsType() {}

func (*Int256Type) String() string {
	return "Int256"
}

func (*Int256Type) QualifiedString() string {
	return "Int256"
}

func (*Int256Type) ID() TypeID {
	return "Int256"
}

func (*Int256Type) Equal(other Type) bool {
	_, ok := other.(*Int256Type)
	return ok
}

func (*Int256Type) IsResourceType() bool {
	return false
}

func (*Int256Type) IsInvalidType() bool {
	return false
}

func (*Int256Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Int256Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Int256TypeMinIntBig *big.Int

func init() {
	Int256TypeMinIntBig = big.NewInt(-1)
	Int256TypeMinIntBig.Lsh(Int256TypeMinIntBig, 255)
}

var Int256TypeMaxIntBig *big.Int

func init() {
	Int256TypeMaxIntBig = big.NewInt(1)
	Int256TypeMaxIntBig.Lsh(Int256TypeMaxIntBig, 255)
	Int256TypeMaxIntBig.Sub(Int256TypeMaxIntBig, big.NewInt(1))
}

func (*Int256Type) MinInt() *big.Int {
	return Int256TypeMinIntBig
}

func (*Int256Type) MaxInt() *big.Int {
	return Int256TypeMaxIntBig
}

func (*Int256Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Int256Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Int256Type) CanHaveMembers() bool {
	return true
}

func (t *Int256Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// UIntType represents the arbitrary-precision unsigned integer type `UInt`
type UIntType struct{}

func (*UIntType) IsType() {}

func (*UIntType) String() string {
	return "UInt"
}

func (*UIntType) QualifiedString() string {
	return "UInt"
}

func (*UIntType) ID() TypeID {
	return "UInt"
}

func (*UIntType) Equal(other Type) bool {
	_, ok := other.(*UIntType)
	return ok
}

func (*UIntType) IsResourceType() bool {
	return false
}

func (*UIntType) IsInvalidType() bool {
	return false
}

func (*UIntType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*UIntType) ContainsFirstLevelInterfaceType() bool {
	return false
}

var UIntTypeMin = new(big.Int)

func (*UIntType) MinInt() *big.Int {
	return UIntTypeMin
}

func (*UIntType) MaxInt() *big.Int {
	return nil
}

func (*UIntType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *UIntType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*UIntType) CanHaveMembers() bool {
	return true
}

func (t *UIntType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// UInt8Type represents the 8-bit unsigned integer type `UInt8`
// which checks for overflow and underflow
type UInt8Type struct{}

func (*UInt8Type) IsType() {}

func (*UInt8Type) String() string {
	return "UInt8"
}

func (*UInt8Type) QualifiedString() string {
	return "UInt8"
}

func (*UInt8Type) ID() TypeID {
	return "UInt8"
}

func (*UInt8Type) Equal(other Type) bool {
	_, ok := other.(*UInt8Type)
	return ok
}

func (*UInt8Type) IsResourceType() bool {
	return false
}

func (*UInt8Type) IsInvalidType() bool {
	return false
}

func (*UInt8Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*UInt8Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var UInt8TypeMinInt = new(big.Int)
var UInt8TypeMaxInt = new(big.Int).SetUint64(math.MaxUint8)

func (*UInt8Type) MinInt() *big.Int {
	return UInt8TypeMinInt
}

func (*UInt8Type) MaxInt() *big.Int {
	return UInt8TypeMaxInt
}

func (*UInt8Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *UInt8Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*UInt8Type) CanHaveMembers() bool {
	return true
}

func (t *UInt8Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// UInt16Type represents the 16-bit unsigned integer type `UInt16`
// which checks for overflow and underflow
type UInt16Type struct{}

func (*UInt16Type) IsType() {}

func (*UInt16Type) String() string {
	return "UInt16"
}

func (*UInt16Type) QualifiedString() string {
	return "UInt16"
}

func (*UInt16Type) ID() TypeID {
	return "UInt16"
}

func (*UInt16Type) Equal(other Type) bool {
	_, ok := other.(*UInt16Type)
	return ok
}

func (*UInt16Type) IsResourceType() bool {
	return false
}

func (*UInt16Type) IsInvalidType() bool {
	return false
}

func (*UInt16Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*UInt16Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var UInt16TypeMinInt = new(big.Int)
var UInt16TypeMaxInt = new(big.Int).SetUint64(math.MaxUint16)

func (*UInt16Type) MinInt() *big.Int {
	return UInt16TypeMinInt
}

func (*UInt16Type) MaxInt() *big.Int {
	return UInt16TypeMaxInt
}

func (*UInt16Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *UInt16Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*UInt16Type) CanHaveMembers() bool {
	return true
}

func (t *UInt16Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// UInt32Type represents the 32-bit unsigned integer type `UInt32`
// which checks for overflow and underflow
type UInt32Type struct{}

func (*UInt32Type) IsType() {}

func (*UInt32Type) String() string {
	return "UInt32"
}

func (*UInt32Type) QualifiedString() string {
	return "UInt32"
}

func (*UInt32Type) ID() TypeID {
	return "UInt32"
}

func (*UInt32Type) Equal(other Type) bool {
	_, ok := other.(*UInt32Type)
	return ok
}

func (*UInt32Type) IsResourceType() bool {
	return false
}

func (*UInt32Type) IsInvalidType() bool {
	return false
}

func (*UInt32Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*UInt32Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var UInt32TypeMinInt = new(big.Int)
var UInt32TypeMaxInt = new(big.Int).SetUint64(math.MaxUint32)

func (*UInt32Type) MinInt() *big.Int {
	return UInt32TypeMinInt
}

func (*UInt32Type) MaxInt() *big.Int {
	return UInt32TypeMaxInt
}

func (*UInt32Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *UInt32Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*UInt32Type) CanHaveMembers() bool {
	return true
}

func (t *UInt32Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// UInt64Type represents the 64-bit unsigned integer type `UInt64`
// which checks for overflow and underflow
type UInt64Type struct{}

func (*UInt64Type) IsType() {}

func (*UInt64Type) String() string {
	return "UInt64"
}

func (*UInt64Type) QualifiedString() string {
	return "UInt64"
}

func (*UInt64Type) ID() TypeID {
	return "UInt64"
}

func (*UInt64Type) Equal(other Type) bool {
	_, ok := other.(*UInt64Type)
	return ok
}

func (*UInt64Type) IsResourceType() bool {
	return false
}

func (*UInt64Type) IsInvalidType() bool {
	return false
}

func (*UInt64Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*UInt64Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var UInt64TypeMinInt = new(big.Int)
var UInt64TypeMaxInt = new(big.Int).SetUint64(math.MaxUint64)

func (*UInt64Type) MinInt() *big.Int {
	return UInt64TypeMinInt
}

func (*UInt64Type) MaxInt() *big.Int {
	return UInt64TypeMaxInt
}

func (*UInt64Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *UInt64Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*UInt64Type) CanHaveMembers() bool {
	return true
}

func (t *UInt64Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// UInt128Type represents the 128-bit unsigned integer type `UInt128`
// which checks for overflow and underflow
type UInt128Type struct{}

func (*UInt128Type) IsType() {}

func (*UInt128Type) String() string {
	return "UInt128"
}

func (*UInt128Type) QualifiedString() string {
	return "UInt128"
}

func (*UInt128Type) ID() TypeID {
	return "UInt128"
}

func (*UInt128Type) Equal(other Type) bool {
	_, ok := other.(*UInt128Type)
	return ok
}

func (*UInt128Type) IsResourceType() bool {
	return false
}

func (*UInt128Type) IsInvalidType() bool {
	return false
}

func (*UInt128Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*UInt128Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var UInt128TypeMinIntBig = new(big.Int)
var UInt128TypeMaxIntBig *big.Int

func init() {
	UInt128TypeMaxIntBig = big.NewInt(1)
	UInt128TypeMaxIntBig.Lsh(UInt128TypeMaxIntBig, 128)
	UInt128TypeMaxIntBig.Sub(UInt128TypeMaxIntBig, big.NewInt(1))
}

func (*UInt128Type) MinInt() *big.Int {
	return UInt128TypeMinIntBig
}

func (*UInt128Type) MaxInt() *big.Int {
	return UInt128TypeMaxIntBig
}

func (*UInt128Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *UInt128Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*UInt128Type) CanHaveMembers() bool {
	return true
}

func (t *UInt128Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// UInt256Type represents the 256-bit unsigned integer type `UInt256`
// which checks for overflow and underflow
type UInt256Type struct{}

func (*UInt256Type) IsType() {}

func (*UInt256Type) String() string {
	return "UInt256"
}

func (*UInt256Type) QualifiedString() string {
	return "UInt256"
}

func (*UInt256Type) ID() TypeID {
	return "UInt256"
}

func (*UInt256Type) Equal(other Type) bool {
	_, ok := other.(*UInt256Type)
	return ok
}

func (*UInt256Type) IsResourceType() bool {
	return false
}

func (*UInt256Type) IsInvalidType() bool {
	return false
}

func (*UInt256Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*UInt256Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var UInt256TypeMinIntBig = new(big.Int)
var UInt256TypeMaxIntBig *big.Int

func init() {
	UInt256TypeMaxIntBig = big.NewInt(1)
	UInt256TypeMaxIntBig.Lsh(UInt256TypeMaxIntBig, 256)
	UInt256TypeMaxIntBig.Sub(UInt256TypeMaxIntBig, big.NewInt(1))
}

func (*UInt256Type) MinInt() *big.Int {
	return UInt256TypeMinIntBig
}

func (*UInt256Type) MaxInt() *big.Int {
	return UInt256TypeMaxIntBig
}

func (*UInt256Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *UInt256Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*UInt256Type) CanHaveMembers() bool {
	return true
}

func (t *UInt256Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Word8Type represents the 8-bit unsigned integer type `Word8`
// which does NOT check for overflow and underflow
type Word8Type struct{}

func (*Word8Type) IsType() {}

func (*Word8Type) String() string {
	return "Word8"
}

func (*Word8Type) QualifiedString() string {
	return "Word8"
}

func (*Word8Type) ID() TypeID {
	return "Word8"
}

func (*Word8Type) Equal(other Type) bool {
	_, ok := other.(*Word8Type)
	return ok
}

func (*Word8Type) IsResourceType() bool {
	return false
}

func (*Word8Type) IsInvalidType() bool {
	return false
}

func (*Word8Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Word8Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Word8TypeMinInt = new(big.Int)
var Word8TypeMaxInt = new(big.Int).SetUint64(math.MaxUint8)

func (*Word8Type) MinInt() *big.Int {
	return Word8TypeMinInt
}

func (*Word8Type) MaxInt() *big.Int {
	return Word8TypeMaxInt
}

func (*Word8Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Word8Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Word8Type) CanHaveMembers() bool {
	return true
}

func (t *Word8Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Word16Type represents the 16-bit unsigned integer type `Word16`
// which does NOT check for overflow and underflow
type Word16Type struct{}

func (*Word16Type) IsType() {}

func (*Word16Type) String() string {
	return "Word16"
}

func (*Word16Type) QualifiedString() string {
	return "Word16"
}

func (*Word16Type) ID() TypeID {
	return "Word16"
}

func (*Word16Type) Equal(other Type) bool {
	_, ok := other.(*Word16Type)
	return ok
}

func (*Word16Type) IsResourceType() bool {
	return false
}

func (*Word16Type) IsInvalidType() bool {
	return false
}

func (*Word16Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Word16Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Word16TypeMinInt = new(big.Int)
var Word16TypeMaxInt = new(big.Int).SetUint64(math.MaxUint16)

func (*Word16Type) MinInt() *big.Int {
	return Word16TypeMinInt
}

func (*Word16Type) MaxInt() *big.Int {
	return Word16TypeMaxInt
}

func (*Word16Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Word16Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Word16Type) CanHaveMembers() bool {
	return true
}

func (t *Word16Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Word32Type represents the 32-bit unsigned integer type `Word32`
// which does NOT check for overflow and underflow
type Word32Type struct{}

func (*Word32Type) IsType() {}

func (*Word32Type) String() string {
	return "Word32"
}

func (*Word32Type) QualifiedString() string {
	return "Word32"
}

func (*Word32Type) ID() TypeID {
	return "Word32"
}

func (*Word32Type) Equal(other Type) bool {
	_, ok := other.(*Word32Type)
	return ok
}

func (*Word32Type) IsResourceType() bool {
	return false
}

func (*Word32Type) IsInvalidType() bool {
	return false
}

func (*Word32Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Word32Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Word32TypeMinInt = new(big.Int)
var Word32TypeMaxInt = new(big.Int).SetUint64(math.MaxUint32)

func (*Word32Type) MinInt() *big.Int {
	return Word32TypeMinInt
}

func (*Word32Type) MaxInt() *big.Int {
	return Word32TypeMaxInt
}

func (*Word32Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Word32Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Word32Type) CanHaveMembers() bool {
	return true
}

func (t *Word32Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// Word64Type represents the 64-bit unsigned integer type `Word64`
// which does NOT check for overflow and underflow
type Word64Type struct{}

func (*Word64Type) IsType() {}

func (*Word64Type) String() string {
	return "Word64"
}

func (*Word64Type) QualifiedString() string {
	return "Word64"
}

func (*Word64Type) ID() TypeID {
	return "Word64"
}

func (*Word64Type) Equal(other Type) bool {
	_, ok := other.(*Word64Type)
	return ok
}

func (*Word64Type) IsResourceType() bool {
	return false
}

func (*Word64Type) IsInvalidType() bool {
	return false
}

func (*Word64Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Word64Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

var Word64TypeMinInt = new(big.Int)
var Word64TypeMaxInt = new(big.Int).SetUint64(math.MaxUint64)

func (*Word64Type) MinInt() *big.Int {
	return Word64TypeMinInt
}

func (*Word64Type) MaxInt() *big.Int {
	return Word64TypeMaxInt
}

func (*Word64Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Word64Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Word64Type) CanHaveMembers() bool {
	return true
}

func (t *Word64Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// FixedPointType represents the super-type of all fixed-point types
type FixedPointType struct{}

func (*FixedPointType) IsType() {}

func (*FixedPointType) String() string {
	return "FixedPoint"
}

func (*FixedPointType) QualifiedString() string {
	return "FixedPoint"
}

func (*FixedPointType) ID() TypeID {
	return "FixedPoint"
}

func (*FixedPointType) Equal(other Type) bool {
	_, ok := other.(*FixedPointType)
	return ok
}

func (*FixedPointType) IsResourceType() bool {
	return false
}

func (*FixedPointType) IsInvalidType() bool {
	return false
}

func (*FixedPointType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*FixedPointType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*FixedPointType) MinInt() *big.Int {
	return nil
}

func (*FixedPointType) MaxInt() *big.Int {
	return nil
}

func (*FixedPointType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *FixedPointType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*FixedPointType) CanHaveMembers() bool {
	return true
}

func (t *FixedPointType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// SignedFixedPointType represents the super-type of all signed fixed-point types
type SignedFixedPointType struct{}

func (*SignedFixedPointType) IsType() {}

func (*SignedFixedPointType) String() string {
	return "SignedFixedPoint"
}

func (*SignedFixedPointType) QualifiedString() string {
	return "SignedFixedPoint"
}

func (*SignedFixedPointType) ID() TypeID {
	return "SignedFixedPoint"
}

func (*SignedFixedPointType) Equal(other Type) bool {
	_, ok := other.(*SignedFixedPointType)
	return ok
}

func (*SignedFixedPointType) IsResourceType() bool {
	return false
}

func (*SignedFixedPointType) IsInvalidType() bool {
	return false
}

func (*SignedFixedPointType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*SignedFixedPointType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*SignedFixedPointType) MinInt() *big.Int {
	return nil
}

func (*SignedFixedPointType) MaxInt() *big.Int {
	return nil
}

func (*SignedFixedPointType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *SignedFixedPointType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*SignedFixedPointType) CanHaveMembers() bool {
	return true
}

func (t *SignedFixedPointType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

const Fix64Scale uint = 8
const Fix64Factor = 100_000_000

// Fix64Type represents the 64-bit signed decimal fixed-point type `Fix64`
// which has a scale of Fix64Scale, and checks for overflow and underflow
type Fix64Type struct{}

func (*Fix64Type) IsType() {}

func (*Fix64Type) String() string {
	return "Fix64"
}

func (*Fix64Type) QualifiedString() string {
	return "Fix64"
}

func (*Fix64Type) ID() TypeID {
	return "Fix64"
}

func (*Fix64Type) Equal(other Type) bool {
	_, ok := other.(*Fix64Type)
	return ok
}

func (*Fix64Type) IsResourceType() bool {
	return false
}

func (*Fix64Type) IsInvalidType() bool {
	return false
}

func (*Fix64Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*Fix64Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

const Fix64TypeMinInt = math.MinInt64 / Fix64Factor
const Fix64TypeMaxInt = math.MaxInt64 / Fix64Factor

var Fix64TypeMinIntBig = new(big.Int).SetInt64(Fix64TypeMinInt)
var Fix64TypeMaxIntBig = new(big.Int).SetInt64(Fix64TypeMaxInt)

const Fix64TypeMinFractional = math.MinInt64 % Fix64Factor
const Fix64TypeMaxFractional = math.MaxInt64 % Fix64Factor

var Fix64TypeMinFractionalBig = new(big.Int).SetInt64(Fix64TypeMinFractional)
var Fix64TypeMaxFractionalBig = new(big.Int).SetInt64(Fix64TypeMaxFractional)

func init() {
	Fix64TypeMinFractionalBig.Abs(Fix64TypeMinFractionalBig)
}

func (*Fix64Type) MinInt() *big.Int {
	return Fix64TypeMinIntBig
}

func (*Fix64Type) MaxInt() *big.Int {
	return Fix64TypeMaxIntBig
}

func (*Fix64Type) Scale() uint {
	return Fix64Scale
}

func (*Fix64Type) MinFractional() *big.Int {
	return Fix64TypeMinFractionalBig
}

func (*Fix64Type) MaxFractional() *big.Int {
	return Fix64TypeMaxFractionalBig
}

func (*Fix64Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *Fix64Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*Fix64Type) CanHaveMembers() bool {
	return true
}

func (t *Fix64Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// UFix64Type represents the 64-bit unsigned decimal fixed-point type `UFix64`
// which has a scale of 1E9, and checks for overflow and underflow
type UFix64Type struct{}

func (*UFix64Type) IsType() {}

func (*UFix64Type) String() string {
	return "UFix64"
}

func (*UFix64Type) QualifiedString() string {
	return "UFix64"
}

func (*UFix64Type) ID() TypeID {
	return "UFix64"
}

func (*UFix64Type) Equal(other Type) bool {
	_, ok := other.(*UFix64Type)
	return ok
}

func (*UFix64Type) IsResourceType() bool {
	return false
}

func (*UFix64Type) IsInvalidType() bool {
	return false
}

func (*UFix64Type) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*UFix64Type) ContainsFirstLevelInterfaceType() bool {
	return false
}

const UFix64TypeMinInt = 0
const UFix64TypeMaxInt = math.MaxUint64 / uint64(Fix64Factor)

var UFix64TypeMinIntBig = new(big.Int).SetUint64(UFix64TypeMinInt)
var UFix64TypeMaxIntBig = new(big.Int).SetUint64(UFix64TypeMaxInt)

const UFix64TypeMinFractional = 0
const UFix64TypeMaxFractional = math.MaxUint64 % uint64(Fix64Factor)

var UFix64TypeMinFractionalBig = new(big.Int).SetUint64(UFix64TypeMinFractional)
var UFix64TypeMaxFractionalBig = new(big.Int).SetUint64(UFix64TypeMaxFractional)

func (*UFix64Type) MinInt() *big.Int {
	return UFix64TypeMinIntBig
}

func (*UFix64Type) MaxInt() *big.Int {
	return UFix64TypeMaxIntBig
}

func (*UFix64Type) Scale() uint {
	return Fix64Scale
}

func (*UFix64Type) MinFractional() *big.Int {
	return UFix64TypeMinFractionalBig
}

func (*UFix64Type) MaxFractional() *big.Int {
	return UFix64TypeMaxFractionalBig
}

func (*UFix64Type) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *UFix64Type) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*UFix64Type) CanHaveMembers() bool {
	return true
}

func (t *UFix64Type) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// ArrayType

type ArrayType interface {
	ValueIndexableType
	isArrayType()
}

func getArrayMember(arrayType ArrayType, field string, targetRange ast.Range, report func(error)) *Member {
	newFunction := func(functionType *FunctionType) *Member {
		return NewPublicFunctionMember(arrayType, field, functionType)
	}

	switch field {
	case "append":
		// Appending elements to a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)
		return newFunction(
			&FunctionType{
				Parameters: []*Parameter{
					{
						Label:          ArgumentLabelNotRequired,
						Identifier:     "element",
						TypeAnnotation: NewTypeAnnotation(elementType),
					},
				},
				ReturnTypeAnnotation: NewTypeAnnotation(
					&VoidType{},
				),
			},
		)

	case "concat":
		// TODO: maybe allow constant sized:
		//    concatenate with variable sized and return variable sized

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		// TODO: maybe allow for resource element type

		elementType := arrayType.ElementType(false)

		if elementType.IsResourceType() {
			report(
				&InvalidResourceArrayMemberError{
					Name:            field,
					DeclarationKind: common.DeclarationKindFunction,
					Range:           targetRange,
				},
			)
		}

		typeAnnotation := NewTypeAnnotation(arrayType)

		return newFunction(
			&FunctionType{
				Parameters: []*Parameter{
					{
						Label:          ArgumentLabelNotRequired,
						Identifier:     "other",
						TypeAnnotation: typeAnnotation,
					},
				},
				ReturnTypeAnnotation: typeAnnotation,
			},
		)

	case "insert":
		// Inserting elements into to a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)

		return newFunction(
			&FunctionType{
				Parameters: []*Parameter{
					{
						Identifier:     "at",
						TypeAnnotation: NewTypeAnnotation(&IntegerType{}),
					},
					{
						Label:          ArgumentLabelNotRequired,
						Identifier:     "element",
						TypeAnnotation: NewTypeAnnotation(elementType),
					},
				},
				ReturnTypeAnnotation: NewTypeAnnotation(
					&VoidType{},
				),
			},
		)

	case "remove":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)

		return newFunction(
			&FunctionType{
				Parameters: []*Parameter{
					{
						Identifier:     "at",
						TypeAnnotation: NewTypeAnnotation(&IntegerType{}),
					},
				},
				ReturnTypeAnnotation: NewTypeAnnotation(
					elementType,
				),
			},
		)

	case "removeFirst":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)

		return newFunction(
			&FunctionType{
				ReturnTypeAnnotation: NewTypeAnnotation(
					elementType,
				),
			},
		)

	case "removeLast":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)

		return newFunction(
			&FunctionType{
				ReturnTypeAnnotation: NewTypeAnnotation(
					elementType,
				),
			},
		)

	case "contains":
		elementType := arrayType.ElementType(false)

		// It impossible for an array of resources to have a `contains` function:
		// if the resource is passed as an argument, it cannot be inside the array

		if elementType.IsResourceType() {
			report(
				&InvalidResourceArrayMemberError{
					Name:            field,
					DeclarationKind: common.DeclarationKindFunction,
					Range:           targetRange,
				},
			)
		}

		// TODO: implement Equatable interface: https://github.com/dapperlabs/bamboo-node/issues/78

		if !IsEquatableType(elementType) {
			report(
				&NotEquatableTypeError{
					Type:  elementType,
					Range: targetRange,
				},
			)
		}

		return newFunction(
			&FunctionType{
				Parameters: []*Parameter{
					{
						Label:          ArgumentLabelNotRequired,
						Identifier:     "element",
						TypeAnnotation: NewTypeAnnotation(elementType),
					},
				},
				ReturnTypeAnnotation: NewTypeAnnotation(
					&BoolType{},
				),
			},
		)

	case "length":
		return NewPublicConstantFieldMember(
			arrayType,
			field,
			&IntType{},
		)

	default:
		return nil
	}
}

// VariableSizedType is a variable sized array type
type VariableSizedType struct {
	Type Type
}

func (*VariableSizedType) IsType() {}

func (*VariableSizedType) isArrayType() {}

func (t *VariableSizedType) String() string {
	return fmt.Sprintf("[%s]", t.Type)
}

func (t *VariableSizedType) QualifiedString() string {
	return fmt.Sprintf("[%s]", t.Type.QualifiedString())
}

func (t *VariableSizedType) ID() TypeID {
	return TypeID(fmt.Sprintf("[%s]", t.Type.ID()))
}

func (t *VariableSizedType) Equal(other Type) bool {
	otherArray, ok := other.(*VariableSizedType)
	if !ok {
		return false
	}

	return t.Type.Equal(otherArray.Type)
}

func (t *VariableSizedType) CanHaveMembers() bool {
	return true
}

func (t *VariableSizedType) GetMember(identifier string, targetRange ast.Range, report func(error)) *Member {
	return getArrayMember(t, identifier, targetRange, report)
}

func (t *VariableSizedType) IsResourceType() bool {
	return t.Type.IsResourceType()
}

func (t *VariableSizedType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

func (t *VariableSizedType) TypeAnnotationState() TypeAnnotationState {
	return t.Type.TypeAnnotationState()
}

func (t *VariableSizedType) ContainsFirstLevelInterfaceType() bool {
	return t.Type.ContainsFirstLevelInterfaceType()
}

func (t *VariableSizedType) isValueIndexableType() bool {
	return true
}

func (t *VariableSizedType) ElementType(_ bool) Type {
	return t.Type
}

func (t *VariableSizedType) IndexingType() Type {
	return &IntegerType{}
}

func (t *VariableSizedType) Unify(
	other Type,
	typeParameters map[*TypeParameter]Type,
	report func(err error),
	outerRange ast.Range,
) bool {
	otherArray, ok := other.(*VariableSizedType)
	if !ok {
		return false
	}

	return t.Type.Unify(otherArray.Type, typeParameters, report, outerRange)
}

func (t *VariableSizedType) Resolve(typeParameters map[*TypeParameter]Type) Type {
	newInnerType := t.Type.Resolve(typeParameters)
	if newInnerType == nil {
		return nil
	}

	return &VariableSizedType{
		Type: newInnerType,
	}
}

// ConstantSizedType is a constant sized array type
type ConstantSizedType struct {
	Type Type
	Size int64
}

func (*ConstantSizedType) IsType() {}

func (*ConstantSizedType) isArrayType() {}

func (t *ConstantSizedType) String() string {
	return fmt.Sprintf("[%s; %d]", t.Type, t.Size)
}

func (t *ConstantSizedType) QualifiedString() string {
	return fmt.Sprintf("[%s; %d]", t.Type.QualifiedString(), t.Size)
}

func (t *ConstantSizedType) ID() TypeID {
	return TypeID(fmt.Sprintf("[%s;%d]", t.Type.ID(), t.Size))
}

func (t *ConstantSizedType) Equal(other Type) bool {
	otherArray, ok := other.(*ConstantSizedType)
	if !ok {
		return false
	}

	return t.Type.Equal(otherArray.Type) &&
		t.Size == otherArray.Size
}

func (t *ConstantSizedType) CanHaveMembers() bool {
	return true
}

func (t *ConstantSizedType) GetMember(identifier string, targetRange ast.Range, report func(error)) *Member {
	return getArrayMember(t, identifier, targetRange, report)
}

func (t *ConstantSizedType) IsResourceType() bool {
	return t.Type.IsResourceType()
}

func (t *ConstantSizedType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

func (t *ConstantSizedType) TypeAnnotationState() TypeAnnotationState {
	return t.Type.TypeAnnotationState()
}

func (t *ConstantSizedType) ContainsFirstLevelInterfaceType() bool {
	return t.Type.ContainsFirstLevelInterfaceType()
}

func (t *ConstantSizedType) isValueIndexableType() bool {
	return true
}

func (t *ConstantSizedType) ElementType(_ bool) Type {
	return t.Type
}

func (t *ConstantSizedType) IndexingType() Type {
	return &IntegerType{}
}

func (t *ConstantSizedType) Unify(
	other Type,
	typeParameters map[*TypeParameter]Type,
	report func(err error),
	outerRange ast.Range,
) bool {
	otherArray, ok := other.(*ConstantSizedType)
	if !ok {
		return false
	}

	if t.Size != otherArray.Size {
		return false
	}

	return t.Type.Unify(otherArray.Type, typeParameters, report, outerRange)
}

func (t *ConstantSizedType) Resolve(typeParameters map[*TypeParameter]Type) Type {
	newInnerType := t.Type.Resolve(typeParameters)
	if newInnerType == nil {
		return nil
	}

	return &ConstantSizedType{
		Type: newInnerType,
		Size: t.Size,
	}
}

// InvokableType

type InvokableType interface {
	Type
	InvocationFunctionType() *FunctionType
	CheckArgumentExpressions(checker *Checker, argumentExpressions []ast.Expression)
	ArgumentLabels() []string
}

// Parameter

func formatParameter(spaces bool, label, identifier, typeAnnotation string) string {
	var builder strings.Builder

	if label != "" {
		builder.WriteString(label)
		if spaces {
			builder.WriteRune(' ')
		}
	}

	if identifier != "" {
		builder.WriteString(identifier)
		builder.WriteRune(':')
		if spaces {
			builder.WriteRune(' ')
		}
	}

	builder.WriteString(typeAnnotation)

	return builder.String()
}

type Parameter struct {
	Label          string
	Identifier     string
	TypeAnnotation *TypeAnnotation
}

func (p *Parameter) String() string {
	return formatParameter(
		true,
		p.Label,
		p.Identifier,
		p.TypeAnnotation.String(),
	)
}

func (p *Parameter) QualifiedString() string {
	return formatParameter(
		true,
		p.Label,
		p.Identifier,
		p.TypeAnnotation.QualifiedString(),
	)
}

// EffectiveArgumentLabel returns the effective argument label that
// an argument in a call must use:
// If no argument label is declared for parameter,
// the parameter name is used as the argument label
//
func (p *Parameter) EffectiveArgumentLabel() string {
	if p.Label != "" {
		return p.Label
	}
	return p.Identifier
}

// TypeParameter

type TypeParameter struct {
	Name      string
	TypeBound Type
}

func (p TypeParameter) string(typeFormatter func(Type) string) string {
	var builder strings.Builder
	builder.WriteString(p.Name)
	if p.TypeBound != nil {
		builder.WriteString(": ")
		builder.WriteString(typeFormatter(p.TypeBound))
	}
	return builder.String()
}

func (p TypeParameter) String() string {
	return p.string(func(t Type) string {
		return t.String()
	})
}

func (p TypeParameter) QualifiedString() string {
	return p.string(func(t Type) string {
		return t.QualifiedString()
	})
}

func (p TypeParameter) Equal(other *TypeParameter) bool {
	return p.Name == other.Name &&
		(p.TypeBound == nil || !p.TypeBound.Equal(other.TypeBound))
}

func (p TypeParameter) checkTypeBound(ty Type, typeRange ast.Range) error {
	if p.TypeBound == nil ||
		p.TypeBound.IsInvalidType() {

		return nil
	}

	if !IsSubType(ty, p.TypeBound) {
		return &TypeMismatchError{
			ExpectedType: p.TypeBound,
			ActualType:   ty,
			Range:        typeRange,
		}
	}

	return nil
}

// Function types

func formatFunctionType(
	spaces bool,
	typeParameters []string,
	parameters []string,
	returnTypeAnnotation string,
) string {

	var builder strings.Builder
	builder.WriteRune('(')
	if len(typeParameters) > 0 {
		builder.WriteRune('<')
		for i, typeParameter := range typeParameters {
			if i > 0 {
				builder.WriteRune(',')
				if spaces {
					builder.WriteRune(' ')
				}
			}
			builder.WriteString(typeParameter)
		}
		builder.WriteRune('>')
	}
	builder.WriteRune('(')
	for i, parameter := range parameters {
		if i > 0 {
			builder.WriteRune(',')
			if spaces {
				builder.WriteRune(' ')
			}
		}
		builder.WriteString(parameter)
	}
	builder.WriteString("):")
	if spaces {
		builder.WriteRune(' ')
	}
	builder.WriteString(returnTypeAnnotation)
	builder.WriteRune(')')
	return builder.String()
}

// FunctionType is a monomorphic function type.
//
type FunctionType struct {
	TypeParameters        []*TypeParameter
	Parameters            []*Parameter
	ReturnTypeAnnotation  *TypeAnnotation
	RequiredArgumentCount *int
}

func (*FunctionType) IsType() {}

func (t *FunctionType) InvocationFunctionType() *FunctionType {
	return t
}

func (*FunctionType) CheckArgumentExpressions(_ *Checker, _ []ast.Expression) {
	// NO-OP: no checks for normal functions
}

func (t *FunctionType) String() string {

	typeParameters := make([]string, len(t.TypeParameters))

	for i, typeParameter := range t.TypeParameters {
		typeParameters[i] = typeParameter.String()
	}

	parameters := make([]string, len(t.Parameters))

	for i, parameter := range t.Parameters {
		parameters[i] = parameter.String()
	}

	returnTypeAnnotation := t.ReturnTypeAnnotation.String()

	return formatFunctionType(
		true,
		typeParameters,
		parameters,
		returnTypeAnnotation,
	)
}

func (t *FunctionType) QualifiedString() string {

	typeParameters := make([]string, len(t.TypeParameters))

	for i, typeParameter := range t.TypeParameters {
		typeParameters[i] = typeParameter.QualifiedString()
	}

	parameters := make([]string, len(t.Parameters))

	for i, parameter := range t.Parameters {
		parameters[i] = parameter.QualifiedString()
	}

	returnTypeAnnotation := t.ReturnTypeAnnotation.QualifiedString()

	return formatFunctionType(
		true,
		typeParameters,
		parameters,
		returnTypeAnnotation,
	)
}

// NOTE: parameter names and argument labels are *not* part of the ID!
func (t *FunctionType) ID() TypeID {
	typeParameters := make([]string, len(t.TypeParameters))

	for i, typeParameter := range t.TypeParameters {
		typeParameters[i] = string(typeParameter.TypeBound.ID())
	}

	parameters := make([]string, len(t.Parameters))

	for i, parameter := range t.Parameters {
		parameters[i] = string(parameter.TypeAnnotation.Type.ID())
	}

	returnTypeAnnotation := string(t.ReturnTypeAnnotation.Type.ID())

	return TypeID(
		formatFunctionType(
			false,
			typeParameters,
			parameters,
			returnTypeAnnotation,
		),
	)
}

// NOTE: parameter names and argument labels are intentionally *not* considered!
func (t *FunctionType) Equal(other Type) bool {
	otherFunction, ok := other.(*FunctionType)
	if !ok {
		return false
	}

	// type parameters

	if len(t.TypeParameters) != len(otherFunction.TypeParameters) {
		return false
	}

	for i, typeParameter := range t.TypeParameters {
		otherTypeParameter := otherFunction.TypeParameters[i]
		if !typeParameter.Equal(otherTypeParameter) {
			return false
		}
	}

	// parameters

	if len(t.Parameters) != len(otherFunction.Parameters) {
		return false
	}

	for i, parameter := range t.Parameters {
		otherParameter := otherFunction.Parameters[i]
		if !parameter.TypeAnnotation.Equal(otherParameter.TypeAnnotation) {
			return false
		}
	}

	// return type

	return t.ReturnTypeAnnotation.Equal(otherFunction.ReturnTypeAnnotation)
}

// NOTE: argument labels *are* considered! parameter names are intentionally *not* considered!
func (t *FunctionType) EqualIncludingArgumentLabels(other Type) bool {
	if !t.Equal(other) {
		return false
	}

	otherFunction := other.(*FunctionType)

	for i, parameter := range t.Parameters {
		otherParameter := otherFunction.Parameters[i]
		if parameter.EffectiveArgumentLabel() != otherParameter.EffectiveArgumentLabel() {
			return false
		}
	}

	return true
}

func (*FunctionType) IsResourceType() bool {
	return false
}

func (t *FunctionType) IsInvalidType() bool {

	for _, typeParameter := range t.TypeParameters {

		if typeParameter.TypeBound != nil &&
			typeParameter.TypeBound.IsInvalidType() {

			return true
		}
	}

	for _, parameter := range t.Parameters {
		if parameter.TypeAnnotation.Type.IsInvalidType() {
			return true
		}
	}

	return t.ReturnTypeAnnotation.Type.IsInvalidType()
}

func (t *FunctionType) TypeAnnotationState() TypeAnnotationState {

	for _, typeParameter := range t.TypeParameters {
		typeParameterTypeAnnotationState := typeParameter.TypeBound.TypeAnnotationState()
		if typeParameterTypeAnnotationState != TypeAnnotationStateValid {
			return typeParameterTypeAnnotationState
		}
	}

	for _, parameter := range t.Parameters {
		parameterTypeAnnotationState := parameter.TypeAnnotation.TypeAnnotationState()
		if parameterTypeAnnotationState != TypeAnnotationStateValid {
			return parameterTypeAnnotationState
		}
	}

	returnTypeAnnotationState := t.ReturnTypeAnnotation.TypeAnnotationState()
	if returnTypeAnnotationState != TypeAnnotationStateValid {
		return returnTypeAnnotationState
	}

	return TypeAnnotationStateValid
}

func (t *FunctionType) ContainsFirstLevelInterfaceType() bool {

	for _, parameter := range t.Parameters {
		if parameter.TypeAnnotation.Type.ContainsFirstLevelInterfaceType() {
			return true
		}
	}

	return t.ReturnTypeAnnotation.Type.ContainsFirstLevelInterfaceType()
}

func (t *FunctionType) ArgumentLabels() (argumentLabels []string) {

	for _, parameter := range t.Parameters {

		argumentLabel := ArgumentLabelNotRequired
		if parameter.Label != "" {
			argumentLabel = parameter.Label
		} else if parameter.Identifier != "" {
			argumentLabel = parameter.Identifier
		}

		argumentLabels = append(argumentLabels, argumentLabel)
	}

	return
}

func (*FunctionType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	// TODO:
	return false
}

func (t *FunctionType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// SpecialFunctionType is the the type representing a special function,
// i.e., a constructor or destructor

type SpecialFunctionType struct {
	*FunctionType
	Members map[string]*Member
}

func (t *SpecialFunctionType) CanHaveMembers() bool {
	return true
}

func (t *SpecialFunctionType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	return t.Members[identifier]
}

// CheckedFunctionType is the the type representing a function that checks the arguments,
// e.g., integer functions

type CheckedFunctionType struct {
	*FunctionType
	ArgumentExpressionsCheck func(checker *Checker, argumentExpressions []ast.Expression)
}

func (t *CheckedFunctionType) CheckArgumentExpressions(checker *Checker, argumentExpressions []ast.Expression) {
	t.ArgumentExpressionsCheck(checker, argumentExpressions)
}

// baseTypes are the nominal types available in programs

var baseTypes map[string]Type

func init() {

	baseTypes = map[string]Type{
		"": &VoidType{},
	}

	otherTypes := []Type{
		&MetaType{},
		&VoidType{},
		&AnyStructType{},
		&AnyResourceType{},
		&NeverType{},
		&BoolType{},
		&CharacterType{},
		&StringType{},
		&AddressType{},
		&AuthAccountType{},
		&PublicAccountType{},
		&PathType{},
		&CapabilityType{},
	}

	types := append(
		AllNumberTypes,
		otherTypes...,
	)

	for _, ty := range types {
		typeName := ty.String()

		// check type is not accidentally redeclared
		if _, ok := baseTypes[typeName]; ok {
			panic(errors.NewUnreachableError())
		}

		baseTypes[typeName] = ty
	}
}

// baseValues are the values available in programs

var BaseValues = map[string]ValueDeclaration{}

type baseFunction struct {
	name           string
	invokableType  InvokableType
	argumentLabels []string
}

func (f baseFunction) ValueDeclarationType() Type {
	return f.invokableType
}

func (baseFunction) ValueDeclarationKind() common.DeclarationKind {
	return common.DeclarationKindFunction
}

func (baseFunction) ValueDeclarationPosition() ast.Position {
	return ast.Position{}
}

func (baseFunction) ValueDeclarationIsConstant() bool {
	return true
}

func (f baseFunction) ValueDeclarationArgumentLabels() []string {
	return f.argumentLabels
}

var AllSignedFixedPointTypes = []Type{
	&Fix64Type{},
}

var AllUnsignedFixedPointTypes = []Type{
	&UFix64Type{},
}

var AllFixedPointTypes = append(
	AllUnsignedFixedPointTypes,
	AllSignedFixedPointTypes...,
)

var AllSignedIntegerTypes = []Type{
	&IntType{},
	&Int8Type{},
	&Int16Type{},
	&Int32Type{},
	&Int64Type{},
	&Int128Type{},
	&Int256Type{},
}

var AllUnsignedIntegerTypes = []Type{
	// UInt*
	&UIntType{},
	&UInt8Type{},
	&UInt16Type{},
	&UInt32Type{},
	&UInt64Type{},
	&UInt128Type{},
	&UInt256Type{},
	// Word*
	&Word8Type{},
	&Word16Type{},
	&Word32Type{},
	&Word64Type{},
}

var AllIntegerTypes = append(
	AllUnsignedIntegerTypes,
	AllSignedIntegerTypes...,
)

var AllNumberTypes = append(
	AllIntegerTypes,
	AllFixedPointTypes...,
)

func init() {

	for _, numberType := range AllNumberTypes {
		typeName := numberType.String()

		// check type is not accidentally redeclared
		if _, ok := BaseValues[typeName]; ok {
			panic(errors.NewUnreachableError())
		}

		BaseValues[typeName] = baseFunction{
			name: typeName,
			invokableType: &CheckedFunctionType{
				FunctionType: &FunctionType{
					Parameters: []*Parameter{
						{
							Label:          ArgumentLabelNotRequired,
							Identifier:     "value",
							TypeAnnotation: NewTypeAnnotation(&NumberType{}),
						},
					},
					ReturnTypeAnnotation: &TypeAnnotation{Type: numberType},
				},
				ArgumentExpressionsCheck: numberFunctionArgumentExpressionsChecker(numberType),
			},
		}
	}
}

func init() {
	addressType := &AddressType{}
	typeName := addressType.String()

	// check type is not accidentally redeclared
	if _, ok := BaseValues[typeName]; ok {
		panic(errors.NewUnreachableError())
	}

	BaseValues[typeName] = baseFunction{
		name: typeName,
		invokableType: &CheckedFunctionType{
			FunctionType: &FunctionType{
				Parameters: []*Parameter{
					{
						Label:          ArgumentLabelNotRequired,
						Identifier:     "value",
						TypeAnnotation: NewTypeAnnotation(&IntegerType{}),
					},
				},
				ReturnTypeAnnotation: &TypeAnnotation{Type: addressType},
			},
			ArgumentExpressionsCheck: func(checker *Checker, argumentExpressions []ast.Expression) {
				if len(argumentExpressions) < 1 {
					return
				}

				intExpression, ok := argumentExpressions[0].(*ast.IntegerExpression)
				if !ok {
					return
				}

				checker.checkAddressLiteral(intExpression)
			},
		},
	}
}

func numberFunctionArgumentExpressionsChecker(numberType Type) func(*Checker, []ast.Expression) {
	return func(checker *Checker, argumentExpressions []ast.Expression) {
		if len(argumentExpressions) < 1 {
			return
		}

		switch numberExpression := argumentExpressions[0].(type) {
		case *ast.IntegerExpression:
			checker.checkIntegerLiteral(numberExpression, numberType)

		case *ast.FixedPointExpression:
			checker.checkFixedPointLiteral(numberExpression, numberType)
		}
	}
}

func init() {

	metaType := &MetaType{}
	typeName := metaType.String()

	// check type is not accidentally redeclared
	if _, ok := BaseValues[typeName]; ok {
		panic(errors.NewUnreachableError())
	}

	BaseValues[typeName] = baseFunction{
		name: typeName,
		invokableType: &FunctionType{
			TypeParameters:       []*TypeParameter{{Name: "T"}},
			ReturnTypeAnnotation: &TypeAnnotation{Type: metaType},
		},
	}
}

// CompositeType

type CompositeType struct {
	Location   ast.Location
	Identifier string
	Kind       common.CompositeKind
	// an internal set of field `ExplicitInterfaceConformances`
	explicitInterfaceConformanceSet     InterfaceSet
	ExplicitInterfaceConformances       []*InterfaceType
	ImplicitTypeRequirementConformances []*CompositeType
	Members                             map[string]*Member
	// TODO: add support for overloaded initializers
	ConstructorParameters []*Parameter
	nestedTypes           map[string]Type
	ContainerType         Type
}

func (t *CompositeType) ExplicitInterfaceConformanceSet() InterfaceSet {
	if t.explicitInterfaceConformanceSet == nil {
		// TODO: also include conformances' conformances recursively
		//   once interface can have conformances

		t.explicitInterfaceConformanceSet = make(InterfaceSet, len(t.ExplicitInterfaceConformances))
		for _, conformance := range t.ExplicitInterfaceConformances {
			t.explicitInterfaceConformanceSet.Add(conformance)
		}
	}

	return t.explicitInterfaceConformanceSet
}

func (t *CompositeType) AddImplicitTypeRequirementConformance(typeRequirement *CompositeType) {
	t.ImplicitTypeRequirementConformances =
		append(t.ImplicitTypeRequirementConformances, typeRequirement)
}

func (*CompositeType) IsType() {}

func (t *CompositeType) String() string {
	return t.Identifier
}

func (t *CompositeType) QualifiedString() string {
	return t.QualifiedIdentifier()
}

func (t *CompositeType) GetContainerType() Type {
	return t.ContainerType
}

func (t *CompositeType) GetCompositeKind() common.CompositeKind {
	return t.Kind
}

func (t *CompositeType) GetLocation() ast.Location {
	return t.Location
}

func (t *CompositeType) QualifiedIdentifier() string {
	return qualifiedIdentifier(t.Identifier, t.ContainerType)
}

func (t *CompositeType) ID() TypeID {
	return TypeID(fmt.Sprintf("%s.%s", t.Location.ID(), t.QualifiedIdentifier()))
}

func (t *CompositeType) Equal(other Type) bool {
	otherStructure, ok := other.(*CompositeType)
	if !ok {
		return false
	}

	return otherStructure.Kind == t.Kind &&
		otherStructure.ID() == t.ID()
}

func (t *CompositeType) CanHaveMembers() bool {
	return true
}

func (t *CompositeType) GetMember(identifier string, accessRange ast.Range, report func(error)) *Member {
	member := t.Members[identifier]
	if member != nil {
		return member
	}

	// Check conformances.
	// If this composite type results from a normal composite declaration,
	// it must have members declared for all interfaces it conforms to.
	// However, if this composite type is a type requirement,
	// it acts like an interface and does not have to declare members.

	for conformance := range t.ExplicitInterfaceConformanceSet() {
		member = conformance.GetMember(identifier, accessRange, report)
		if member != nil {
			return member
		}
	}

	return nil
}

func (t *CompositeType) IsResourceType() bool {
	return t.Kind == common.CompositeKindResource
}

func (*CompositeType) IsInvalidType() bool {
	return false
}

func (*CompositeType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*CompositeType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (t *CompositeType) InterfaceType() *InterfaceType {
	return &InterfaceType{
		Location:              t.Location,
		Identifier:            t.Identifier,
		CompositeKind:         t.Kind,
		Members:               t.Members,
		InitializerParameters: t.ConstructorParameters,
		ContainerType:         t.ContainerType,
		nestedTypes:           t.nestedTypes,
	}
}

func (t *CompositeType) TypeRequirements() []*CompositeType {

	var typeRequirements []*CompositeType

	if containerComposite, ok := t.ContainerType.(*CompositeType); ok {
		for _, conformance := range containerComposite.ExplicitInterfaceConformances {
			ty := conformance.nestedTypes[t.Identifier]
			typeRequirement, ok := ty.(*CompositeType)
			if !ok {
				continue
			}

			typeRequirements = append(typeRequirements, typeRequirement)
		}
	}

	return typeRequirements
}

func (*CompositeType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	// TODO:
	return false
}

func (t *CompositeType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (t *CompositeType) NestedTypes() map[string]Type {
	return t.nestedTypes
}

// AuthAccountType

type AuthAccountType struct{}

func (*AuthAccountType) IsType() {}

func (*AuthAccountType) String() string {
	return "AuthAccount"
}

func (*AuthAccountType) QualifiedString() string {
	return "AuthAccount"
}

func (*AuthAccountType) ID() TypeID {
	return "AuthAccount"
}

func (*AuthAccountType) Equal(other Type) bool {
	_, ok := other.(*AuthAccountType)
	return ok
}

func (*AuthAccountType) IsResourceType() bool {
	return false
}

func (*AuthAccountType) IsInvalidType() bool {
	return false
}

func (*AuthAccountType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*AuthAccountType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*AuthAccountType) CanHaveMembers() bool {
	return true
}

var authAccountSetCodeFunctionType = &FunctionType{
	Parameters: []*Parameter{
		{
			Label:      ArgumentLabelNotRequired,
			Identifier: "code",
			TypeAnnotation: NewTypeAnnotation(
				&VariableSizedType{
					// TODO: UInt8. Requires array literals of integer literals
					//   to be type compatible with with [UInt8]
					Type: &IntType{},
				},
			),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(
		&VoidType{},
	),
	// additional arguments are passed to the contract initializer
	RequiredArgumentCount: (func() *int {
		var count = 1
		return &count
	})(),
}

var authAccountAddPublicKeyFunctionType = &FunctionType{
	Parameters: []*Parameter{
		{
			Label:      ArgumentLabelNotRequired,
			Identifier: "key",
			TypeAnnotation: NewTypeAnnotation(
				&VariableSizedType{
					// TODO: UInt8. Requires array literals of integer literals
					//   to be type compatible with with [UInt8]
					Type: &IntType{},
				},
			),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(
		&VoidType{},
	),
}

var authAccountRemovePublicKeyFunctionType = &FunctionType{
	Parameters: []*Parameter{
		{
			Label:      ArgumentLabelNotRequired,
			Identifier: "index",
			TypeAnnotation: NewTypeAnnotation(
				&IntType{},
			),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(
		&VoidType{},
	),
}

var authAccountSaveFunctionType = func() *FunctionType {

	typeParameter := &TypeParameter{Name: "T"}

	return &FunctionType{
		TypeParameters: []*TypeParameter{
			typeParameter,
		},
		Parameters: []*Parameter{
			{
				Label:      ArgumentLabelNotRequired,
				Identifier: "value",
				TypeAnnotation: NewTypeAnnotation(
					&GenericType{
						TypeParameter: typeParameter,
					},
				),
			},
			{
				Label:          "to",
				Identifier:     "path",
				TypeAnnotation: NewTypeAnnotation(&PathType{}),
			},
		},
		ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
	}
}()

var authAccountLoadFunctionType = func() *FunctionType {

	typeParameter := &TypeParameter{Name: "T"}

	return &FunctionType{
		TypeParameters: []*TypeParameter{
			typeParameter,
		},
		Parameters: []*Parameter{
			{
				Label:          "from",
				Identifier:     "path",
				TypeAnnotation: NewTypeAnnotation(&PathType{}),
			},
		},
		ReturnTypeAnnotation: NewTypeAnnotation(
			&OptionalType{
				Type: &GenericType{
					TypeParameter: typeParameter,
				},
			},
		),
	}
}()

var authAccountCopyFunctionType = func() *FunctionType {

	typeParameter := &TypeParameter{
		Name:      "T",
		TypeBound: &AnyStructType{},
	}

	return &FunctionType{
		TypeParameters: []*TypeParameter{
			typeParameter,
		},
		Parameters: []*Parameter{
			{
				Label:          "from",
				Identifier:     "path",
				TypeAnnotation: NewTypeAnnotation(&PathType{}),
			},
		},
		ReturnTypeAnnotation: NewTypeAnnotation(
			&OptionalType{
				Type: &GenericType{
					TypeParameter: typeParameter,
				},
			},
		),
	}
}()

var authAccountBorrowFunctionType = func() *FunctionType {

	typeParameter := &TypeParameter{
		TypeBound: &ReferenceType{
			Type: &AnyType{},
		},
		Name: "T",
	}

	return &FunctionType{
		TypeParameters: []*TypeParameter{
			typeParameter,
		},
		Parameters: []*Parameter{
			{
				Label:          "from",
				Identifier:     "path",
				TypeAnnotation: NewTypeAnnotation(&PathType{}),
			},
		},
		ReturnTypeAnnotation: NewTypeAnnotation(
			&OptionalType{
				Type: &GenericType{
					TypeParameter: typeParameter,
				},
			},
		),
	}
}()

var authAccountLinkFunctionType = func() *FunctionType {

	typeParameter := &TypeParameter{
		TypeBound: &ReferenceType{
			Type: &AnyType{},
		},
		Name: "T",
	}

	return &FunctionType{
		TypeParameters: []*TypeParameter{
			typeParameter,
		},
		Parameters: []*Parameter{
			{
				Label:          ArgumentLabelNotRequired,
				Identifier:     "newCapabilityPath",
				TypeAnnotation: NewTypeAnnotation(&PathType{}),
			},
			{
				Identifier:     "target",
				TypeAnnotation: NewTypeAnnotation(&PathType{}),
			},
		},
		ReturnTypeAnnotation: NewTypeAnnotation(
			&OptionalType{
				Type: &CapabilityType{},
			},
		),
	}
}()

var authAccountUnlinkFunctionType = &FunctionType{
	Parameters: []*Parameter{
		{
			Label:          ArgumentLabelNotRequired,
			Identifier:     "capabilityPath",
			TypeAnnotation: NewTypeAnnotation(&PathType{}),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
}

var accountGetCapabilityFunctionType = &FunctionType{
	Parameters: []*Parameter{
		{
			Label:          ArgumentLabelNotRequired,
			Identifier:     "capabilityPath",
			TypeAnnotation: NewTypeAnnotation(&PathType{}),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(
		&OptionalType{
			Type: &CapabilityType{},
		},
	),
}

var accountGetLinkTargetFunctionType = &FunctionType{
	Parameters: []*Parameter{
		{
			Label:          ArgumentLabelNotRequired,
			Identifier:     "capabilityPath",
			TypeAnnotation: NewTypeAnnotation(&PathType{}),
		},
	},
	ReturnTypeAnnotation: NewTypeAnnotation(
		&OptionalType{
			Type: &PathType{},
		},
	),
}

func (t *AuthAccountType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {

	newField := func(fieldType Type) *Member {
		return NewPublicConstantFieldMember(t, identifier, fieldType)
	}

	newFunction := func(functionType InvokableType) *Member {
		return NewPublicFunctionMember(t, identifier, functionType)
	}

	switch identifier {
	case "address":
		return newField(&AddressType{})

	case "setCode", "unsafeNotInitializingSetCode":
		return newFunction(authAccountSetCodeFunctionType)

	case "addPublicKey":
		return newFunction(authAccountAddPublicKeyFunctionType)

	case "removePublicKey":
		return newFunction(authAccountRemovePublicKeyFunctionType)

	case "save":
		return newFunction(authAccountSaveFunctionType)

	case "load":
		return newFunction(authAccountLoadFunctionType)

	case "copy":
		return newFunction(authAccountCopyFunctionType)

	case "borrow":
		return newFunction(authAccountBorrowFunctionType)

	case "link":
		return newFunction(authAccountLinkFunctionType)

	case "unlink":
		return newFunction(authAccountUnlinkFunctionType)

	case "getCapability":
		return newFunction(accountGetCapabilityFunctionType)

	case "getLinkTarget":
		return newFunction(accountGetLinkTargetFunctionType)

	default:
		return nil
	}
}

func (*AuthAccountType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *AuthAccountType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// PublicAccountType

type PublicAccountType struct{}

func (*PublicAccountType) IsType() {}

func (*PublicAccountType) String() string {
	return "PublicAccount"
}

func (*PublicAccountType) QualifiedString() string {
	return "PublicAccount"
}

func (*PublicAccountType) ID() TypeID {
	return "PublicAccount"
}

func (*PublicAccountType) Equal(other Type) bool {
	_, ok := other.(*PublicAccountType)
	return ok
}

func (*PublicAccountType) IsResourceType() bool {
	return false
}

func (*PublicAccountType) IsInvalidType() bool {
	return false
}

func (*PublicAccountType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*PublicAccountType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*PublicAccountType) CanHaveMembers() bool {
	return true
}

func (t *PublicAccountType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {

	newField := func(fieldType Type) *Member {
		return NewPublicConstantFieldMember(t, identifier, fieldType)
	}

	newFunction := func(functionType InvokableType) *Member {
		return NewPublicFunctionMember(t, identifier, functionType)
	}

	switch identifier {
	case "address":
		return newField(&AddressType{})

	case "getCapability":
		return newFunction(accountGetCapabilityFunctionType)

	case "getLinkTarget":
		return newFunction(accountGetLinkTargetFunctionType)

	default:
		return nil
	}
}

func (*PublicAccountType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *PublicAccountType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// Member

type Member struct {
	ContainerType   Type
	Access          ast.Access
	Identifier      ast.Identifier
	TypeAnnotation  *TypeAnnotation
	DeclarationKind common.DeclarationKind
	VariableKind    ast.VariableKind
	ArgumentLabels  []string
	// Predeclared fields can be considered initialized
	Predeclared bool
	// IgnoreInSerialization fields are ignored in serialization
	IgnoreInSerialization bool
}

func NewPublicFunctionMember(containerType Type, identifier string, invokableType InvokableType) *Member {

	return &Member{
		ContainerType:   containerType,
		Access:          ast.AccessPublic,
		Identifier:      ast.Identifier{Identifier: identifier},
		DeclarationKind: common.DeclarationKindFunction,
		VariableKind:    ast.VariableKindConstant,
		TypeAnnotation:  &TypeAnnotation{Type: invokableType},
		ArgumentLabels:  invokableType.ArgumentLabels(),
	}
}

func NewPublicConstantFieldMember(containerType Type, identifier string, fieldType Type) *Member {
	return &Member{
		ContainerType:   containerType,
		Access:          ast.AccessPublic,
		Identifier:      ast.Identifier{Identifier: identifier},
		DeclarationKind: common.DeclarationKindField,
		VariableKind:    ast.VariableKindConstant,
		TypeAnnotation:  NewTypeAnnotation(fieldType),
	}
}

// InterfaceType

type InterfaceType struct {
	Location      ast.Location
	Identifier    string
	CompositeKind common.CompositeKind
	Members       map[string]*Member
	// TODO: add support for overloaded initializers
	InitializerParameters []*Parameter
	ContainerType         Type
	nestedTypes           map[string]Type
}

func (*InterfaceType) IsType() {}

func (t *InterfaceType) String() string {
	return t.Identifier
}

func (t *InterfaceType) QualifiedString() string {
	return t.QualifiedIdentifier()
}

func (t *InterfaceType) GetContainerType() Type {
	return t.ContainerType
}

func (t *InterfaceType) GetCompositeKind() common.CompositeKind {
	return t.CompositeKind
}

func (t *InterfaceType) GetLocation() ast.Location {
	return t.Location
}

func (t *InterfaceType) QualifiedIdentifier() string {
	return qualifiedIdentifier(t.Identifier, t.ContainerType)
}

func (t *InterfaceType) ID() TypeID {
	return TypeID(fmt.Sprintf("%s.%s", t.Location.ID(), t.QualifiedIdentifier()))
}

func (t *InterfaceType) Equal(other Type) bool {
	otherInterface, ok := other.(*InterfaceType)
	if !ok {
		return false
	}

	return otherInterface.CompositeKind == t.CompositeKind &&
		otherInterface.ID() == t.ID()
}

func (t *InterfaceType) CanHaveMembers() bool {
	return true
}

func (t *InterfaceType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	return t.Members[identifier]
}

func (t *InterfaceType) IsResourceType() bool {
	return t.CompositeKind == common.CompositeKindResource
}

func (t *InterfaceType) IsInvalidType() bool {
	return false
}

func (*InterfaceType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (t *InterfaceType) ContainsFirstLevelInterfaceType() bool {
	return t.CompositeKind == common.CompositeKindResource ||
		t.CompositeKind == common.CompositeKindStructure
}

func (*InterfaceType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	// TODO:
	return false
}

func (t *InterfaceType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (t *InterfaceType) NestedTypes() map[string]Type {
	return t.nestedTypes
}

// DictionaryType

type DictionaryType struct {
	KeyType   Type
	ValueType Type
}

func (*DictionaryType) IsType() {}

func (t *DictionaryType) String() string {
	return fmt.Sprintf(
		"{%s: %s}",
		t.KeyType,
		t.ValueType,
	)
}

func (t *DictionaryType) QualifiedString() string {
	return fmt.Sprintf(
		"{%s: %s}",
		t.KeyType.QualifiedString(),
		t.ValueType.QualifiedString(),
	)
}

func (t *DictionaryType) ID() TypeID {
	return TypeID(fmt.Sprintf(
		"{%s:%s}",
		t.KeyType.ID(),
		t.ValueType.ID(),
	))
}

func (t *DictionaryType) Equal(other Type) bool {
	otherDictionary, ok := other.(*DictionaryType)
	if !ok {
		return false
	}

	return otherDictionary.KeyType.Equal(t.KeyType) &&
		otherDictionary.ValueType.Equal(t.ValueType)
}

func (t *DictionaryType) IsResourceType() bool {
	return t.KeyType.IsResourceType() ||
		t.ValueType.IsResourceType()
}

func (t *DictionaryType) IsInvalidType() bool {
	return t.KeyType.IsInvalidType() ||
		t.ValueType.IsInvalidType()
}

func (t *DictionaryType) TypeAnnotationState() TypeAnnotationState {
	keyTypeAnnotationState := t.KeyType.TypeAnnotationState()
	if keyTypeAnnotationState != TypeAnnotationStateValid {
		return keyTypeAnnotationState
	}

	valueTypeAnnotationState := t.ValueType.TypeAnnotationState()
	if valueTypeAnnotationState != TypeAnnotationStateValid {
		return valueTypeAnnotationState
	}

	return TypeAnnotationStateValid
}

func (t *DictionaryType) ContainsFirstLevelInterfaceType() bool {
	return t.KeyType.ContainsFirstLevelInterfaceType() ||
		t.ValueType.ContainsFirstLevelInterfaceType()
}

func (t *DictionaryType) CanHaveMembers() bool {
	return true
}

func (t *DictionaryType) GetMember(identifier string, targetRange ast.Range, report func(error)) *Member {
	newField := func(fieldType Type) *Member {
		return NewPublicConstantFieldMember(t, identifier, fieldType)
	}

	newFunction := func(functionType *FunctionType) *Member {
		return NewPublicFunctionMember(t, identifier, functionType)
	}

	switch identifier {
	case "length":
		return newField(&IntType{})

	case "keys":
		// TODO: maybe allow for resource key type

		if t.KeyType.IsResourceType() {
			report(
				&InvalidResourceDictionaryMemberError{
					Name:            identifier,
					DeclarationKind: common.DeclarationKindField,
					Range:           targetRange,
				},
			)
		}

		return newField(&VariableSizedType{Type: t.KeyType})

	case "values":
		// TODO: maybe allow for resource value type

		if t.ValueType.IsResourceType() {
			report(
				&InvalidResourceDictionaryMemberError{
					Name:            identifier,
					DeclarationKind: common.DeclarationKindField,
					Range:           targetRange,
				},
			)
		}

		return newField(&VariableSizedType{Type: t.ValueType})

	case "insert":
		return newFunction(
			&FunctionType{
				Parameters: []*Parameter{
					{
						Identifier:     "key",
						TypeAnnotation: NewTypeAnnotation(t.KeyType),
					},
					{
						Label:          ArgumentLabelNotRequired,
						Identifier:     "value",
						TypeAnnotation: NewTypeAnnotation(t.ValueType),
					},
				},
				ReturnTypeAnnotation: NewTypeAnnotation(
					&OptionalType{
						Type: t.ValueType,
					},
				),
			},
		)

	case "remove":
		return newFunction(
			&FunctionType{
				Parameters: []*Parameter{
					{
						Identifier:     "key",
						TypeAnnotation: NewTypeAnnotation(t.KeyType),
					},
				},
				ReturnTypeAnnotation: NewTypeAnnotation(
					&OptionalType{
						Type: t.ValueType,
					},
				),
			},
		)

	default:
		return nil
	}
}

func (t *DictionaryType) isValueIndexableType() bool {
	return true
}

func (t *DictionaryType) ElementType(_ bool) Type {
	return &OptionalType{Type: t.ValueType}
}

func (t *DictionaryType) IndexingType() Type {
	return t.KeyType
}

type DictionaryEntryType struct {
	KeyType   Type
	ValueType Type
}

func (t *DictionaryType) Unify(
	other Type,
	typeParameters map[*TypeParameter]Type,
	report func(err error),
	outerRange ast.Range,
) bool {
	otherDictionary, ok := other.(*DictionaryType)
	if !ok {
		return false
	}

	return t.KeyType.Unify(otherDictionary.KeyType, typeParameters, report, outerRange) &&
		t.ValueType.Unify(otherDictionary.ValueType, typeParameters, report, outerRange)
}

func (t *DictionaryType) Resolve(typeParameters map[*TypeParameter]Type) Type {
	newKeyType := t.KeyType.Resolve(typeParameters)
	if newKeyType == nil {
		return nil
	}

	newValueType := t.ValueType.Resolve(typeParameters)
	if newValueType == nil {
		return nil
	}

	return &DictionaryType{
		KeyType:   newKeyType,
		ValueType: newValueType,
	}
}

// ReferenceType represents the reference to a value
type ReferenceType struct {
	Authorized bool
	Type       Type
}

func (*ReferenceType) IsType() {}

func (t *ReferenceType) String() string {
	if t.Type == nil {
		return "reference"
	}
	var builder strings.Builder
	if t.Authorized {
		builder.WriteString("auth ")
	}
	builder.WriteRune('&')
	builder.WriteString(t.Type.String())
	return builder.String()
}

func (t *ReferenceType) QualifiedString() string {
	if t.Type == nil {
		return "reference"
	}
	var builder strings.Builder
	if t.Authorized {
		builder.WriteString("auth ")
	}
	builder.WriteRune('&')
	builder.WriteString(t.Type.QualifiedString())
	return builder.String()
}

func (t *ReferenceType) ID() TypeID {
	var builder strings.Builder
	if t.Authorized {
		builder.WriteString("auth ")
	}
	builder.WriteRune('&')
	if t.Type != nil {
		builder.WriteString(string(t.Type.ID()))
	}
	return TypeID(builder.String())
}

func (t *ReferenceType) Equal(other Type) bool {
	otherReference, ok := other.(*ReferenceType)
	if !ok {
		return false
	}

	return t.Authorized == otherReference.Authorized &&
		t.Type.Equal(otherReference.Type)
}

func (t *ReferenceType) IsResourceType() bool {
	return false
}

func (t *ReferenceType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

func (*ReferenceType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (t *ReferenceType) ContainsFirstLevelInterfaceType() bool {
	return t.Type.ContainsFirstLevelInterfaceType()
}

func (t *ReferenceType) CanHaveMembers() bool {
	referencedType, ok := t.Type.(MemberAccessibleType)
	if !ok {
		return false
	}
	return referencedType.CanHaveMembers()
}

func (t *ReferenceType) GetMember(identifier string, targetRange ast.Range, report func(error)) *Member {
	// forward to referenced type, if it has members
	referencedTypeWithMember, ok := t.Type.(MemberAccessibleType)
	if !ok {
		return nil
	}
	return referencedTypeWithMember.GetMember(identifier, targetRange, report)
}

func (t *ReferenceType) isValueIndexableType() bool {
	referencedType, ok := t.Type.(ValueIndexableType)
	if !ok {
		return false
	}
	return referencedType.isValueIndexableType()
}

func (t *ReferenceType) ElementType(isAssignment bool) Type {
	referencedType, ok := t.Type.(ValueIndexableType)
	if !ok {
		return nil
	}
	return referencedType.ElementType(isAssignment)
}

func (t *ReferenceType) IndexingType() Type {
	referencedType, ok := t.Type.(ValueIndexableType)
	if !ok {
		return nil
	}
	return referencedType.IndexingType()
}

func (*ReferenceType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	// TODO:
	return false
}

func (t *ReferenceType) Resolve(_ map[*TypeParameter]Type) Type {
	// TODO:
	return t
}

// AddressType represents the address type
type AddressType struct{}

func (*AddressType) IsType() {}

func (*AddressType) String() string {
	return "Address"
}

func (*AddressType) QualifiedString() string {
	return "Address"
}

func (*AddressType) ID() TypeID {
	return "Address"
}

func (*AddressType) Equal(other Type) bool {
	_, ok := other.(*AddressType)
	return ok
}

func (*AddressType) IsResourceType() bool {
	return false
}

func (*AddressType) IsInvalidType() bool {
	return false
}

func (*AddressType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*AddressType) ContainsFirstLevelInterfaceType() bool {
	return false
}

var AddressTypeMinIntBig = new(big.Int)
var AddressTypeMaxIntBig = new(big.Int).SetUint64(math.MaxUint64)

func (*AddressType) MinInt() *big.Int {
	return AddressTypeMinIntBig
}

func (*AddressType) MaxInt() *big.Int {
	return AddressTypeMaxIntBig
}

func (*AddressType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *AddressType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

func (*AddressType) CanHaveMembers() bool {
	return true
}

func (t *AddressType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	builtinFunctionTypes, ok := typeBuiltinFunctionTypes[t.ID()]
	if !ok {
		return nil
	}
	functionType, ok := builtinFunctionTypes[identifier]
	if !ok {
		return nil
	}
	return NewPublicFunctionMember(t, identifier, functionType)
}

// IsSubType determines if the given subtype is a subtype
// of the given supertype.
//
// Types are subtypes of themselves.
//
func IsSubType(subType Type, superType Type) bool {

	if subType.Equal(superType) {
		return true
	}

	if _, ok := subType.(*NeverType); ok {
		return true
	}

	switch superType.(type) {
	case *AnyType:
		return true

	case *AnyStructType:
		return !subType.IsResourceType()

	case *AnyResourceType:
		return subType.IsResourceType()
	}

	switch typedSuperType := superType.(type) {

	case *NumberType:
		if _, ok := subType.(*NumberType); ok {
			return true
		}

		return IsSubType(subType, &IntegerType{}) ||
			IsSubType(subType, &FixedPointType{})

	case *SignedNumberType:
		if _, ok := subType.(*SignedNumberType); ok {
			return true
		}

		return IsSubType(subType, &SignedIntegerType{}) ||
			IsSubType(subType, &SignedFixedPointType{})

	case *IntegerType:
		switch subType.(type) {
		case *IntegerType, *SignedIntegerType,
			*IntType, *UIntType,
			*Int8Type, *Int16Type, *Int32Type, *Int64Type, *Int128Type, *Int256Type,
			*UInt8Type, *UInt16Type, *UInt32Type, *UInt64Type, *UInt128Type, *UInt256Type,
			*Word8Type, *Word16Type, *Word32Type, *Word64Type:

			return true

		default:
			return false
		}

	case *SignedIntegerType:
		switch subType.(type) {
		case *SignedIntegerType,
			*IntType,
			*Int8Type, *Int16Type, *Int32Type, *Int64Type, *Int128Type, *Int256Type:

			return true

		default:
			return false
		}

	case *FixedPointType:
		switch subType.(type) {
		case *FixedPointType, *SignedFixedPointType,
			*Fix64Type, *UFix64Type:

			return true

		default:
			return false
		}

	case *SignedFixedPointType:
		switch subType.(type) {
		case *SignedNumberType, *Fix64Type:

			return true

		default:
			return false
		}

	case *OptionalType:
		optionalSubType, ok := subType.(*OptionalType)
		if !ok {
			// T <: U? if T <: U
			return IsSubType(subType, typedSuperType.Type)
		}
		// Optionals are covariant: T? <: U? if T <: U
		return IsSubType(optionalSubType.Type, typedSuperType.Type)

	case *DictionaryType:
		typedSubType, ok := subType.(*DictionaryType)
		if !ok {
			return false
		}

		return IsSubType(typedSubType.KeyType, typedSuperType.KeyType) &&
			IsSubType(typedSubType.ValueType, typedSuperType.ValueType)

	case *VariableSizedType:
		typedSubType, ok := subType.(*VariableSizedType)
		if !ok {
			return false
		}

		return IsSubType(
			typedSubType.ElementType(false),
			typedSuperType.ElementType(false),
		)

	case *ConstantSizedType:
		typedSubType, ok := subType.(*ConstantSizedType)
		if !ok {
			return false
		}

		if typedSubType.Size != typedSuperType.Size {
			return false
		}

		return IsSubType(
			typedSubType.ElementType(false),
			typedSuperType.ElementType(false),
		)

	case *ReferenceType:
		// References types are only subtypes of reference types

		typedSubType, ok := subType.(*ReferenceType)
		if !ok {
			return false
		}

		// An authorized reference type `auth &T`
		// is a subtype of a reference type `&U` (authorized or non-authorized),
		// if `T` is a subtype of `U`

		if typedSubType.Authorized {
			return IsSubType(typedSubType.Type, typedSuperType.Type)
		}

		// An unauthorized reference type is not a subtype of an authorized reference type.
		// Not even dynamically.
		//
		// The holder of the reference may not gain more permissions.

		if typedSuperType.Authorized {
			return false
		}

		switch typedInnerSuperType := typedSuperType.Type.(type) {
		case *RestrictedType:

			switch restrictedSuperType := typedInnerSuperType.Type.(type) {
			case *AnyResourceType, *AnyStructType, *AnyType:

				switch typedInnerSubType := typedSubType.Type.(type) {
				case *RestrictedType:
					// An unauthorized reference to a restricted type `&T{Us}`
					// is a subtype of a reference to a restricted type
					// `&AnyResource{Vs}` / `&AnyStruct{Vs}` / `&Any{Vs}`:
					// if the `T` is a subset of the supertype's restricted type,
					// and `Vs` is a subset of `Us`.
					//
					// The holder of the reference may only further restrict the reference.
					//
					// The requirement for `T` to conform to `Vs` is implied by the subset requirement.

					return IsSubType(typedInnerSubType.Type, restrictedSuperType) &&
						typedInnerSuperType.RestrictionSet().
							IsSubsetOf(typedInnerSubType.RestrictionSet())

				case *CompositeType:
					// An unauthorized reference to an unrestricted type `&T`
					// is a subtype of a reference to a restricted type
					// `&AnyResource{Us}` / `&AnyStruct{Us}` / `&Any{Us}`:
					// When `T != AnyResource && T != AnyStruct && T != Any`:
					// if `T` conforms to `Us`.
					//
					// The holder of the reference may only restrict the reference.

					// TODO: once interfaces can conform to interfaces, include
					return IsSubType(typedInnerSubType, restrictedSuperType) &&
						typedInnerSuperType.RestrictionSet().
							IsSubsetOf(typedInnerSubType.ExplicitInterfaceConformanceSet())

				case *AnyResourceType, *AnyStructType, *AnyType:
					// An unauthorized reference to an unrestricted type `&T`
					// is a subtype of a reference to a restricted type
					// `&AnyResource{Us}` / `&AnyStruct{Us}` / `&Any{Us}`:
					// When `T == AnyResource || T == AnyStruct || T == Any`: never.
					//
					// The holder of the reference may not gain more permissions or knowledge.

					return false
				}

			default:

				switch typedInnerSubType := typedSubType.Type.(type) {
				case *RestrictedType:

					// An unauthorized reference to a restricted type `&T{Us}`
					// is a subtype of a reference to a restricted type `&V{Ws}:`

					switch typedInnerSubType.Type.(type) {
					case *CompositeType:
						// When `T != AnyResource && T != AnyStruct && T != Any`:
						// if `T == V` and `Ws` is a subset of `Us`.
						//
						// The holder of the reference may not gain more permissions or knowledge
						// and may only further restrict the reference to the composite.

						return typedInnerSubType.Type == typedInnerSuperType.Type &&
							typedInnerSuperType.RestrictionSet().
								IsSubsetOf(typedInnerSubType.RestrictionSet())

					case *AnyResourceType, *AnyStructType, *AnyType:
						// When `T == AnyResource || T == AnyStruct || T == Any`: never.

						return false
					}

				case *CompositeType:
					// An unauthorized reference to an unrestricted type `&T`
					// is a subtype of a reference to a restricted type `&U{Vs}`:
					// When `T != AnyResource && T != AnyStruct && T != Any`: if `T == U`.
					//
					// The holder of the reference may only further restrict the reference.

					return typedInnerSubType == typedInnerSuperType.Type

				case *AnyResourceType, *AnyStructType, *AnyType:
					// An unauthorized reference to an unrestricted type `&T`
					// is a subtype of a reference to a restricted type `&U{Vs}`:
					// When `T == AnyResource || T == AnyStruct || T == Any`: never.
					//
					// The holder of the reference may not gain more permissions or knowledge.

					return false
				}
			}

		case *CompositeType:
			// An unauthorized reference is not a subtype of a reference to a composite type `&V`
			// (e.g. reference to a restricted type `&T{Us}`, or reference to an interface type `&T`)
			//
			// The holder of the reference may not gain more permissions or knowledge.

			return false

		case *AnyType:

			// An unauthorized reference to a restricted type `&T{Us}`
			// or to a unrestricted type `&T`
			// is a subtype of the type `&Any`: always.

			return true

		case *AnyResourceType:

			// An unauthorized reference to a restricted type `&T{Us}`
			// or to a unrestricted type `&T`
			// is a subtype of the type `&AnyResource`:
			// if `T == AnyResource` or `T` is a resource-kinded composite.

			switch typedInnerSubType := typedSubType.Type.(type) {
			case *RestrictedType:
				switch typedInnerInnerSubType := typedInnerSubType.Type.(type) {
				case *AnyResourceType:
					return true

				case *CompositeType:
					return typedInnerInnerSubType.Kind == common.CompositeKindResource

				default:
					return false
				}

			case *CompositeType:
				return typedInnerSubType.Kind == common.CompositeKindResource
			}

		case *AnyStructType:

			// An unauthorized reference to a restricted type `&T{Us}`
			// or to a unrestricted type `&T`
			// is a subtype of the type `&AnyStruct`:
			// if `T == AnyStruct` or `T` is a struct-kinded composite.

			switch typedInnerSubType := typedSubType.Type.(type) {
			case *RestrictedType:
				switch typedInnerInnerSubType := typedInnerSubType.Type.(type) {
				case *AnyStructType:
					return true

				case *CompositeType:
					return typedInnerInnerSubType.Kind == common.CompositeKindStructure

				default:
					return false
				}

			case *CompositeType:
				return typedInnerSubType.Kind == common.CompositeKindStructure
			}
		}

	case *FunctionType:
		typedSubType, ok := subType.(*FunctionType)
		if !ok {
			return false
		}

		if len(typedSubType.Parameters) != len(typedSuperType.Parameters) {
			return false
		}

		// Functions are contravariant in their parameter types

		for i, subParameter := range typedSubType.Parameters {
			superParameter := typedSuperType.Parameters[i]
			if !IsSubType(
				superParameter.TypeAnnotation.Type,
				subParameter.TypeAnnotation.Type,
			) {
				return false
			}
		}

		// Functions are covariant in their return type

		if typedSubType.ReturnTypeAnnotation != nil &&
			typedSuperType.ReturnTypeAnnotation != nil {

			return IsSubType(
				typedSubType.ReturnTypeAnnotation.Type,
				typedSuperType.ReturnTypeAnnotation.Type,
			)
		}

		if typedSubType.ReturnTypeAnnotation == nil &&
			typedSuperType.ReturnTypeAnnotation == nil {

			return true
		}

	case *RestrictedType:

		switch restrictedSuperType := typedSuperType.Type.(type) {
		case *AnyResourceType, *AnyStructType, *AnyType:

			switch typedSubType := subType.(type) {
			case *RestrictedType:

				// A restricted type `T{Us}`
				// is a subtype of a restricted type `AnyResource{Vs}` / `AnyStruct{Vs}` / `Any{Vs}`:

				switch restrictedSubtype := typedSubType.Type.(type) {
				case *AnyResourceType, *AnyStructType, *AnyType:
					// When `T == AnyResource || T == AnyStruct || T == Any`:
					// if the restricted type of the subtype
					// is a subtype of the restricted supertype,
					// and `Vs` is a subset of `Us`.

					return IsSubType(restrictedSubtype, restrictedSuperType) &&
						typedSuperType.RestrictionSet().
							IsSubsetOf(typedSubType.RestrictionSet())

				case *CompositeType:
					// When `T != AnyResource && T != AnyStruct && T != Any`:
					// if the restricted type of the subtype
					// is a subtype of the restricted supertype,
					// and `T` conforms to `Vs`.
					// `Us` and `Vs` do *not* have to be subsets.

					// TODO: once interfaces can conform to interfaces, include
					return IsSubType(restrictedSubtype, restrictedSuperType) &&
						typedSuperType.RestrictionSet().
							IsSubsetOf(restrictedSubtype.ExplicitInterfaceConformanceSet())
				}

			case *AnyResourceType:
				// `AnyResource` is a subtype of a restricted type
				// - `AnyResource{Us}`: not statically;
				// - `AnyStruct{Us}`: never.
				// - `Any{Us}`: not statically;

				return false

			case *AnyStructType:
				// `AnyStruct` is a subtype of a restricted type
				// - `AnyStruct{Us}`: not statically.
				// - `AnyResource{Us}`: never;
				// - `Any{Us}`: not statically.

				return false

			case *AnyType:
				// `Any` is a subtype of a restricted type
				// - `Any{Us}: not statically.`
				// - `AnyStruct{Us}`: never;
				// - `AnyResource{Us}`: never;

				return false

			case *CompositeType:
				// An unrestricted type `T`
				// is a subtype of a restricted type `AnyResource{Us}` / `AnyStruct{Us}` / `Any{Us}`:
				// if `T` is a subtype of the restricted supertype,
				// and `T` conforms to `Us`.

				return IsSubType(typedSubType, typedSuperType.Type) &&
					typedSuperType.RestrictionSet().
						IsSubsetOf(typedSubType.ExplicitInterfaceConformanceSet())
			}

		default:

			switch typedSubType := subType.(type) {
			case *RestrictedType:

				// A restricted type `T{Us}`
				// is a subtype of a restricted type `V{Ws}`:

				switch restrictedSubType := typedSubType.Type.(type) {
				case *AnyResourceType, *AnyStructType, *AnyType:
					// When `T == AnyResource || T == AnyStruct || T == Any`:
					// not statically.
					return false

				case *CompositeType:
					// When `T != AnyResource && T != AnyStructType && T != Any`: if `T == V`.
					//
					// `Us` and `Ws` do *not* have to be subsets:
					// The owner may freely restrict and unrestrict.

					return restrictedSubType == typedSuperType.Type
				}

			case *CompositeType:
				// An unrestricted type `T`
				// is a subtype of a restricted type `U{Vs}`: if `T == U`.
				//
				// The owner may freely restrict.

				return typedSubType == typedSuperType.Type

			case *AnyResourceType, *AnyStructType, *AnyType:
				// An unrestricted type `T`
				// is a subtype of a restricted type `AnyResource{Vs}` / `AnyStruct{Vs}` / `Any{Vs}`:
				// not statically.

				return false
			}
		}

	case *CompositeType:

		// NOTE: type equality case (composite type `T` is subtype of composite type `U`)
		// is already handled at beginning of function

		switch typedSubType := subType.(type) {
		case *RestrictedType:

			// A restricted type `T{Us}`
			// is a subtype of an unrestricted type `V`:

			switch restrictedSubType := typedSubType.Type.(type) {
			case *AnyResourceType, *AnyStructType, *AnyType:
				// When `T == AnyResource || T == AnyStruct || T == Any`: not statically.
				return false

			case *CompositeType:
				// When `T != AnyResource && T != AnyStruct`: if `T == V`.
				//
				// The owner may freely unrestrict.

				return restrictedSubType == typedSuperType
			}

		case *CompositeType:
			// The supertype composite type might be a type requirement.
			// Check if the subtype composite type implicitly conforms to it.

			for _, conformance := range typedSubType.ImplicitTypeRequirementConformances {
				if conformance == typedSuperType {
					return true
				}
			}
		}

	case *InterfaceType:

		switch typedSubType := subType.(type) {
		case *CompositeType:

			// Resources are not subtypes of resource interfaces.
			// (Use `AnyResource` / `AnyStruct` / `Any` with restriction instead).

			if typedSuperType.CompositeKind == common.CompositeKindResource ||
				typedSuperType.CompositeKind == common.CompositeKindStructure {

				return false
			}

			// A composite type `T` is a subtype of a interface type `V`:
			// if `T` conforms to `V`, and `V` and `T` are of the same kind

			if typedSubType.Kind != typedSuperType.CompositeKind {
				return false
			}

			// TODO: once interfaces can conform to interfaces, include
			if _, ok := typedSubType.ExplicitInterfaceConformanceSet()[typedSuperType]; ok {
				return true
			}

			return false

		case *InterfaceType:
			// TODO: Once interfaces can conform to interfaces, check conformances here
			return false
		}
	}

	return false
}

func IsConcatenatableType(ty Type) bool {
	_, isArrayType := ty.(ArrayType)
	return IsSubType(ty, &StringType{}) || isArrayType
}

func IsEquatableType(ty Type) bool {

	// TODO: add support for arrays and dictionaries
	// TODO: add support for composites that are equatable

	if IsSubType(ty, &MetaType{}) ||
		IsSubType(ty, &StringType{}) ||
		IsSubType(ty, &BoolType{}) ||
		IsSubType(ty, &NumberType{}) ||
		IsSubType(ty, &ReferenceType{}) ||
		IsSubType(ty, &AddressType{}) {

		return true
	}

	if optionalType, ok := ty.(*OptionalType); ok {
		return IsEquatableType(optionalType.Type)
	}

	return false
}

// UnwrapOptionalType returns the type if it is not an optional type,
// or the inner-most type if it is (optional types are repeatedly unwrapped)
//
func UnwrapOptionalType(ty Type) Type {
	for {
		optionalType, ok := ty.(*OptionalType)
		if !ok {
			return ty
		}
		ty = optionalType.Type
	}
}

func AreCompatibleEquatableTypes(leftType, rightType Type) bool {
	unwrappedLeftType := UnwrapOptionalType(leftType)
	unwrappedRightType := UnwrapOptionalType(rightType)

	leftIsEquatable := IsEquatableType(unwrappedLeftType)
	rightIsEquatable := IsEquatableType(unwrappedRightType)

	if unwrappedLeftType.Equal(unwrappedRightType) &&
		leftIsEquatable && rightIsEquatable {

		return true
	}

	// The types are equatable if this is a comparison with `nil`,
	// which has type `Never?`

	if IsNilType(leftType) || IsNilType(rightType) {
		return true
	}

	return false
}

// IsNilType returns true if the given type is the type of `nil`, i.e. `Never?`.
//
func IsNilType(ty Type) bool {
	optionalType, ok := ty.(*OptionalType)
	if !ok {
		return false
	}

	if _, ok := optionalType.Type.(*NeverType); !ok {
		return false
	}

	return true
}

type TransactionType struct {
	Members           map[string]*Member
	PrepareParameters []*Parameter
	Parameters        []*Parameter
}

func (t *TransactionType) EntryPointFunctionType() *FunctionType {
	return &FunctionType{
		Parameters:           append(t.Parameters, t.PrepareParameters...),
		ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
	}
}

func (t *TransactionType) PrepareFunctionType() *SpecialFunctionType {
	return &SpecialFunctionType{
		FunctionType: &FunctionType{
			Parameters:           t.PrepareParameters,
			ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
		},
	}
}

func (*TransactionType) ExecuteFunctionType() *SpecialFunctionType {
	return &SpecialFunctionType{
		FunctionType: &FunctionType{
			Parameters:           []*Parameter{},
			ReturnTypeAnnotation: NewTypeAnnotation(&VoidType{}),
		},
	}
}

func (*TransactionType) IsType() {}

func (*TransactionType) String() string {
	return "Transaction"
}

func (*TransactionType) QualifiedString() string {
	return "Transaction"
}

func (*TransactionType) ID() TypeID {
	return "Transaction"
}

func (*TransactionType) Equal(other Type) bool {
	_, ok := other.(*TransactionType)
	return ok
}

func (*TransactionType) IsResourceType() bool {
	return false
}

func (*TransactionType) IsInvalidType() bool {
	return false
}

func (*TransactionType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*TransactionType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (t *TransactionType) CanHaveMembers() bool {
	return true
}

func (t *TransactionType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	return t.Members[identifier]
}

func (*TransactionType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *TransactionType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// InterfaceSet

type InterfaceSet map[*InterfaceType]struct{}

func (s InterfaceSet) IsSubsetOf(other InterfaceSet) bool {
	for interfaceType := range s {
		if _, ok := other[interfaceType]; !ok {
			return false
		}
	}

	return true
}

func (s InterfaceSet) Includes(interfaceType *InterfaceType) bool {
	_, ok := s[interfaceType]
	return ok
}

func (s InterfaceSet) Add(interfaceType *InterfaceType) {
	s[interfaceType] = struct{}{}
}

// RestrictedType
//
// No restrictions implies the type is fully restricted,
// i.e. no members of the underlying resource type are available.
//
type RestrictedType struct {
	Type         Type
	Restrictions []*InterfaceType
	// an internal set of field `Restrictions`
	restrictionSet InterfaceSet
}

func (t *RestrictedType) RestrictionSet() InterfaceSet {
	if t.restrictionSet == nil {
		t.restrictionSet = make(InterfaceSet, len(t.Restrictions))
		for _, restriction := range t.Restrictions {
			t.restrictionSet[restriction] = struct{}{}
		}
	}
	return t.restrictionSet
}

func (*RestrictedType) IsType() {}

func (t *RestrictedType) String() string {
	var result strings.Builder
	result.WriteString(t.Type.String())
	result.WriteRune('{')
	for i, restriction := range t.Restrictions {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(restriction.String())
	}
	result.WriteRune('}')
	return result.String()
}

func (t *RestrictedType) QualifiedString() string {
	var result strings.Builder
	result.WriteString(t.Type.QualifiedString())
	result.WriteRune('{')
	for i, restriction := range t.Restrictions {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(restriction.QualifiedString())
	}
	result.WriteRune('}')
	return result.String()
}

func (t *RestrictedType) ID() TypeID {
	var result strings.Builder
	result.WriteString(string(t.Type.ID()))
	result.WriteRune('{')
	for i, restriction := range t.Restrictions {
		if i > 0 {
			result.WriteString(",")
		}
		result.WriteString(string(restriction.ID()))
	}
	result.WriteRune('}')
	return TypeID(result.String())
}

func (t *RestrictedType) Equal(other Type) bool {
	otherRestrictedType, ok := other.(*RestrictedType)
	if !ok {
		return false
	}

	if !otherRestrictedType.Type.Equal(t.Type) {
		return false
	}

	// Check that the set of restrictions are equal; order does not matter

	restrictionSet := t.RestrictionSet()
	otherRestrictionSet := otherRestrictedType.RestrictionSet()

	count := len(restrictionSet)
	if count != len(otherRestrictionSet) {
		return false
	}

	return restrictionSet.IsSubsetOf(otherRestrictionSet)
}

func (t *RestrictedType) IsResourceType() bool {
	if t.Type == nil {
		return false
	}
	return t.Type.IsResourceType()
}

func (t *RestrictedType) IsInvalidType() bool {
	if t.Type != nil && t.Type.IsInvalidType() {
		return true
	}

	for _, restriction := range t.Restrictions {
		if restriction.IsInvalidType() {
			return true
		}
	}

	return false
}

func (*RestrictedType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*RestrictedType) ContainsFirstLevelInterfaceType() bool {
	// Even though the restrictions should be resource interfaces,
	// they are not on the "first level", i.e. not the restricted type
	return false
}

func (t *RestrictedType) CanHaveMembers() bool {
	return true
}

func (t *RestrictedType) GetMember(identifier string, targetRange ast.Range, reportError func(error)) *Member {

	// Return the first member of any restriction.
	// The invariant that restrictions may not have overlapping members is not checked here,
	// but implicitly when the resource declaration's conformances are checked.

	for _, restriction := range t.Restrictions {
		member := restriction.GetMember(identifier, targetRange, reportError)
		if member != nil {
			return member
		}
	}

	// If none of the restrictions had a member, see if
	// the restricted type has a member with the identifier.
	//
	// Still return it for convenience to help check the rest
	// of the program and improve the developer experience,
	// *but* also report an error that this access is invalid
	//
	// The restricted type may be `AnyResource`,
	// in which case there are no members.

	if memberAccessibleType, ok := t.Type.(MemberAccessibleType); ok {
		member := memberAccessibleType.GetMember(identifier, targetRange, reportError)

		if member != nil {
			reportError(
				&InvalidRestrictedTypeMemberAccessError{
					Name:  identifier,
					Range: targetRange,
				},
			)
		}

		return member
	}

	return nil
}

func (*RestrictedType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	// TODO: how do we unify the restriction sets?
	return false
}

func (t *RestrictedType) Resolve(_ map[*TypeParameter]Type) Type {
	// TODO:
	return t
}

// PathType

type PathType struct{}

func (*PathType) IsType() {}

func (*PathType) String() string {
	return "Path"
}

func (*PathType) QualifiedString() string {
	return "Path"
}

func (*PathType) ID() TypeID {
	return "Path"
}

func (*PathType) Equal(other Type) bool {
	_, ok := other.(*PathType)
	return ok
}

func (*PathType) IsResourceType() bool {
	return false
}

func (*PathType) IsInvalidType() bool {
	return false
}

func (*PathType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*PathType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*PathType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *PathType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

// CapabilityType

type CapabilityType struct{}

func (*CapabilityType) IsType() {}

func (*CapabilityType) String() string {
	return "Capability"
}

func (*CapabilityType) QualifiedString() string {
	return "Capability"
}

func (*CapabilityType) ID() TypeID {
	return "Capability"
}

func (*CapabilityType) Equal(other Type) bool {
	_, ok := other.(*CapabilityType)
	return ok
}

func (*CapabilityType) IsResourceType() bool {
	return false
}

func (*CapabilityType) IsInvalidType() bool {
	return false
}

func (*CapabilityType) TypeAnnotationState() TypeAnnotationState {
	return TypeAnnotationStateValid
}

func (*CapabilityType) ContainsFirstLevelInterfaceType() bool {
	return false
}

func (*CapabilityType) Unify(_ Type, _ map[*TypeParameter]Type, _ func(err error), _ ast.Range) bool {
	return false
}

func (t *CapabilityType) Resolve(_ map[*TypeParameter]Type) Type {
	return t
}

var capabilityBorrowFunctionType = func() *FunctionType {

	typeParameter := &TypeParameter{
		TypeBound: &ReferenceType{
			Type: &AnyType{},
		},
		Name: "T",
	}

	return &FunctionType{
		TypeParameters: []*TypeParameter{
			typeParameter,
		},
		ReturnTypeAnnotation: NewTypeAnnotation(
			&OptionalType{
				Type: &GenericType{
					TypeParameter: typeParameter,
				},
			},
		),
	}
}()

var capabilityCheckFunctionType = func() *FunctionType {

	typeParameter := &TypeParameter{
		TypeBound: &ReferenceType{
			Type: &AnyType{},
		},
		Name: "T",
	}

	return &FunctionType{
		TypeParameters: []*TypeParameter{
			typeParameter,
		},
		ReturnTypeAnnotation: NewTypeAnnotation(&BoolType{}),
	}
}()

func (t *CapabilityType) CanHaveMembers() bool {
	return true
}

func (t *CapabilityType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {

	newFunction := func(functionType InvokableType) *Member {
		return NewPublicFunctionMember(t, identifier, functionType)
	}

	switch identifier {
	case "borrow":
		return newFunction(capabilityBorrowFunctionType)

	case "check":
		return newFunction(capabilityCheckFunctionType)

	default:
		return nil
	}
}
