package ast

type Type interface {
	isType()
}

// BaseType represents a base type (e.g. boolean, integer, etc.)

type BaseType struct {
	Identifier string
}

func (BaseType) isType() {}

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
