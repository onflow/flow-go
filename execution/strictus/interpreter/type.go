package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
	"strings"
)

type Type interface {
	isType()
	String() string
}

// VoidType represents the void type

type VoidType struct{}

func (VoidType) isType() {}

func (VoidType) String() string {
	return "Void"
}

// BoolType represents the boolean type

type BoolType struct{}

func (BoolType) isType() {}

func (BoolType) String() string {
	return "Bool"
}

// IntegerType represents the super-type of all integer types

type IntegerType struct{}

func (IntegerType) isType() {}

func (IntegerType) String() string {
	return "integer"
}

// IntType represents the arbitrary-precision integer type `Int`

type IntType struct{}

func (IntType) isType() {}

func (IntType) String() string {
	return "Int"
}

// Int8Type represents the 8-bit signed integer type `Int8`

type Int8Type struct{}

func (Int8Type) isType() {}

func (Int8Type) String() string {
	return "Int8"
}

// Int16Type represents the 16-bit signed integer type `Int16`

type Int16Type struct{}

func (Int16Type) isType() {}

func (Int16Type) String() string {
	return "Int16"
}

// Int32Type represents the 32-bit signed integer type `Int32`

type Int32Type struct{}

func (Int32Type) isType() {}

func (Int32Type) String() string {
	return "Int32"
}

// Int64Type represents the 64-bit signed integer type `Int64`

type Int64Type struct{}

func (Int64Type) isType() {}

func (Int64Type) String() string {
	return "Int64"
}

// UInt8Type represents the 8-bit unsigned integer type `UInt8`

type UInt8Type struct{}

func (UInt8Type) isType() {}

func (UInt8Type) String() string {
	return "UInt8"
}

// UInt16Type represents the 16-bit unsigned integer type `UInt16`

type UInt16Type struct{}

func (UInt16Type) isType() {}

func (UInt16Type) String() string {
	return "UInt16"
}

// UInt32Type represents the 32-bit unsigned integer type `UInt32`

type UInt32Type struct{}

func (UInt32Type) isType() {}

func (UInt32Type) String() string {
	return "UInt32"
}

// UInt64Type represents the 64-bit unsigned integer type `UInt64`

type UInt64Type struct{}

func (UInt64Type) isType() {}

func (UInt64Type) String() string {
	return "UInt64"
}

// ArrayType

type ArrayType interface {
	Type
	isArrayType()
}

// VariableSizedType is a variable sized array type

type VariableSizedType struct {
	Type
}

func (VariableSizedType) isType()      {}
func (VariableSizedType) isArrayType() {}

func (t VariableSizedType) String() string {
	return ArrayTypeToString(t)
}

// ConstantSizedType is a constant sized array type

type ConstantSizedType struct {
	Type
	Size int
}

func (ConstantSizedType) isType()      {}
func (ConstantSizedType) isArrayType() {}

func (t ConstantSizedType) String() string {
	return ArrayTypeToString(t)
}

// ArrayTypeToString

func ArrayTypeToString(arrayType ArrayType) string {
	var arraySuffixes strings.Builder
	var currentType Type = arrayType
	currentTypeIsArrayType := true
	for currentTypeIsArrayType {
		switch arrayType := currentType.(type) {
		case ConstantSizedType:
			fmt.Fprintf(&arraySuffixes, "[%d]", arrayType.Size)
			currentType = arrayType.Type
		case VariableSizedType:
			arraySuffixes.WriteString("[]")
			currentType = arrayType.Type
		default:
			currentTypeIsArrayType = false
		}
	}
	return currentType.String() + arraySuffixes.String()
}

// FunctionType

type FunctionType struct {
	ParameterTypes []Type
	ReturnType     Type
}

func (FunctionType) isType() {}

func (t FunctionType) String() string {
	var parameters strings.Builder
	for i, parameter := range t.ParameterTypes {
		if i > 0 {
			parameters.WriteString(", ")
		}
		parameters.WriteString(parameter.String())
	}

	return fmt.Sprintf("(%s) => %s", parameters.String(), t.ReturnType.String())
}

func ConvertType(t ast.Type) Type {
	switch t := t.(type) {
	case ast.BaseType:
		return ParseBaseType(t.Identifier)

	case ast.VariableSizedType:
		return VariableSizedType{
			Type: ConvertType(t.Type),
		}

	case ast.ConstantSizedType:
		return ConstantSizedType{
			Type: ConvertType(t.Type),
			Size: t.Size,
		}

	case ast.FunctionType:
		var parameterTypes []Type
		for _, parameterType := range t.ParameterTypes {
			parameterTypes = append(parameterTypes,
				ConvertType(parameterType),
			)
		}

		returnType := ConvertType(t.ReturnType)

		return FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     returnType,
		}
	default:
		panic(fmt.Sprintf("can't convert unsupported type: %#+v", t))
	}
}

var baseTypes = map[string]Type{
	"":       VoidType{},
	"Void":   VoidType{},
	"Bool":   BoolType{},
	"Int":    IntType{},
	"Int8":   Int8Type{},
	"Int16":  Int16Type{},
	"Int32":  Int32Type{},
	"Int64":  Int64Type{},
	"UInt8":  UInt8Type{},
	"UInt16": UInt16Type{},
	"UInt32": UInt32Type{},
	"UInt64": UInt64Type{},
}

func ParseBaseType(name string) Type {
	baseType, ok := baseTypes[name]
	if !ok {
		panic(fmt.Sprintf("unknown type: %s", name))
	}

	return baseType
}
