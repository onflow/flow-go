//revive:disable

package sema

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
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
	isType()
	String() string
	Equal(other Type) bool
	IsResourceType() bool
	IsInvalidType() bool
	ID() TypeID
}

// ValueIndexableType

type ValueIndexableType interface {
	Type
	isValueIndexableType() bool
	ElementType(isAssignment bool) Type
	IndexingType() Type
}

// TypeIndexableType

type TypeIndexableType interface {
	Type
	isTypeIndexableType()
	IsAssignable() bool
	IsValidIndexingType(indexingType Type) (isValid bool, expectedTypeDescription string)
	ElementType(indexingType Type, isAssignment bool) Type
}

// TypeAnnotation

type TypeAnnotation struct {
	IsResource bool
	Type       Type
}

func (a *TypeAnnotation) String() string {
	if a.IsResource {
		return fmt.Sprintf("<-%s", a.Type)
	} else {
		return fmt.Sprint(a.Type)
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

// AnyType represents the top type of all types.
// NOTE: This type is only used internally and not available in programs.
type AnyType struct{}

func (*AnyType) isType() {}

func (*AnyType) String() string {
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

// AnyStructType represents the top type of all non-resource types
type AnyStructType struct{}

func (*AnyStructType) isType() {}

func (*AnyStructType) String() string {
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

// AnyResourceType represents the top type of all resource types
type AnyResourceType struct{}

func (*AnyResourceType) isType() {}

func (*AnyResourceType) String() string {
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

// NeverType represents the bottom type
type NeverType struct{}

func (*NeverType) isType() {}

func (*NeverType) String() string {
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

// VoidType represents the void type
type VoidType struct{}

func (*VoidType) isType() {}

func (*VoidType) String() string {
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

// InvalidType represents a type that is invalid.
// It is the result of type checking failing and
// can't be expressed in programs.
//
type InvalidType struct{}

func (*InvalidType) isType() {}

func (t *InvalidType) String() string {
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

// OptionalType represents the optional variant of another type
type OptionalType struct {
	Type Type
}

func (*OptionalType) isType() {}

func (t *OptionalType) String() string {
	if t.Type == nil {
		return "optional"
	}
	return fmt.Sprintf("%s?", t.Type)
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

// BoolType represents the boolean type
type BoolType struct{}

func (*BoolType) isType() {}

func (*BoolType) String() string {
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

// CharacterType represents the character type

type CharacterType struct{}

func (*CharacterType) isType() {}

func (*CharacterType) String() string {
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

// StringType represents the string type
type StringType struct{}

func (*StringType) isType() {}

func (*StringType) String() string {
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

func (*StringType) HasMembers() bool {
	return true
}

func (t *StringType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	switch identifier {
	case "concat":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
				&FunctionType{
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
				},
			),
		})

	case "slice":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
				&FunctionType{
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
				},
			),
			ArgumentLabels: []string{"from", "upTo"},
		})

	case "length":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation:  NewTypeAnnotation(&IntType{}),
		})

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

// Ranged

type Ranged interface {
	Min() *big.Int
	Max() *big.Int
}

// IntegerType represents the super-type of all integer types
type IntegerType struct{}

func (*IntegerType) isType() {}

func (*IntegerType) String() string {
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

func (*IntegerType) Min() *big.Int {
	return nil
}

func (*IntegerType) Max() *big.Int {
	return nil
}

// IntType represents the arbitrary-precision integer type `Int`
type IntType struct{}

func (*IntType) isType() {}

func (*IntType) String() string {
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

func (*IntType) Min() *big.Int {
	return nil
}

func (*IntType) Max() *big.Int {
	return nil
}

// Int8Type represents the 8-bit signed integer type `Int8`

type Int8Type struct{}

func (*Int8Type) isType() {}

func (*Int8Type) String() string {
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

var Int8TypeMin = big.NewInt(0).SetInt64(math.MinInt8)
var Int8TypeMax = big.NewInt(0).SetInt64(math.MaxInt8)

func (*Int8Type) Min() *big.Int {
	return Int8TypeMin
}

func (*Int8Type) Max() *big.Int {
	return Int8TypeMax
}

// Int16Type represents the 16-bit signed integer type `Int16`
type Int16Type struct{}

func (*Int16Type) isType() {}

func (*Int16Type) String() string {
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

var Int16TypeMin = big.NewInt(0).SetInt64(math.MinInt16)
var Int16TypeMax = big.NewInt(0).SetInt64(math.MaxInt16)

func (*Int16Type) Min() *big.Int {
	return Int16TypeMin
}

func (*Int16Type) Max() *big.Int {
	return Int16TypeMax
}

// Int32Type represents the 32-bit signed integer type `Int32`
type Int32Type struct{}

func (*Int32Type) isType() {}

func (*Int32Type) String() string {
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

var Int32TypeMin = big.NewInt(0).SetInt64(math.MinInt32)
var Int32TypeMax = big.NewInt(0).SetInt64(math.MaxInt32)

func (*Int32Type) Min() *big.Int {
	return Int32TypeMin
}

func (*Int32Type) Max() *big.Int {
	return Int32TypeMax
}

// Int64Type represents the 64-bit signed integer type `Int64`
type Int64Type struct{}

func (*Int64Type) isType() {}

func (*Int64Type) String() string {
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

var Int64TypeMin = big.NewInt(0).SetInt64(math.MinInt64)
var Int64TypeMax = big.NewInt(0).SetInt64(math.MaxInt64)

func (*Int64Type) Min() *big.Int {
	return Int64TypeMin
}

func (*Int64Type) Max() *big.Int {
	return Int64TypeMax
}

// UInt8Type represents the 8-bit unsigned integer type `UInt8`
type UInt8Type struct{}

func (*UInt8Type) isType() {}

func (*UInt8Type) String() string {
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

var UInt8TypeMin = big.NewInt(0)
var UInt8TypeMax = big.NewInt(0).SetUint64(math.MaxUint8)

func (*UInt8Type) Min() *big.Int {
	return UInt8TypeMin
}

func (*UInt8Type) Max() *big.Int {
	return UInt8TypeMax
}

// UInt16Type represents the 16-bit unsigned integer type `UInt16`
type UInt16Type struct{}

func (*UInt16Type) isType() {}

func (*UInt16Type) String() string {
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

var UInt16TypeMin = big.NewInt(0)
var UInt16TypeMax = big.NewInt(0).SetUint64(math.MaxUint16)

func (*UInt16Type) Min() *big.Int {
	return UInt16TypeMin
}

func (*UInt16Type) Max() *big.Int {
	return UInt16TypeMax
}

// UInt32Type represents the 32-bit unsigned integer type `UInt32`
type UInt32Type struct{}

func (*UInt32Type) isType() {}

func (*UInt32Type) String() string {
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

var UInt32TypeMin = big.NewInt(0)
var UInt32TypeMax = big.NewInt(0).SetUint64(math.MaxUint32)

func (*UInt32Type) Min() *big.Int {
	return UInt32TypeMin
}

func (*UInt32Type) Max() *big.Int {
	return UInt32TypeMax
}

// UInt64Type represents the 64-bit unsigned integer type `UInt64`
type UInt64Type struct{}

func (*UInt64Type) isType() {}

func (*UInt64Type) String() string {
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

var UInt64TypeMin = big.NewInt(0)
var UInt64TypeMax = big.NewInt(0).SetUint64(math.MaxUint64)

func (*UInt64Type) Min() *big.Int {
	return UInt64TypeMin
}

func (*UInt64Type) Max() *big.Int {
	return UInt64TypeMax
}

// ArrayType

type ArrayType interface {
	ValueIndexableType
	isArrayType()
}

func getArrayMember(arrayType ArrayType, field string, targetRange ast.Range, report func(error)) *Member {

	switch field {
	case "append":
		// Appending elements to a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)
		return NewCheckedMember(&Member{
			ContainerType:   arrayType,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: field},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
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
			),
		})

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

		return NewCheckedMember(&Member{
			ContainerType:   arrayType,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: field},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
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
			),
		})

	case "insert":
		// Inserting elements into to a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)

		return NewCheckedMember(&Member{
			ContainerType:   arrayType,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: field},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
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
			),
			ArgumentLabels: []string{"at", ArgumentLabelNotRequired},
		})

	case "remove":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)

		return NewCheckedMember(&Member{
			ContainerType:   arrayType,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: field},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
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
			),
			ArgumentLabels: []string{"at"},
		})

	case "removeFirst":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)

		return NewCheckedMember(&Member{
			ContainerType:   arrayType,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: field},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
				&FunctionType{
					ReturnTypeAnnotation: NewTypeAnnotation(
						elementType,
					),
				},
			),
		})

	case "removeLast":
		// Removing elements from a constant sized array is not allowed

		if _, isConstantSized := arrayType.(*ConstantSizedType); isConstantSized {
			// TODO: maybe return member but report helpful error?
			return nil
		}

		elementType := arrayType.ElementType(false)

		return NewCheckedMember(&Member{
			ContainerType:   arrayType,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: field},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
				&FunctionType{
					ReturnTypeAnnotation: NewTypeAnnotation(
						elementType,
					),
				},
			),
		})

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

		return NewCheckedMember(&Member{
			ContainerType:   arrayType,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: field},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
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
			),
		})

	case "length":
		return NewCheckedMember(&Member{
			ContainerType:   arrayType,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: field},
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation:  NewTypeAnnotation(&IntType{}),
		})

	default:
		return nil
	}
}

