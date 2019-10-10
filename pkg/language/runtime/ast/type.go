package ast

import (
	"fmt"
	"strings"
)

// TypeAnnotation

type TypeAnnotation struct {
	Move     bool
	Type     Type
	StartPos Position
}

func (e *TypeAnnotation) String() string {
	if e.Move {
		return fmt.Sprintf("<-%s", e.Type)
	} else {
		return fmt.Sprint(e.Type)
	}
}

func (e *TypeAnnotation) StartPosition() Position {
	return e.StartPos
}

func (e *TypeAnnotation) EndPosition() Position {
	return e.Type.EndPosition()
}

// Type

type Type interface {
	HasPosition
	fmt.Stringer
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

func (t *OptionalType) String() string {
	return fmt.Sprintf("%s?", t.Type)
}

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

func (t *VariableSizedType) String() string {
	return fmt.Sprintf("[%s]", t.Type)
}

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

func (t *ConstantSizedType) String() string {
	return fmt.Sprintf("[%s; %d]", t.Type, t.Size)
}

func (t *ConstantSizedType) StartPosition() Position {
	return t.StartPos
}

func (t *ConstantSizedType) EndPosition() Position {
	return t.EndPos
}

// DictionaryType

type DictionaryType struct {
	KeyType   Type
	ValueType Type
	StartPos  Position
	EndPos    Position
}

func (*DictionaryType) isType() {}

func (t *DictionaryType) String() string {
	return fmt.Sprintf("{%s: %s}", t.KeyType, t.ValueType)
}

func (t *DictionaryType) StartPosition() Position {
	return t.StartPos
}

func (t *DictionaryType) EndPosition() Position {
	return t.EndPos
}

// FunctionType

type FunctionType struct {
	ParameterTypeAnnotations []*TypeAnnotation
	ReturnTypeAnnotation     *TypeAnnotation
	StartPos                 Position
	EndPos                   Position
}

func (*FunctionType) isType() {}

func (t *FunctionType) String() string {
	var parameters strings.Builder
	for i, parameterTypeAnnotation := range t.ParameterTypeAnnotations {
		if i > 0 {
			parameters.WriteString(", ")
		}
		parameters.WriteString(parameterTypeAnnotation.String())
	}

	return fmt.Sprintf("((%s): %s)", parameters.String(), t.ReturnTypeAnnotation.String())
}

func (t *FunctionType) StartPosition() Position {
	return t.StartPos
}

func (t *FunctionType) EndPosition() Position {
	return t.EndPos
}
