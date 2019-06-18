package ast

type Type interface {
	isType()
}

// Int32Type

type Int32Type struct{}

func (Int32Type) isType() {}

// Int64Type

type Int64Type struct{}

func (Int64Type) isType() {}

// DynamicType is a variable sized array type

type DynamicType struct {
	Type
}

func (DynamicType) isType() {}

// FixedType is a constant sized array type

type FixedType struct {
	Type
	Size int
}

func (FixedType) isType() {}

// FunctionType

// TODO: add parameter types and return type
type FunctionType struct{}

func (FunctionType) isType() {}
