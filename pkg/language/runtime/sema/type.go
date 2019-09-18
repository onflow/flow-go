package sema

import (
	"encoding/gob"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

type Type interface {
	isType()
	String() string
	Equal(other Type) bool
	IsResourceType() bool
}

// AnyType represents the top type
type AnyType struct{}

func (*AnyType) isType() {}

func (*AnyType) String() string {
	return "Any"
}

func (*AnyType) Equal(other Type) bool {
	_, ok := other.(*AnyType)
	return ok
}

func (*AnyType) IsResourceType() bool {
	return false
}

// NeverType represents the bottom type
type NeverType struct{}

func (*NeverType) isType() {}

func (*NeverType) String() string {
	return "Never"
}

func (*NeverType) Equal(other Type) bool {
	_, ok := other.(*NeverType)
	return ok
}

func (*NeverType) IsResourceType() bool {
	return false
}

// VoidType represents the void type
type VoidType struct{}

func (*VoidType) isType() {}

func (*VoidType) String() string {
	return "Void"
}

func (*VoidType) Equal(other Type) bool {
	_, ok := other.(*VoidType)
	return ok
}

func (*VoidType) IsResourceType() bool {
	return false
}

// InvalidType represents a type that is invalid.
// It is the result of type checking failing and
// can't be expressed in programs.
//
type InvalidType struct{}

func (*InvalidType) isType() {}

func (*InvalidType) String() string {
	return "<<invalid>>"
}

func (*InvalidType) Equal(other Type) bool {
	_, ok := other.(*InvalidType)
	return ok
}

func (*InvalidType) IsResourceType() bool {
	return false
}

func isInvalidType(ty Type) bool {
	_, ok := ty.(*InvalidType)
	return ok
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
	return fmt.Sprintf("%s?", t.Type.String())
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

// BoolType represents the boolean type
type BoolType struct{}

func (*BoolType) isType() {}

func (*BoolType) String() string {
	return "Bool"
}

func (*BoolType) Equal(other Type) bool {
	_, ok := other.(*BoolType)
	return ok
}

func (*BoolType) IsResourceType() bool {
	return false
}

// CharacterType represents the character type

type CharacterType struct{}

func (*CharacterType) isType() {}

func (*CharacterType) String() string {
	return "Character"
}

func (*CharacterType) Equal(other Type) bool {
	_, ok := other.(*CharacterType)
	return ok
}

func (*CharacterType) IsResourceType() bool {
	return false
}

// StringType represents the string type
type StringType struct{}

func (*StringType) isType() {}

func (*StringType) String() string {
	return "String"
}

func (*StringType) Equal(other Type) bool {
	_, ok := other.(*StringType)
	return ok
}

func (*StringType) IsResourceType() bool {
	return false
}

var stringMembers = map[string]*Member{
	"length": {
		Type:          &IntType{},
		VariableKind:  ast.VariableKindConstant,
		IsInitialized: true,
	},
	"concat": {
		Type: &FunctionType{
			ParameterTypes: []Type{
				&StringType{},
			},
			ReturnType: &StringType{},
		},
	},
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

func (*IntegerType) Equal(other Type) bool {
	_, ok := other.(*IntegerType)
	return ok
}

func (*IntegerType) IsResourceType() bool {
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

func (*IntType) Equal(other Type) bool {
	_, ok := other.(*IntType)
	return ok
}

func (*IntType) IsResourceType() bool {
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

func (*Int8Type) Equal(other Type) bool {
	_, ok := other.(*Int8Type)
	return ok
}

func (*Int8Type) IsResourceType() bool {
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

func (*Int16Type) Equal(other Type) bool {
	_, ok := other.(*Int16Type)
	return ok
}

func (*Int16Type) IsResourceType() bool {
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

func (*Int32Type) Equal(other Type) bool {
	_, ok := other.(*Int32Type)
	return ok
}

func (*Int32Type) IsResourceType() bool {
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

func (*Int64Type) Equal(other Type) bool {
	_, ok := other.(*Int64Type)
	return ok
}

func (*Int64Type) IsResourceType() bool {
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

func (*UInt8Type) Equal(other Type) bool {
	_, ok := other.(*UInt8Type)
	return ok
}

func (*UInt8Type) IsResourceType() bool {
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

func (*UInt16Type) Equal(other Type) bool {
	_, ok := other.(*UInt16Type)
	return ok
}

func (*UInt16Type) IsResourceType() bool {
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

func (*UInt32Type) Equal(other Type) bool {
	_, ok := other.(*UInt32Type)
	return ok
}

func (*UInt32Type) IsResourceType() bool {
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

func (*UInt64Type) Equal(other Type) bool {
	_, ok := other.(*UInt64Type)
	return ok
}

func (*UInt64Type) IsResourceType() bool {
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
	Type
	isArrayType()
	elementType() Type
}

var arrayMembers = map[string]*Member{
	"length": {
		Type:          &IntType{},
		VariableKind:  ast.VariableKindConstant,
		IsInitialized: true,
	},
}

func getArrayMember(ty ArrayType, field string) *Member {
	switch field {
	case "append":
		return &Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypes: []Type{
					ty.elementType(),
				},
				ReturnType: &VoidType{},
			},
			IsInitialized: true,
		}
	case "concat":
		return &Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypes: []Type{ty},
				ReturnType:     ty,
			},
			IsInitialized: true,
		}
	case "insert":
		return &Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypes: []Type{
					&IntegerType{},
					ty.elementType(),
				},
				ReturnType: &VoidType{},
			},
			IsInitialized:  true,
			ArgumentLabels: []string{"at"},
		}
	case "remove":
		return &Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypes: []Type{&IntegerType{}},
				ReturnType:     ty.elementType(),
			},
			IsInitialized:  true,
			ArgumentLabels: []string{"at"},
		}
	case "removeFirst":
		return &Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypes: []Type{},
				ReturnType:     ty.elementType(),
			},
			IsInitialized: true,
		}
	case "removeLast":
		return &Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypes: []Type{},
				ReturnType:     ty.elementType(),
			},
			IsInitialized: true,
		}
	case "contains":
		return &Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypes: []Type{ty.elementType()},
				ReturnType:     &BoolType{},
			},
			IsInitialized: true,
		}
	default:
		return arrayMembers[field]
	}
}