// VariableSizedType is a variable sized array type
type VariableSizedType struct {
	Type
}

func (*VariableSizedType) isType()      {}
func (*VariableSizedType) isArrayType() {}

func (t *VariableSizedType) String() string {
	return fmt.Sprintf("[%s]", t.Type)
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

func (t *VariableSizedType) HasMembers() bool {
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

func (t *VariableSizedType) isValueIndexableType() bool {
	return true
}

func (t *VariableSizedType) ElementType(_ bool) Type {
	return t.Type
}

func (t *VariableSizedType) IndexingType() Type {
	return &IntegerType{}
}

// ConstantSizedType is a constant sized array type
type ConstantSizedType struct {
	Type
	Size int
}

func (*ConstantSizedType) isType()      {}
func (*ConstantSizedType) isArrayType() {}

func (t *ConstantSizedType) String() string {
	return fmt.Sprintf("[%s; %d]", t.Type, t.Size)
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

func (t *ConstantSizedType) HasMembers() bool {
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

func (t *ConstantSizedType) isValueIndexableType() bool {
	return true
}

func (t *ConstantSizedType) ElementType(_ bool) Type {
	return t.Type
}

func (t *ConstantSizedType) IndexingType() Type {
	return &IntegerType{}
}

// InvokableType

type InvokableType interface {
	Type
	InvocationFunctionType() *FunctionType
	CheckArgumentExpressions(checker *Checker, argumentExpressions []ast.Expression)
}

// Parameter

type Parameter struct {
	Label          string
	Identifier     string
	TypeAnnotation *TypeAnnotation
}

func (t *Parameter) String() string {
	if t.Label != "" {
		return fmt.Sprintf(
			"%s %s: %s",
			t.Label,
			t.Identifier,
			t.TypeAnnotation.String(),
		)
	}

	if t.Identifier != "" {
		return fmt.Sprintf(
			"%s: %s",
			t.Identifier,
			t.TypeAnnotation.String(),
		)
	}

	return t.TypeAnnotation.String()
}

// FunctionType

type FunctionType struct {
	Parameters            []*Parameter
	ReturnTypeAnnotation  *TypeAnnotation
	ReturnTypeGetter      func(argumentTypes []Type) Type
	RequiredArgumentCount *int
}

func (t *FunctionType) ReturnType(argumentTypes []Type) Type {
	if len(argumentTypes) == len(t.Parameters) &&
		t.ReturnTypeGetter != nil {

		return t.ReturnTypeGetter(argumentTypes)
	}

	return t.ReturnTypeAnnotation.Type
}

func (*FunctionType) isType() {}

func (t *FunctionType) InvocationFunctionType() *FunctionType {
	return t
}

func (*FunctionType) CheckArgumentExpressions(_ *Checker, _ []ast.Expression) {
	// NO-OP: no checks for normal functions
}

func (t *FunctionType) String() string {
	var parameters strings.Builder
	for i, parameter := range t.Parameters {
		if i > 0 {
			parameters.WriteString(", ")
		}
		parameters.WriteString(parameter.String())
	}

	return fmt.Sprintf(
		"((%s): %s)",
		parameters.String(),
		t.ReturnTypeAnnotation,
	)
}

// NOTE: parameter names and argument labels are *not* part of the ID!
func (t *FunctionType) ID() TypeID {
	var parameters strings.Builder
	for i, parameter := range t.Parameters {
		if i > 0 {
			parameters.WriteString(",")
		}
		parameters.WriteString(string(parameter.TypeAnnotation.Type.ID()))
	}

	return TypeID(fmt.Sprintf("((%s):%s)", parameters.String(), t.ReturnTypeAnnotation))
}

// NOTE: parameter names and argument labels are intentionally *not* considered!
func (t *FunctionType) Equal(other Type) bool {
	otherFunction, ok := other.(*FunctionType)
	if !ok {
		return false
	}

	if len(t.Parameters) != len(otherFunction.Parameters) {
		return false
	}

	for i, parameter := range t.Parameters {
		otherParameter := otherFunction.Parameters[i]
		if !parameter.TypeAnnotation.Equal(otherParameter.TypeAnnotation) {
			return false
		}
	}

	return t.ReturnTypeAnnotation.Equal(otherFunction.ReturnTypeAnnotation)
}

func (*FunctionType) IsResourceType() bool {
	return false
}

func (t *FunctionType) IsInvalidType() bool {
	if t.ReturnTypeAnnotation.Type.IsInvalidType() {
		return true
	}

	for _, parameter := range t.Parameters {
		if parameter.TypeAnnotation.Type.IsInvalidType() {
			return true
		}
	}

	return false
}

// SpecialFunctionType is the the type representing a special function,
// i.e., a constructor or destructor

type SpecialFunctionType struct {
	*FunctionType
	Members map[string]*Member
}

func (t *SpecialFunctionType) HasMembers() bool {
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

	types := []Type{
		&VoidType{},
		&AnyStructType{},
		&AnyResourceType{},
		&NeverType{},
		&BoolType{},
		&CharacterType{},
		&IntType{},
		&StringType{},
		&Int8Type{},
		&Int16Type{},
		&Int32Type{},
		&Int64Type{},
		&UInt8Type{},
		&UInt16Type{},
		&UInt32Type{},
		&UInt64Type{},
		&AddressType{},
		&AccountType{},
	}

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

var BaseValues map[string]ValueDeclaration

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

func init() {
	BaseValues = map[string]ValueDeclaration{}
	initIntegerFunctions()
	initAddressFunction()
}

func initIntegerFunctions() {
	integerTypes := []Type{
		&IntType{},
		&Int8Type{},
		&Int16Type{},
		&Int32Type{},
		&Int64Type{},
		&UInt8Type{},
		&UInt16Type{},
		&UInt32Type{},
		&UInt64Type{},
	}

	for _, integerType := range integerTypes {
		typeName := integerType.String()

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
					ReturnTypeAnnotation: &TypeAnnotation{Type: integerType},
				},
				ArgumentExpressionsCheck: integerFunctionArgumentExpressionsChecker(integerType),
			},
		}
	}
}

func initAddressFunction() {
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
				intExpression, ok := argumentExpressions[0].(*ast.IntExpression)
				if !ok {
					return
				}
				checker.checkAddressLiteral(intExpression)
			},
		},
	}
}

