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
	Type Type
	Range
}

func (*VariableSizedType) isType() {}

func (t *VariableSizedType) String() string {
	return fmt.Sprintf("[%s]", t.Type)
}

// ConstantSizedType is a constant sized array type

type ConstantSizedType struct {
	Type Type
	Size int
	Range
}

func (*ConstantSizedType) isType() {}

func (t *ConstantSizedType) String() string {
	return fmt.Sprintf("[%s; %d]", t.Type, t.Size)
}

// DictionaryType

type DictionaryType struct {
	KeyType   Type
	ValueType Type
	Range
}

func (*DictionaryType) isType() {}

func (t *DictionaryType) String() string {
	return fmt.Sprintf("{%s: %s}", t.KeyType, t.ValueType)
}

// FunctionType

type FunctionType struct {
	ParameterTypeAnnotations []*TypeAnnotation
	ReturnTypeAnnotation     *TypeAnnotation
	Range
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
