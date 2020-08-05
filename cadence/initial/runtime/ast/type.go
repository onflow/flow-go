/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

type Type interface {
	HasPosition
	fmt.Stringer
	isType()
}

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
	Size *IntegerExpression
	Range
}

func (*ConstantSizedType) isType() {}

func (t *ConstantSizedType) String() string {
	return fmt.Sprintf("[%s; %s]", t.Type, t.Size)
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
	Range
}

func (*RestrictedType) isType() {}

func (t *RestrictedType) String() string {
	var builder strings.Builder
	if t.Type != nil {
		builder.WriteString(t.Type.String())
	}
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
