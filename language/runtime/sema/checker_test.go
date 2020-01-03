package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

func TestOptionalSubtyping(t *testing.T) {

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
