package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestIsResourceType_AnyStructNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &AnyStructType{},
	}

	assert.False(t, ty.IsResourceType())
}

func TestIsResourceType_AnyResourceNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &AnyResourceType{},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_ResourceNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &CompositeType{
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
		position := ast.Position{
			Offset: 1, Line: 2, Column: 3,
		}
		identifier := "my_structure"

		ty := &CompositeType{
			Location:     nil,
			Identifier:   identifier,
			Kind:         common.CompositeKindStructure,
			Conformances: nil,
			Members: map[string]*Member{
				"fieldA": {
					ContainerType: nil,
					Access:        0,
					Identifier: ast.Identifier{
						Identifier: "fieldA",
						Pos:        position,
					},
					Type:            &IntType{},
					DeclarationKind: 0,
					VariableKind:    ast.VariableKindVariable,
					ArgumentLabels:  nil,
				},
			},
			ConstructorParameterTypeAnnotations: []*TypeAnnotation{
				{
					Move: false,
					Type: &Int8Type{},
				},
			},
		}

		program := &ast.Program{
			Declarations: []ast.Declaration{
				&ast.CompositeDeclaration{
					Identifier: ast.Identifier{
						Identifier: identifier, Pos: position,
					},
					Members: &ast.Members{
						SpecialFunctions: []*ast.SpecialFunctionDeclaration{
							{
								DeclarationKind: common.DeclarationKindInitializer,
								FunctionDeclaration: &ast.FunctionDeclaration{
									Identifier: ast.Identifier{},
									ParameterList: &ast.ParameterList{
										Parameters: []*ast.Parameter{
											{
												Label: "labelA",
												Identifier: ast.Identifier{
													Identifier: "fieldA",
													Pos:        ast.Position{},
												},
											},
										},
									},
								},
							},
						},
					},
					Range: ast.Range{},
				},
			},
		}

		variable := &Variable{
			Identifier:      identifier,
			DeclarationKind: common.DeclarationKindStructure,
			Pos:             &position,
		}

		ex := ty.Export(program, variable)

		assert.IsType(t, types.Struct{}, ex)
		s := ex.(types.Struct)

		assert.Equal(t, identifier, s.Identifier)
		require.Len(t, s.Fields, 1)

		require.Contains(t, s.Fields, "fieldA")

		assert.IsType(t, types.Int{}, s.Fields["fieldA"].Type)
	})

	t.Run("string", func(t *testing.T) {

		ty := &StringType{}

		ex := ty.Export(nil, nil)

		assert.IsType(t, types.String{}, ex)
	})

	t.Run("events", func(t *testing.T) {

		position := ast.Position{
			Offset: 2, Line: 1, Column: 37,
		}

		ty := &EventType{
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
		}

		program := &ast.Program{
			Declarations: []ast.Declaration{
				&ast.EventDeclaration{
					Identifier: ast.Identifier{
						Identifier: "MagicEvent",
						Pos:        position,
					},
					ParameterList: &ast.ParameterList{
						Parameters: []*ast.Parameter{
							{
								Label: "magic_caster",
								Identifier: ast.Identifier{
									Identifier: "who",
								},
							},
							{
								Label: "magic_place",
								Identifier: ast.Identifier{
									Identifier: "where",
								},
							},
						},
					},
				},
			},
		}

		variable := Variable{
			Identifier: "MagicEvent",
			Pos:        &position,
		}

		ex := ty.Export(program, &variable)

		assert.IsType(t, types.Event{}, ex)

		event := ex.(types.Event)

		require.Len(t, event.Fields, 2)

		// for fields in event, order matters
		assert.Equal(t, "where", event.Fields[1].Identifier)
		assert.Equal(t, "magic_place", event.Fields[1].Label)
	})

}