func integerFunctionArgumentExpressionsChecker(integerType Type) func(*Checker, []ast.Expression) {
	return func(checker *Checker, argumentExpressions []ast.Expression) {
		intExpression, ok := argumentExpressions[0].(*ast.IntExpression)
		if !ok {
			return
		}
		checker.checkIntegerLiteral(intExpression, integerType)
	}
}

// CompositeType

type CompositeType struct {
	Location     ast.Location
	Identifier   string
	Kind         common.CompositeKind
	Conformances []*InterfaceType
	Members      map[string]*Member
	// TODO: add support for overloaded initializers
	ConstructorParameters []*Parameter
	NestedTypes           map[string]Type
	ContainerType         Type
}

func (*CompositeType) isType() {}

func (t *CompositeType) String() string {
	return t.Identifier
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
		otherStructure.Identifier == t.Identifier
}

func (t *CompositeType) HasMembers() bool {
	return true
}

func (t *CompositeType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	return t.Members[identifier]
}

func (t *CompositeType) IsResourceType() bool {
	return t.Kind == common.CompositeKindResource
}

func (t *CompositeType) IsInvalidType() bool {
	// TODO: maybe if any member has an invalid type?
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
		NestedTypes:           t.NestedTypes,
	}
}

func (t *CompositeType) TypeRequirements() []*CompositeType {

	var typeRequirements []*CompositeType

	if containerComposite, ok := t.ContainerType.(*CompositeType); ok {
		for _, conformance := range containerComposite.Conformances {
			ty := conformance.NestedTypes[t.Identifier]
			typeRequirement, ok := ty.(*CompositeType)
			if !ok {
				continue
			}

			typeRequirements = append(typeRequirements, typeRequirement)
		}
	}

	return typeRequirements
}

