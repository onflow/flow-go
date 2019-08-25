package sema

import (
	"encoding/gob"
	"fmt"
	"strings"

	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
)

type Type interface {
	isType()
	String() string
	Equal(other Type) bool
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

var stringMembers = map[string]*Member{
	"length": {
		Type:          &IntType{},
		VariableKind:  ast.VariableKindConstant,
		IsInitialized: true,
	},
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

// StructureType

type StructureType struct {
	Identifier                string
	Conformances              []*InterfaceType
	Members                   map[string]*Member
	ConstructorParameterTypes []Type
}

func (*StructureType) isType() {}

func (t *StructureType) String() string {
	return t.Identifier
}

func (t *StructureType) Equal(other Type) bool {
	otherStructure, ok := other.(*StructureType)
	if !ok {
		return false
	}

	return otherStructure.Identifier == t.Identifier
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

	return otherInterface.Identifier == t.Identifier
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

var dictionaryMembers = map[string]*Member{
	"length": {
		Type:          &IntType{},
		VariableKind:  ast.VariableKindConstant,
		IsInitialized: true,
	},
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
	gob.Register(&StructureType{})
	gob.Register(&InterfaceType{})
	gob.Register(&InterfaceMetaType{})
	gob.Register(&DictionaryType{})
}