// VariableSizedType is a variable sized array type
type VariableSizedType struct {
	Type
}

func (*VariableSizedType) isType()      {}
func (*VariableSizedType) isArrayType() {}

func (t *VariableSizedType) elementType() Type {
	return t.Type
}

func (t *VariableSizedType) String() string {
	return ArrayTypeToString(t)
}

func (t *VariableSizedType) Equal(other Type) bool {
	otherArray, ok := other.(*VariableSizedType)
	if !ok {
		return false
	}

	return t.Type.Equal(otherArray.Type)
}

func (t *VariableSizedType) IsResourceType() bool {
	return t.Type.IsResourceType()
}

// ConstantSizedType is a constant sized array type
type ConstantSizedType struct {
	Type
	Size int
}

func (*ConstantSizedType) isType()      {}
func (*ConstantSizedType) isArrayType() {}

func (t *ConstantSizedType) elementType() Type {
	return t.Type
}

func (t *ConstantSizedType) String() string {
	return ArrayTypeToString(t)
}

func (t *ConstantSizedType) Equal(other Type) bool {
	otherArray, ok := other.(*ConstantSizedType)
	if !ok {
		return false
	}

	return t.Type.Equal(otherArray.Type) &&
		t.Size == otherArray.Size
}

func (t *ConstantSizedType) IsResourceType() bool {
	return t.Type.IsResourceType()
}

// ArrayTypeToString

func ArrayTypeToString(arrayType ArrayType) string {
	var arraySuffixes strings.Builder
	var currentType Type = arrayType
	currentTypeIsArrayType := true
	for currentTypeIsArrayType {
		switch arrayType := currentType.(type) {
		case *ConstantSizedType:
			_, err := fmt.Fprintf(&arraySuffixes, "[%d]", arrayType.Size)
			if err != nil {
				panic(&errors.UnreachableError{})
			}
			currentType = arrayType.Type
		case *VariableSizedType:
			arraySuffixes.WriteString("[]")
			currentType = arrayType.Type
		default:
			currentTypeIsArrayType = false
		}
	}

	baseType := currentType.String()
	return baseType + arraySuffixes.String()
}

// FunctionType

type FunctionType struct {
	ParameterTypes        []Type
	ReturnType            Type
	Apply                 func([]Type) Type
	RequiredArgumentCount *int
}

func (*FunctionType) isType() {}

func (t *FunctionType) String() string {
	var parameters strings.Builder
	for i, parameter := range t.ParameterTypes {
		if i > 0 {
			parameters.WriteString(", ")
		}
		parameters.WriteString(parameter.String())
	}

	return fmt.Sprintf("((%s): %s)", parameters.String(), t.ReturnType.String())
}

func (t *FunctionType) Equal(other Type) bool {
	otherFunction, ok := other.(*FunctionType)
	if !ok {
		return false
	}

	if len(t.ParameterTypes) != len(otherFunction.ParameterTypes) {
		return false
	}

	for i, parameterType := range t.ParameterTypes {
		otherParameterType := otherFunction.ParameterTypes[i]
		if !parameterType.Equal(otherParameterType) {
			return false
		}
	}

	return t.ReturnType.Equal(otherFunction.ReturnType)
}