// AccountType

type AccountType struct{}

func (*AccountType) isType() {}

func (*AccountType) String() string {
	return "Account"
}

func (*AccountType) ID() TypeID {
	return "Account"
}

func (*AccountType) Equal(other Type) bool {
	_, ok := other.(*AccountType)
	return ok
}

func (*AccountType) IsResourceType() bool {
	return false
}

func (*AccountType) IsInvalidType() bool {
	return false
}

func (*AccountType) HasMembers() bool {
	return true
}

func (t *AccountType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	switch identifier {
	case "address":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			TypeAnnotation:  NewTypeAnnotation(&AddressType{}),
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
		})

	case "storage":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			TypeAnnotation:  NewTypeAnnotation(&StorageType{}),
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
		})

	case "published":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			TypeAnnotation:  NewTypeAnnotation(&ReferencesType{Assignable: true}),
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
		})

	default:
		return nil
	}
}

// PublicAccountType

type PublicAccountType struct{}

func (*PublicAccountType) isType() {}

func (*PublicAccountType) String() string {
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

func (*PublicAccountType) HasMembers() bool {
	return true
}

func (t *PublicAccountType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	switch identifier {
	case "address":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			TypeAnnotation:  NewTypeAnnotation(&AddressType{}),
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
		})

	case "published":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			TypeAnnotation:  NewTypeAnnotation(&ReferencesType{Assignable: false}),
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
		})

	default:
		return nil
	}
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
}

