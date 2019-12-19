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
	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

type Type interface {
	isType()
	String() string
	Equal(other Type) bool
	IsResourceType() bool
	IsInvalidType() bool
	ID() string
}

type ExportableType interface {
	//TODO once https://github.com/dapperlabs/flow-go/issues/1589 is done
	// we can stop requiring AST tree to fetch extra names
	// and Variable to be able to locate this type
	Export(program *ast.Program, variable *Variable) types.Type
}

// Helper function to wrap a types in a variable if a variable is set
func wrapVariable(t types.Type, variable *Variable) types.Type {
	if variable != nil {
		return types.Variable{Type: t}
	} else {
		return t
	}
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

func NewTypeAnnotations(types ...Type) []*TypeAnnotation {
	typeAnnotations := make([]*TypeAnnotation, len(types))
	for i, ty := range types {
		typeAnnotations[i] = NewTypeAnnotation(ty)
	}
	return typeAnnotations
}

// AnyStructType represents the top type of all non-resource types
type AnyStructType struct{}

func (*AnyStructType) isType() {}

func (*AnyStructType) String() string {
	return "AnyStruct"
}

func (*AnyStructType) ID() string {
	return "AnyStruct"
}

func (*AnyStructType) Equal(other Type) bool {
	_, ok := other.(*AnyStructType)
	return ok
}

func (*AnyStructType) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.AnyStruct{}, variable)
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

func (*AnyResourceType) ID() string {
	return "AnyResource"
}

func (*AnyResourceType) Equal(other Type) bool {
	_, ok := other.(*AnyResourceType)
	return ok
}

func (*AnyResourceType) AnyResourceType(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.AnyResource{}, variable)
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

func (*NeverType) ID() string {
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

func (*VoidType) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Void{}, variable)
}

func (*VoidType) String() string {
	return "Void"
}

func (*VoidType) ID() string {
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

func (*InvalidType) ID() string {
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

func (t *OptionalType) ID() string {
	if t.Type == nil {
		return "optional"
	}
	return fmt.Sprintf("%s?", t.Type.ID())
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

func (t *OptionalType) Export(program *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Optional{
		Type: t.Type.(ExportableType).Export(program, nil),
	}, variable)
}

// BoolType represents the boolean type
type BoolType struct{}

func (*BoolType) isType() {}

func (*BoolType) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Bool{}, variable)
}

func (*BoolType) String() string {
	return "Bool"
}

func (*BoolType) ID() string {
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

func (*CharacterType) ID() string {
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

func (*StringType) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.String{}, variable)
}

func (*StringType) String() string {
	return "String"
}

func (*StringType) ID() string {
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
					ParameterTypeAnnotations: NewTypeAnnotations(
						&StringType{},
					),
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
					ParameterTypeAnnotations: NewTypeAnnotations(
						&IntType{},
						&IntType{},
					),
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

func (t *StringType) ElementType(isAssignment bool) Type {
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
	return "integer"
}

func (*IntegerType) ID() string {
	return "integer"
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

func (*IntType) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Int{}, variable)
}

func (*IntType) String() string {
	return "Int"
}

func (*IntType) ID() string {
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

func (*Int8Type) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Int8{}, variable)
}

func (*Int8Type) String() string {
	return "Int8"
}

func (*Int8Type) ID() string {
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

func (*Int16Type) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Int16{}, variable)
}

func (*Int16Type) String() string {
	return "Int16"
}

func (*Int16Type) ID() string {
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

func (*Int32Type) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Int32{}, variable)
}

func (*Int32Type) String() string {
	return "Int32"
}

func (*Int32Type) ID() string {
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

func (*Int64Type) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Int64{}, variable)
}

func (*Int64Type) String() string {
	return "Int64"
}

func (*Int64Type) ID() string {
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

func (*UInt8Type) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.UInt8{}, variable)
}

func (*UInt8Type) String() string {
	return "UInt8"
}