func (*FunctionType) IsResourceType() bool {
	return false
}

// BaseTypes

var baseTypes hamt.Map

func init() {

	typeNames := map[string]Type{
		"": &VoidType{},
	}

	types := []Type{
		&VoidType{},
		&AnyType{},
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
	}

	for _, ty := range types {
		typeName := ty.String()

		// check type is not accidentally redeclared
		if _, ok := typeNames[typeName]; ok {
			panic(&errors.UnreachableError{})
		}

		typeNames[typeName] = ty
	}

	for name, baseType := range typeNames {
		key := common.StringKey(name)
		baseTypes = baseTypes.Insert(key, baseType)
	}
}

// CompositeType

type CompositeType struct {
	Kind         common.CompositeKind
	Identifier   string
	Conformances []*InterfaceType
	Members      map[string]*Member
	// TODO: add support for overloaded initializers
	ConstructorParameterTypes []Type
}

func (*CompositeType) isType() {}

func (t *CompositeType) String() string {
	return t.Identifier
}

func (t *CompositeType) Equal(other Type) bool {
	otherStructure, ok := other.(*CompositeType)
	if !ok {
		return false
	}

	return otherStructure.Kind == t.Kind &&
		otherStructure.Identifier == t.Identifier
}

func (t *CompositeType) IsResourceType() bool {
	return t.Kind == common.CompositeKindResource
}

// Member

type Member struct {
	Type           Type
	VariableKind   ast.VariableKind
	IsInitialized  bool
	ArgumentLabels []string
}

// InterfaceType

type InterfaceType struct {
	CompositeKind             common.CompositeKind
	Identifier                string
	Members                   map[string]*Member
	InitializerParameterTypes []Type
}

func (*InterfaceType) isType() {}

func (t *InterfaceType) String() string {
	return t.Identifier
}

func (t *InterfaceType) Equal(other Type) bool {
	otherInterface, ok := other.(*InterfaceType)
	if !ok {
		return false
	}

	return otherInterface.CompositeKind == t.CompositeKind &&
		otherInterface.Identifier == t.Identifier
}

func (t *InterfaceType) IsResourceType() bool {
	return t.CompositeKind == common.CompositeKindResource
}

// InterfaceMetaType

type InterfaceMetaType struct {
	InterfaceType *InterfaceType
}

func (*InterfaceMetaType) isType() {}

func (t *InterfaceMetaType) String() string {
	return fmt.Sprintf("%s.Type", t.InterfaceType.Identifier)
}

func (t *InterfaceMetaType) Equal(other Type) bool {
	otherInterface, ok := other.(*InterfaceMetaType)
	if !ok {
		return false
	}

	return otherInterface.InterfaceType.Equal(t.InterfaceType)
}

func (*InterfaceMetaType) IsResourceType() bool {
	return false
}

// DictionaryType

type DictionaryType struct {
	KeyType   Type
	ValueType Type
}

func (*DictionaryType) isType() {}

func (t *DictionaryType) String() string {
	return fmt.Sprintf("%s[%s]", t.ValueType, t.KeyType)
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

var dictionaryMembers = map[string]*Member{
	"length": {
		Type:          &IntType{},
		VariableKind:  ast.VariableKindConstant,
		IsInitialized: true,
	},
}

func getDictionaryMember(ty *DictionaryType, field string) *Member {
	switch field {
	case "remove":
		return &Member{
			VariableKind: ast.VariableKindConstant,
			Type: &FunctionType{
				ParameterTypes: []Type{
					ty.KeyType,
				},
				ReturnType: &OptionalType{
					Type: ty.ValueType,
				},
			},
			IsInitialized:  true,
			ArgumentLabels: []string{"key"},
		}
	default:
		return dictionaryMembers[field]
	}
}

func init() {
	gob.Register(&AnyType{})
	gob.Register(&NeverType{})
	gob.Register(&VoidType{})
	gob.Register(&InvalidType{})
	gob.Register(&OptionalType{})
	gob.Register(&BoolType{})
	gob.Register(&StringType{})
	gob.Register(&IntegerType{})
	gob.Register(&IntType{})
	gob.Register(&Int8Type{})
	gob.Register(&Int16Type{})
	gob.Register(&Int32Type{})
	gob.Register(&Int64Type{})
	gob.Register(&UInt8Type{})
	gob.Register(&UInt16Type{})
	gob.Register(&UInt32Type{})
	gob.Register(&UInt64Type{})
	gob.Register(&DictionaryType{})
	gob.Register(&VariableSizedType{})
	gob.Register(&ConstantSizedType{})
	gob.Register(&FunctionType{})
	gob.Register(&CompositeType{})
	gob.Register(&InterfaceType{})
	gob.Register(&InterfaceMetaType{})
	gob.Register(&DictionaryType{})
}
