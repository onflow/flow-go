package ast

import (
	"fmt"
	"strings"
)

// TypeAnnotation

type TypeAnnotation struct {
	IsResource bool
	Type       Type
	StartPos   Position
}

func (e *TypeAnnotation) String() string {
	if e.IsResource {
		return fmt.Sprintf("@%s", e.Type)
	}
	return fmt.Sprint(e.Type)
}

func (e *TypeAnnotation) StartPosition() Position {
	return e.StartPos
}

func (e *TypeAnnotation) EndPosition() Position {
	return e.Type.EndPosition()
}

// revive:disable:redefines-builtin-id

type Type interface {
	HasPosition
	fmt.Stringer
	isType()
}

// revive:enable

// NominalType represents a named type

type NominalType struct {
	Identifier        Identifier
	NestedIdentifiers []Identifier
}

func (*NominalType) isType() {}

func (t *NominalType) String() string {
	var sb strings.Builder
	sb.WriteString(t.Identifier.String())
	for _, identifier := range t.NestedIdentifiers {
		sb.WriteRune('.')
		sb.WriteString(identifier.String())
	}
	return sb.String()
}

func (t *NominalType) StartPosition() Position {
	return t.Identifier.StartPosition()
}

func (t *NominalType) EndPosition() Position {
	nestedCount := len(t.NestedIdentifiers)
	if nestedCount == 0 {
		return t.Identifier.EndPosition()
	}
	lastIdentifier := t.NestedIdentifiers[nestedCount-1]
	return lastIdentifier.EndPosition()
}

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

// ReferenceType

type ReferenceType struct {
	Authorized bool
	Type       Type
	StartPos   Position
}

func (*ReferenceType) isType() {}

func (t *ReferenceType) String() string {
	var builder strings.Builder
	if t.Authorized {
		builder.WriteString("auth ")
	}
	builder.WriteRune('&')
	builder.WriteString(t.Type.String())
	return builder.String()
}

func (t *ReferenceType) StartPosition() Position {
	return t.StartPos
}

func (t *ReferenceType) EndPosition() Position {
	return t.Type.EndPosition()
}

// RestrictedType

type RestrictedType struct {
	Type         Type
	Restrictions []*NominalType
	EndPos       Position
}

func (*RestrictedType) isType() {}

func (t *RestrictedType) String() string {
	var builder strings.Builder
	builder.WriteString(t.Type.String())
	builder.WriteRune('{')
	for i, restriction := range t.Restrictions {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(restriction.String())
	}
	builder.WriteRune('}')
	return builder.String()
}

func (t *RestrictedType) StartPosition() Position {
	return t.Type.StartPosition()
}

func (t *RestrictedType) EndPosition() Position {
	return t.EndPos
}
