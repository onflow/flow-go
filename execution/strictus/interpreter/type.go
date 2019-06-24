package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	"fmt"
)

type Type interface {
	isType()
}

// VoidType represents the void type

type VoidType struct{}

func (VoidType) isType() {}

// BoolType represents the boolean type

type BoolType struct{}

func (BoolType) isType() {}

// IntType represents the arbitrary-precision integer type `Int`

type IntType struct{}

func (IntType) isType() {}

// Int8Type represents the 8-bit signed integer type `Int8`

type Int8Type struct{}

func (Int8Type) isType() {}

// Int16Type represents the 16-bit signed integer type `Int16`

type Int16Type struct{}

func (Int16Type) isType() {}

// Int32Type represents the 32-bit signed integer type `Int32`

type Int32Type struct{}

func (Int32Type) isType() {}

// Int64Type represents the 64-bit signed integer type `Int64`

type Int64Type struct{}

func (Int64Type) isType() {}

// UInt8Type represents the 8-bit unsigned integer type `UInt8`

type UInt8Type struct{}

func (UInt8Type) isType() {}

// UInt16Type represents the 16-bit unsigned integer type `UInt16`

type UInt16Type struct{}

func (UInt16Type) isType() {}

// UInt32Type represents the 32-bit unsigned integer type `UInt32`

type UInt32Type struct{}

func (UInt32Type) isType() {}

// UInt64Type represents the 64-bit unsigned integer type `UInt64`

type UInt64Type struct{}

func (UInt64Type) isType() {}

// VariableSizedType is a variable sized array type

type VariableSizedType struct {
	Type
}

func (VariableSizedType) isType() {}

// ConstantSizedType is a constant sized array type

type ConstantSizedType struct {
	Type
	Size int
}

func (ConstantSizedType) isType() {}

// FunctionType

type FunctionType struct {
	ParameterTypes []Type
	ReturnType     Type
}

func (FunctionType) isType() {}

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

func ParseBaseType(name string) Type {
	switch name {
	case "Int":
		return IntType{}
	case "Int8":
		return Int8Type{}
	case "Int16":
		return Int16Type{}
	case "Int32":
		return Int32Type{}
	case "Int64":
		return Int64Type{}
	case "UInt8":
		return UInt8Type{}
	case "UInt16":
		return UInt16Type{}
	case "UInt32":
		return UInt32Type{}
	case "UInt64":
		return UInt64Type{}
	case "", "Void":
		return VoidType{}
	case "Bool":
		return BoolType{}
	default:
		panic(fmt.Sprintf("unknown type: %s", name))
	}
}
