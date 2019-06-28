package ast

type Type interface {
	HasPosition
	isType()
}

// BaseType represents a base type (e.g. boolean, integer, etc.)

type BaseType struct {
	Identifier string
	Position   Position
}

func (BaseType) isType() {}

func (t BaseType) GetStartPosition() Position {
	return t.Position
}

func (t BaseType) GetEndPosition() Position {
	return t.Position
}

// VariableSizedType is a variable sized array type

type VariableSizedType struct {
	Type
	StartPosition Position
	EndPosition   Position
}

func (VariableSizedType) isType() {}

func (t VariableSizedType) GetStartPosition() Position {
	return t.StartPosition
}

func (t VariableSizedType) GetEndPosition() Position {
	return t.EndPosition
}

// ConstantSizedType is a constant sized array type

type ConstantSizedType struct {
	Type
	Size          int
	StartPosition Position
	EndPosition   Position
}

func (ConstantSizedType) isType() {}

func (t ConstantSizedType) GetStartPosition() Position {
	return t.StartPosition
}

func (t ConstantSizedType) GetEndPosition() Position {
	return t.EndPosition
}

// FunctionType

type FunctionType struct {
	ParameterTypes []Type
	ReturnType     Type
	StartPosition  Position
	EndPosition    Position
}

func (FunctionType) isType() {}

func (t FunctionType) GetStartPosition() Position {
	return t.StartPosition
}

func (t FunctionType) GetEndPosition() Position {
	return t.EndPosition
}