// NewCheckedMember panics if the member declaration is invalid.
func NewCheckedMember(member *Member) *Member {

	if member.DeclarationKind == common.DeclarationKindUnknown {
		panic(fmt.Sprintf(
			"member `%s.%s` has unknown declaration kind",
			member.ContainerType,
			member.Identifier.Identifier,
		))
	}

	if member.Access == ast.AccessNotSpecified {
		panic(fmt.Sprintf(
			"member `%s.%s` has unspecified access",
			member.ContainerType,
			member.Identifier.Identifier,
		))
	}

	if invokableType, ok := member.TypeAnnotation.Type.(InvokableType); ok {
		functionType := invokableType.InvocationFunctionType()

		if member.ArgumentLabels != nil &&
			len(member.ArgumentLabels) != len(functionType.Parameters) {

			panic(fmt.Sprintf(
				"member `%s.%s` has incorrect argument label count",
				member.ContainerType,
				member.Identifier.Identifier,
			))
		}
	} else {
		if member.ArgumentLabels != nil {
			panic(fmt.Sprintf(
				"non-function member `%s.%s` should not declare argument labels",
				member.ContainerType,
				member.Identifier.Identifier,
			))
		}
	}

	return member
}

type MemberAccessibleType interface {
	Type
	HasMembers() bool
	GetMember(identifier string, targetRange ast.Range, report func(error)) *Member
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
	NestedTypes           map[string]Type
}

