package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

func TestConstantSizedType_String(t *testing.T) {

	ty := &ConstantSizedType{
		Type: &VariableSizedType{Type: &IntType{}},
		Size: 2,
	}

	assert.Equal(t, ty.String(), "[[Int]; 2]")
}

func TestConstantSizedType_String_OfFunctionType(t *testing.T) {

	ty := &ConstantSizedType{
		Type: &FunctionType{
			ParameterTypeAnnotations: NewTypeAnnotations(
				&Int8Type{},
			),
			ReturnTypeAnnotation: NewTypeAnnotation(
				&Int16Type{},
			),
		},
		Size: 2,
	}

	assert.Equal(t, ty.String(), "[((Int8): Int16); 2]")
}

func TestVariableSizedType_String(t *testing.T) {

	ty := &VariableSizedType{
		Type: &ConstantSizedType{
			Type: &IntType{},
			Size: 2,
		},
	}

	assert.Equal(t, ty.String(), "[[Int; 2]]")
}

func TestVariableSizedType_String_OfFunctionType(t *testing.T) {

	ty := &VariableSizedType{
		Type: &FunctionType{
			ParameterTypeAnnotations: NewTypeAnnotations(
				&Int8Type{},
			),
			ReturnTypeAnnotation: NewTypeAnnotation(
				&Int16Type{},
			),
		},
	}

	assert.Equal(t, ty.String(), "[((Int8): Int16)]")
}

func TestIsResourceType_AnyNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &AnyType{},
	}

	assert.False(t, ty.IsResourceType())
}

func TestIsResourceType_ResourceNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		&CompositeType{
			Kind: common.CompositeKindResource,
		},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_ResourceNestedInDictionary(t *testing.T) {

	ty := &DictionaryType{
		KeyType: &StringType{},
		ValueType: &VariableSizedType{
			Type: &CompositeType{
				Kind: common.CompositeKindResource,
			},
		},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_StructNestedInDictionary(t *testing.T) {

	ty := &DictionaryType{
		KeyType: &StringType{},
		ValueType: &VariableSizedType{
			Type: &CompositeType{
				Kind: common.CompositeKindStructure,
			},
		},
	}

	assert.False(t, ty.IsResourceType())
}

func Test_exportability(t *testing.T) {

	t.Run("structs", func(t *testing.T) {
		ty := &CompositeType{
			Location:     nil,
			Identifier:   "foo",
			Kind:         common.CompositeKindStructure,
			Conformances: nil,
			Members: map[string]*Member{
				"fieldA": {
					ContainerType: nil,
					Access:        0,
					Identifier: ast.Identifier{
						Identifier: "fieldA",
						Pos:        ast.Position{},
					},
					Type:            &IntType{},
					DeclarationKind: 0,
					VariableKind:    ast.VariableKindVariable,
					ArgumentLabels:  nil,
				},
			},
			ConstructorParameterTypeAnnotations: nil,
		}

		ex := ty.Export()

		assert.IsType(t, types.Struct{}, ex)
	})

	t.Run("string", func(t *testing.T) {

		ty := &StringType{}

		ex := ty.Export()

		assert.IsType(t, types.String{}, ex)
	})

	t.Run("events", func(t *testing.T) {

		ty := &EventType{
			Location:   nil,
			Identifier: "MagicEvent",
			Fields: []EventFieldType{
				{
					Identifier: "who",
					Type:       &StringType{},
				},
				{
					Identifier: "where",
					Type:       &IntType{},
				},
			},
			ConstructorParameterTypeAnnotations: nil,
		}

		assert.IsType(t, &types.Event{}, ty)

		assert.Len(t, ty.Fields, 2)

		assert.Equal(t, ty.Fields[1].Identifier, "where")
		assert.Equal(t, ty.Fields[1].Type, IntType{})
	})

}