func (*UInt8Type) ID() string {
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

func (*UInt16Type) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.UInt16{}, variable)
}

func (*UInt16Type) String() string {
	return "UInt16"
}

func (*UInt16Type) ID() string {
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

func (*UInt32Type) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.UInt32{}, variable)
}

func (*UInt32Type) String() string {
	return "UInt32"
}

func (*UInt32Type) ID() string {
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

func (*UInt64Type) Export(_ *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.UInt64{}, variable)
}

func (*UInt64Type) String() string {
	return "UInt64"
}

func (*UInt64Type) ID() string {
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
					ParameterTypeAnnotations: NewTypeAnnotations(
						elementType,
					),
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
					ParameterTypeAnnotations: []*TypeAnnotation{
						typeAnnotation,
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
					ParameterTypeAnnotations: NewTypeAnnotations(
						&IntegerType{},
						elementType,
					),
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
					ParameterTypeAnnotations: NewTypeAnnotations(
						&IntegerType{},
					),
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
					ParameterTypeAnnotations: NewTypeAnnotations(
						elementType,
					),
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

func (t *VariableSizedType) Export(program *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.VariableSizedArray{
		ElementType: t.Type.(ExportableType).Export(program, nil),
	}, variable)
}

func (t *VariableSizedType) String() string {
	return fmt.Sprintf("[%s]", t.Type)
}

func (t *VariableSizedType) ID() string {
	return fmt.Sprintf("[%s]", t.Type.ID())
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

func (t *VariableSizedType) ElementType(isAssignment bool) Type {
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

func (t *ConstantSizedType) Export(program *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.ConstantSizedArray{
		Size:        uint(t.Size),
		ElementType: t.Type.(ExportableType).Export(program, nil),
	}, variable)
}

func (t *ConstantSizedType) String() string {
	return fmt.Sprintf("[%s; %d]", t.Type, t.Size)
}

func (t *ConstantSizedType) ID() string {
	return fmt.Sprintf("[%s;%d]", t.Type.ID(), t.Size)
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

func (t *ConstantSizedType) ElementType(isAssignment bool) Type {
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

// FunctionType

type FunctionType struct {
	ParameterTypeAnnotations []*TypeAnnotation
	ReturnTypeAnnotation     *TypeAnnotation
	GetReturnType            func(argumentTypes []Type) Type
	RequiredArgumentCount    *int
}

func (*FunctionType) isType() {}

func (t *FunctionType) Export(program *ast.Program, variable *Variable) types.Type {

	// we have function type rather than named functions with params
	if variable == nil {
		parameterTypes := make([]types.Type, len(t.ParameterTypeAnnotations))

		for i, annotation := range t.ParameterTypeAnnotations {
			parameterTypes[i] = annotation.Type.(ExportableType).Export(program, nil)
		}

		return types.FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     t.ReturnTypeAnnotation.Type.(ExportableType).Export(program, nil),
		}

	} else {
		functionDeclaration := func() *ast.FunctionDeclaration {
			for _, fn := range program.FunctionDeclarations() {
				if fn.Identifier.Identifier == variable.Identifier && fn.Identifier.Pos == *variable.Pos {
					return fn
				}
			}

			panic(fmt.Sprintf("cannot find type %v declaration in AST tree", t))
		}()

		parameterTypeAnnotations := make([]types.Parameter, len(t.ParameterTypeAnnotations))

		for i, annotation := range t.ParameterTypeAnnotations {

			astParam := functionDeclaration.ParameterList.Parameters[i]

			parameterTypeAnnotations[i] = types.Parameter{
				Label:      astParam.Label,
				Identifier: astParam.Identifier.Identifier,
				Type:       annotation.Type.(ExportableType).Export(program, nil),
			}
		}

		return types.Function{
			Parameters: parameterTypeAnnotations,
			ReturnType: t.ReturnTypeAnnotation.Type.(ExportableType).Export(program, nil),
		}.WithID(t.ID())
	}
}

func (t *FunctionType) InvocationFunctionType() *FunctionType {
	return t
}

func (*FunctionType) CheckArgumentExpressions(checker *Checker, argumentExpressions []ast.Expression) {
	// NO-OP: no checks for normal functions
}

func (t *FunctionType) String() string {
	var parameters strings.Builder
	for i, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		if i > 0 {
			parameters.WriteString(", ")
		}
		parameters.WriteString(parameterTypeAnnotation.String())
	}

	return fmt.Sprintf(
		"((%s): %s)",
		parameters.String(),
		t.ReturnTypeAnnotation,
	)
}

func (t *FunctionType) ID() string {
	var parameters strings.Builder
	for i, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		if i > 0 {
			parameters.WriteString(",")
		}
		parameters.WriteString(parameterTypeAnnotation.Type.ID())
	}

	return fmt.Sprintf(
		"((%s):%s)",
		parameters.String(),
		t.ReturnTypeAnnotation,
	)
}

func (t *FunctionType) Equal(other Type) bool {
	otherFunction, ok := other.(*FunctionType)
	if !ok {
		return false
	}

	if len(t.ParameterTypeAnnotations) != len(otherFunction.ParameterTypeAnnotations) {
		return false
	}

	for i, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		otherParameterType := otherFunction.ParameterTypeAnnotations[i]
		if !parameterTypeAnnotation.Equal(otherParameterType) {
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

	for _, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		if parameterTypeAnnotation.Type.IsInvalidType() {
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
					ParameterTypeAnnotations: []*TypeAnnotation{{Type: &IntegerType{}}},
					ReturnTypeAnnotation:     &TypeAnnotation{Type: integerType},
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
				ParameterTypeAnnotations: []*TypeAnnotation{{Type: &IntegerType{}}},
				ReturnTypeAnnotation:     &TypeAnnotation{Type: addressType},
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
	ConstructorParameterTypeAnnotations []*TypeAnnotation
	NestedTypes                         map[string]Type
	ContainerType                       Type
}

func (*CompositeType) isType() {}

func (t *CompositeType) String() string {
	return t.Identifier
}

func (t *CompositeType) ID() string {
	if t.Location == nil {
		return t.Identifier
	}

	return fmt.Sprintf("%s.%s", t.Location.ID(), t.Identifier)
}

func (t *CompositeType) Equal(other Type) bool {
	otherStructure, ok := other.(*CompositeType)
	if !ok {
		return false
	}

	return otherStructure.Kind == t.Kind &&
		otherStructure.Identifier == t.Identifier
}

func (t *CompositeType) exportAsPointer() types.Type {
	switch t.Kind {
	case common.CompositeKindStructure:
		return types.StructPointer{TypeName: t.Identifier}
	case common.CompositeKindResource:
		return types.ResourcePointer{TypeName: t.Identifier}
	}
	panic(fmt.Sprintf("cannot convert type %v of unknown kind %v", t, t.Kind))
}

func (t *CompositeType) Export(program *ast.Program, variable *Variable) types.Type {
	// this type is exported as a field or parameter type, not main definition
	if variable == nil {
		return t.exportAsPointer()
	}

	convert := func() types.Composite {

		compositeDeclaration := func() *ast.CompositeDeclaration {
			for _, cd := range program.CompositeDeclarations() {
				if cd.Identifier.Identifier == variable.Identifier &&
					cd.Identifier.Pos == *variable.Pos {
					return cd
				}
			}
			panic(fmt.Sprintf("cannot find type %v declaration in AST tree", t))
		}()

		fields := make([]types.Field, 0, len(t.Members))

		for identifer, field := range t.Members {
			typ := field.TypeAnnotation.Type.(ExportableType).Export(program, nil)

			fields = append(fields, types.Field{
				Identifier: identifer,
				Type:       typ,
			})
		}

		parameters := make([]types.Parameter, len(t.ConstructorParameterTypeAnnotations))

		// TODO: For now we have only one initializer, so we just assume this here
		// as this is post SEMA we really hope AST list of params matches SEMA type one
		for i, parameter := range compositeDeclaration.Members.Initializers()[0].ParameterList.Parameters {
			semaType := t.ConstructorParameterTypeAnnotations[i].Type

			parameters[i] = types.Parameter{
				Label:      parameter.Label,
				Identifier: parameter.Identifier.Identifier,
				Type:       semaType.(ExportableType).Export(program, nil),
			}
		}

		return types.Composite{
			Identifier:   t.Identifier,
			Fields:       fields,
			Initializers: [][]types.Parameter{parameters},
		}.WithID(t.ID())
	}

	switch t.Kind {
	case common.CompositeKindStructure:
		return types.Struct{
			Composite: convert(),
		}
	case common.CompositeKindResource:
		return types.Resource{
			Composite: convert(),
		}
	}
	panic(fmt.Sprintf("cannot convert type %v of unknown kind %v", t, t.Kind))
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
		Location:                            t.Location,
		Identifier:                          t.Identifier,
		CompositeKind:                       t.Kind,
		Members:                             t.Members,
		InitializerParameterTypeAnnotations: t.ConstructorParameterTypeAnnotations,
		ContainerType:                       t.ContainerType,
		NestedTypes:                         t.NestedTypes,
	}
}

// AccountType

type AccountType struct{}

func (*AccountType) isType() {}

func (*AccountType) String() string {
	return "Account"
}

func (*AccountType) ID() string {
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

func (*PublicAccountType) ID() string {
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
			len(member.ArgumentLabels) != len(functionType.ParameterTypeAnnotations) {

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
	InitializerParameterTypeAnnotations []*TypeAnnotation
	ContainerType                       Type
	NestedTypes                         map[string]Type
}

func (*InterfaceType) isType() {}

func (t *InterfaceType) String() string {
	return t.Identifier
}

func (t *InterfaceType) ID() string {
	if t.Location == nil {
		return t.Identifier
	}

	return fmt.Sprintf("%s.%s", t.Location.ID(), t.Identifier)
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

func (t *DictionaryType) Export(program *ast.Program, variable *Variable) types.Type {
	return wrapVariable(types.Dictionary{
		KeyType:     t.KeyType.(ExportableType).Export(program, nil),
		ElementType: t.ValueType.(ExportableType).Export(program, nil),
	}, variable)
}

func (t *DictionaryType) ID() string {
	return fmt.Sprintf(
		"{%s:%s}",
		t.KeyType.ID(),
		t.ValueType.ID(),
	)
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
					ParameterTypeAnnotations: NewTypeAnnotations(
						t.KeyType,
						t.ValueType,
					),
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
					ParameterTypeAnnotations: NewTypeAnnotations(
						t.KeyType,
					),
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

func (t *DictionaryType) ElementType(isAssignment bool) Type {
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

func (t *StorageType) ID() string {
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

func (t *StorageType) ElementType(indexingType Type, isAssignment bool) Type {
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

func (t *ReferencesType) ID() string {
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

func (t *ReferencesType) ElementType(indexingType Type, isAssignment bool) Type {
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

// EventType

type EventType struct {
	Location                            ast.Location
	Identifier                          string
	Fields                              []EventFieldType
	ConstructorParameterTypeAnnotations []*TypeAnnotation
}

func (*EventType) isType() {}

func (t *EventType) Export(program *ast.Program, variable *Variable) types.Type {

	var parameters []types.Parameter

	if program != nil && variable != nil {
		eventDeclaration := func() *ast.EventDeclaration {
			for _, fn := range program.EventDeclarations() {
				if fn.Identifier.Identifier == variable.Identifier && fn.Identifier.Pos == *variable.Pos {
					return fn
				}
			}

			panic(fmt.Sprintf("cannot find type %v declaration in AST tree", t))
		}()

		parameterList := eventDeclaration.ParameterList.Parameters

		parameters = make([]types.Parameter, len(t.Fields))

		for i, field := range t.Fields {
			identifier := field.Identifier
			typ := field.Type.(ExportableType).Export(program, nil)

			parameters[i] = types.Parameter{
				Label:      parameterList[i].Label,
				Identifier: identifier,
				Type:       typ,
			}
		}
	}

	fields := make([]types.Field, len(t.Fields))

	for i, field := range t.Fields {
		identifier := field.Identifier
		typ := field.Type.(ExportableType).Export(program, nil)

		fields[i] = types.Field{
			Identifier: identifier,
			Type:       typ,
		}
	}

	return types.Event{
		Identifier:  t.Identifier,
		Fields:      fields,
		Initializer: parameters,
	}.WithID(t.ID())
}

func (t *EventType) String() string {
	var fields strings.Builder
	for i, field := range t.Fields {
		if i > 0 {
			fields.WriteString(", ")
		}
		fields.WriteString(field.String())
	}

	return fmt.Sprintf("%s(%s)", t.Identifier, fields.String())
}

func (t *EventType) ID() string {
	if t.Location == nil {
		return t.Identifier
	}

	return fmt.Sprintf("%s.%s", t.Location.ID(), t.Identifier)
}

func (t *EventType) Equal(other Type) bool {
	otherEvent, ok := other.(*EventType)
	if !ok {
		return false
	}

	if t.Identifier != otherEvent.Identifier {
		return false
	}

	if len(t.Fields) != len(otherEvent.Fields) {
		return false
	}

	for i, field := range t.Fields {
		otherField := otherEvent.Fields[i]
		if !field.Equal(otherField) {
			return false
		}
	}

	return true
}

func (t *EventType) ConstructorFunctionType() *SpecialFunctionType {
	return &SpecialFunctionType{
		FunctionType: &FunctionType{
			ParameterTypeAnnotations: t.ConstructorParameterTypeAnnotations,
			ReturnTypeAnnotation:     NewTypeAnnotation(t),
		},
	}
}

func (*EventType) IsResourceType() bool {
	return false
}

func (*EventType) IsInvalidType() bool {
	return false
}

type EventFieldType struct {
	Identifier string
	Type       Type
}

func (t EventFieldType) String() string {
	return fmt.Sprintf("%s: %s", t.Identifier, t.Type)
}

func (t EventFieldType) Equal(other EventFieldType) bool {
	return t.Identifier == other.Identifier &&
		t.Type.Equal(other.Type)
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

func (t *ReferenceType) ID() string {
	if t.Type == nil {
		return "reference"
	}
	return fmt.Sprintf("&%s", t.Type.ID())
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

func (*AddressType) ID() string {
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
		// optionals are covariant: T? <: U? if T <: U
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

		// references are covariant: &T <: &U if T <: U
		return IsSubType(typedSubType.Type, typedSuperType.Type)
	}

	// TODO: functions

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
	Members                         map[string]*Member
	prepareParameterTypeAnnotations []*TypeAnnotation
}

func (t *TransactionType) EntryPointFunctionType() *FunctionType {
	return t.PrepareFunctionType().InvocationFunctionType()
}

func (t *TransactionType) PrepareFunctionType() *SpecialFunctionType {
	return &SpecialFunctionType{
		FunctionType: &FunctionType{
			ParameterTypeAnnotations: t.prepareParameterTypeAnnotations,
			ReturnTypeAnnotation:     NewTypeAnnotation(&VoidType{}),
		},
	}
}

func (*TransactionType) ExecuteFunctionType() *SpecialFunctionType {
	return &SpecialFunctionType{
		FunctionType: &FunctionType{
			ParameterTypeAnnotations: []*TypeAnnotation{},
			ReturnTypeAnnotation:     NewTypeAnnotation(&VoidType{}),
		},
	}
}

func (*TransactionType) isType() {}

func (*TransactionType) String() string {
	return "Transaction"
}

func (*TransactionType) ID() string {
	return "Transaction"
}

func (*TransactionType) Equal(other Type) bool {
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