func (*InterfaceType) isType() {}

func (t *InterfaceType) String() string {
	return t.Identifier
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
		otherInterface.Identifier == t.Identifier
}

func (t *InterfaceType) HasMembers() bool {
	return true
}

func (t *InterfaceType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	return t.Members[identifier]
}

func (t *InterfaceType) IsResourceType() bool {
	return t.CompositeKind == common.CompositeKindResource
}

func (t *InterfaceType) IsInvalidType() bool {
	// TODO: maybe if any member has an invalid type?
	return false
}

// DictionaryType

type DictionaryType struct {
	KeyType   Type
	ValueType Type
}

func (*DictionaryType) isType() {}

func (t *DictionaryType) String() string {
	return fmt.Sprintf(
		"{%s: %s}",
		t.KeyType,
		t.ValueType,
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

func (t *DictionaryType) HasMembers() bool {
	return true
}

func (t *DictionaryType) GetMember(identifier string, targetRange ast.Range, report func(error)) *Member {
	switch identifier {
	case "length":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation:  NewTypeAnnotation(&IntType{}),
		})

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

		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
				&VariableSizedType{Type: t.KeyType},
			),
		})

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

		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
				&VariableSizedType{Type: t.ValueType},
			),
		})

	case "insert":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
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
			),
			ArgumentLabels: []string{"key", ArgumentLabelNotRequired},
		})

	case "remove":
		return NewCheckedMember(&Member{
			ContainerType:   t,
			Access:          ast.AccessPublic,
			Identifier:      ast.Identifier{Identifier: identifier},
			DeclarationKind: common.DeclarationKindFunction,
			VariableKind:    ast.VariableKindConstant,
			TypeAnnotation: NewTypeAnnotation(
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
			),
			ArgumentLabels: []string{"key"},
		})

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

// StorageType

type StorageType struct{}

func (t *StorageType) isType() {}

func (t *StorageType) String() string {
	return "Storage"
}

func (t *StorageType) ID() TypeID {
	return "Storage"
}

func (t *StorageType) Equal(other Type) bool {
	_, ok := other.(*StorageType)
	return ok
}

func (t *StorageType) IsResourceType() bool {
	// NOTE: even though storage may contain resources,
	//   we define it to not behave like a resource
	return false
}

func (t *StorageType) IsInvalidType() bool {
	return false
}

func (t *StorageType) isTypeIndexableType() {}

func (t *StorageType) IsValidIndexingType(indexingType Type) (isValid bool, expectedTypeDescription string) {
	if _, ok := indexingType.(*ReferenceType); ok {
		return true, ""
	}

	if indexingType.IsResourceType() {
		return true, ""
	}

	return false, "resource or reference"
}

func (t *StorageType) IsAssignable() bool {
	return true
}

func (t *StorageType) ElementType(indexingType Type, _ bool) Type {
	// NOTE: like dictionary
	return &OptionalType{Type: indexingType}
}

// ReferencesType is the heterogeneous dictionary that
// is indexed by reference types and has references as values

type ReferencesType struct {
	Assignable bool
}

func (t *ReferencesType) isType() {}

func (t *ReferencesType) String() string {
	return "References"
}

func (t *ReferencesType) ID() TypeID {
	return "References"
}

func (t *ReferencesType) Equal(other Type) bool {
	otherReferences, ok := other.(*ReferencesType)
	if !ok {
		return false
	}
	return t.Assignable && otherReferences.Assignable
}

func (t *ReferencesType) IsResourceType() bool {
	return false
}

func (t *ReferencesType) IsInvalidType() bool {
	return false
}

func (t *ReferencesType) isTypeIndexableType() {}

func (t *ReferencesType) ElementType(indexingType Type, _ bool) Type {
	// NOTE: like dictionary
	return &OptionalType{Type: indexingType}
}

func (t *ReferencesType) IsAssignable() bool {
	return t.Assignable
}

