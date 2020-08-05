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

package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/cadence/runtime/ast"
)

func TestOptionalSubtyping(t *testing.T) {

	t.Parallel()

	t.Run("Int? <: Int?", func(t *testing.T) {
		assert.True(t,
			IsSubType(
				&OptionalType{Type: &IntType{}},
				&OptionalType{Type: &IntType{}},
			),
		)
	})

	t.Run("Int? <: Bool?", func(t *testing.T) {
		assert.False(t,
			IsSubType(
				&OptionalType{Type: &IntType{}},
				&OptionalType{Type: &BoolType{}},
			),
		)
	})

	t.Run("Int8? <: Integer?", func(t *testing.T) {
		assert.True(t,
			IsSubType(
				&OptionalType{Type: &Int8Type{}},
				&OptionalType{Type: &IntegerType{}},
			),
		)
	})
}

func TestCompositeType_ID(t *testing.T) {

	t.Parallel()

	t.Run("composite in composite", func(t *testing.T) {

		compositeInComposite :=
			&CompositeType{
				Location:   ast.StringLocation("x"),
				Identifier: "C",
				ContainerType: &CompositeType{
					Location:   ast.StringLocation("x"),
					Identifier: "B",
					ContainerType: &CompositeType{
						Location:   ast.StringLocation("x"),
						Identifier: "A",
					},
				},
			}

		assert.Equal(t, compositeInComposite.ID(), TypeID("x.A.B.C"))
	})

	t.Run("composite in interface", func(t *testing.T) {

		compositeInInterface :=
			&CompositeType{
				Location:   ast.StringLocation("x"),
				Identifier: "C",
				ContainerType: &InterfaceType{
					Location:   ast.StringLocation("x"),
					Identifier: "B",
					ContainerType: &CompositeType{
						Location:   ast.StringLocation("x"),
						Identifier: "A",
					},
				},
			}

		assert.Equal(t, compositeInInterface.ID(), TypeID("x.A.B.C"))
	})
}

func TestInterfaceType_ID(t *testing.T) {

	t.Parallel()

	t.Run("interface in composite", func(t *testing.T) {

		interfaceInComposite :=
			&InterfaceType{
				Location:   ast.StringLocation("x"),
				Identifier: "C",
				ContainerType: &CompositeType{
					Location:   ast.StringLocation("x"),
					Identifier: "B",
					ContainerType: &CompositeType{
						Location:   ast.StringLocation("x"),
						Identifier: "A",
					},
				},
			}

		assert.Equal(t, interfaceInComposite.ID(), TypeID("x.A.B.C"))
	})

	t.Run("interface in interface", func(t *testing.T) {

		interfaceInInterface :=
			&InterfaceType{
				Location:   ast.StringLocation("x"),
				Identifier: "C",
				ContainerType: &InterfaceType{
					Location:   ast.StringLocation("x"),
					Identifier: "B",
					ContainerType: &CompositeType{
						Location:   ast.StringLocation("x"),
						Identifier: "A",
					},
				},
			}

		assert.Equal(t, interfaceInInterface.ID(), TypeID("x.A.B.C"))
	})
}

func TestFunctionSubtyping(t *testing.T) {

	t.Parallel()

	t.Run("((Int): Void) <: ((AnyStruct): Void)", func(t *testing.T) {
		assert.False(t,
			IsSubType(
				&FunctionType{
					Parameters: []*Parameter{
						{
							TypeAnnotation: NewTypeAnnotation(&IntType{}),
						},
					},
				},
				&FunctionType{
					Parameters: []*Parameter{
						{
							TypeAnnotation: NewTypeAnnotation(&AnyStructType{}),
						},
					},
				},
			),
		)
	})

	t.Run("((AnyStruct): Void) <: ((Int): Void)", func(t *testing.T) {
		assert.True(t,
			IsSubType(
				&FunctionType{
					Parameters: []*Parameter{
						{
							TypeAnnotation: NewTypeAnnotation(&AnyStructType{}),
						},
					},
				},
				&FunctionType{
					Parameters: []*Parameter{
						{
							TypeAnnotation: NewTypeAnnotation(&IntType{}),
						},
					},
				},
			),
		)
	})

	t.Run("((): Int) <: ((): AnyStruct)", func(t *testing.T) {
		assert.True(t,
			IsSubType(
				&FunctionType{
					ReturnTypeAnnotation: NewTypeAnnotation(&IntType{}),
				},
				&FunctionType{
					ReturnTypeAnnotation: NewTypeAnnotation(&AnyStructType{}),
				},
			),
		)
	})

	t.Run("((): Any) <: ((): Int)", func(t *testing.T) {
		assert.False(t,
			IsSubType(
				&FunctionType{
					ReturnTypeAnnotation: NewTypeAnnotation(&AnyStructType{}),
				},
				&FunctionType{
					ReturnTypeAnnotation: NewTypeAnnotation(&IntType{}),
				},
			),
		)
	})
}
