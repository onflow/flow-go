package ast

type Type interface {
	isType()
}

// Int8Type represents the i8 8-byte signed integer type

type Int8Type struct{}

func (Int8Type) isType() {}

// Int16Type represents the i16 16-byte signed integer type

type Int16Type struct{}

func (Int16Type) isType() {}

// Int32Type represents the i32 32-byte signed integer type

type Int32Type struct{}

func (Int32Type) isType() {}

// Int64Type represents the i64 64-byte signed integer type

type Int64Type struct{}

func (Int64Type) isType() {}

// UInt8Type represents the u8 8-byte unsigned integer type

type UInt8Type struct{}

func (UInt8Type) isType() {}

// UInt16Type represents the u16 16-byte unsigned integer type

type UInt16Type struct{}

func (UInt16Type) isType() {}

// UInt32Type represents the u32 32-byte unsigned integer type

type UInt32Type struct{}

func (UInt32Type) isType() {}

// UInt64Type represents the u32 64-byte unsigned integer type

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
