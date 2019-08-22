package ast

type Type interface {
	HasPosition
	isType()
}

// NominalType represents a base type (e.g. boolean, integer, etc.)

type NominalType struct {
	Identifier
}

func (*NominalType) isType() {}

// OptionalType represents am optional variant of another type

type OptionalType struct {
	Type   Type
	EndPos Position
}

func (*OptionalType) isType() {}

func (t *OptionalType) StartPosition() Position {
	return t.Type.StartPosition()
}

func (t *OptionalType) EndPosition() Position {
	return t.EndPos
}

// VariableSizedType is a variable sized array type

type VariableSizedType struct {
	Type
	StartPos Position
	EndPos   Position
}

func (*VariableSizedType) isType() {}

func (t *VariableSizedType) StartPosition() Position {
	return t.StartPos
}

func (t *VariableSizedType) EndPosition() Position {
	return t.EndPos
}

// ConstantSizedType is a constant sized array type

type ConstantSizedType struct {
	Type
	Size     int
	StartPos Position
	EndPos   Position
}

func (*ConstantSizedType) isType() {}

func (t *ConstantSizedType) StartPosition() Position {
	return t.StartPos
}

func (t *ConstantSizedType) EndPosition() Position {
	return t.EndPos
}

// FunctionType

type FunctionType struct {
	ParameterTypes []Type
	ReturnType     Type
	StartPos       Position
	EndPos         Position
}

func (*FunctionType) isType() {}

func (t *FunctionType) StartPosition() Position {
	return t.StartPos
}

func (t *FunctionType) EndPosition() Position {
	return t.EndPos
}
