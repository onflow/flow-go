package ast

type Type interface {
	isType()
}

// VoidType represents the void type

type VoidType struct{}

func (VoidType) isType() {}

// BoolType represents the boolean type

type BoolType struct{}

func (BoolType) isType() {}

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