func (t *ReferencesType) IsValidIndexingType(indexingType Type) (isValid bool, expectedTypeDescription string) {
	if _, isReferenceType := indexingType.(*ReferenceType); !isReferenceType {
		return false, "reference"
	}

	return true, ""
}

// ReferenceType represents the reference to a value
type ReferenceType struct {
	Type Type
}

func (*ReferenceType) isType() {}

func (t *ReferenceType) String() string {
	if t.Type == nil {
		return "reference"
	}
	return fmt.Sprintf("&%s", t.Type)
}

func (t *ReferenceType) ID() TypeID {
	var id string
	if t.Type != nil {
		id = string(t.Type.ID())
	}
	return TypeID(fmt.Sprintf("&%s", id))
}

func (t *ReferenceType) Equal(other Type) bool {
	otherReference, ok := other.(*ReferenceType)
	if !ok {
		return false
	}
	return t.Type.Equal(otherReference.Type)
}

func (t *ReferenceType) IsResourceType() bool {
	return false
}

func (t *ReferenceType) IsInvalidType() bool {
	return t.Type.IsInvalidType()
}

func (t *ReferenceType) HasMembers() bool {
	referencedType, ok := t.Type.(MemberAccessibleType)
	if !ok {
		return false
	}
	return referencedType.HasMembers()
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

// AddressType represents the address type
type AddressType struct{}

func (*AddressType) isType() {}

func (*AddressType) String() string {
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

var AddressTypeMin = big.NewInt(0)
var AddressTypeMax *big.Int

func init() {
	AddressTypeMax = big.NewInt(2)
	AddressTypeMax.Exp(AddressTypeMax, big.NewInt(160), nil)
	AddressTypeMax.Sub(AddressTypeMax, big.NewInt(1))
}

func (*AddressType) Min() *big.Int {
	return AddressTypeMin
}

func (*AddressType) Max() *big.Int {
	return AddressTypeMax
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

	switch superType.(type) {
	case *AnyType:
		return true

	case *AnyStructType:
		return !subType.IsResourceType()

	case *AnyResourceType:
		return subType.IsResourceType()
	}

	if _, ok := subType.(*NeverType); ok {
		return true
	}

	switch typedSuperType := superType.(type) {
	case *IntegerType:
		switch subType.(type) {
		case *IntType,
			*Int8Type, *Int16Type, *Int32Type, *Int64Type,
			*UInt8Type, *UInt16Type, *UInt32Type, *UInt64Type:

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

	case *InterfaceType:
		compositeSubType, ok := subType.(*CompositeType)
		if !ok {
			return false
		}
		// TODO: optimize, use set
		for _, conformance := range compositeSubType.Conformances {
			if typedSuperType.Equal(conformance) {
				return true
			}
		}
		return false

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
		typedSubType, ok := subType.(*ReferenceType)
		if !ok {
			return false
		}

		// References are covariant: &T <: &U if T <: U

		return IsSubType(typedSubType.Type, typedSuperType.Type)

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

		return false
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

	if IsSubType(ty, &StringType{}) ||
		IsSubType(ty, &BoolType{}) ||
		IsSubType(ty, &IntegerType{}) ||
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
	prepareParameters []*Parameter
}

func (t *TransactionType) EntryPointFunctionType() *FunctionType {
	return t.PrepareFunctionType().InvocationFunctionType()
}

func (t *TransactionType) PrepareFunctionType() *SpecialFunctionType {
	return &SpecialFunctionType{
		FunctionType: &FunctionType{
			Parameters:           t.prepareParameters,
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

func (*TransactionType) isType() {}

func (*TransactionType) String() string {
	return "Transaction"
}

func (*TransactionType) ID() TypeID {
	return "Transaction"
}

func (*TransactionType) Equal(_ Type) bool {
	// transaction types are not equatable
	return false
}

func (*TransactionType) IsResourceType() bool {
	return false
}

func (*TransactionType) IsInvalidType() bool {
	return false
}

func (t *TransactionType) HasMembers() bool {
	return true
}

func (t *TransactionType) GetMember(identifier string, _ ast.Range, _ func(error)) *Member {
	return t.Members[identifier]
}
