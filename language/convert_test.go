package language_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language"
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

const testLocation = ast.StringLocation("test")

func TestConvert(t *testing.T) {

	t.Run("structs", func(t *testing.T) {
		position := ast.Position{
			Offset: 1, Line: 2, Column: 3,
		}
		identifier := "my_structure"

		ty := &sema.CompositeType{
			Location:     testLocation,
			Identifier:   identifier,
			Kind:         common.CompositeKindStructure,
			Conformances: nil,
			Members: map[string]*sema.Member{
				"fieldA": {
					ContainerType: nil,
					Access:        0,
					Identifier: ast.Identifier{
						Identifier: "fieldA",
						Pos:        position,
					},
					TypeAnnotation:  &sema.TypeAnnotation{Type: &sema.IntType{}},
					DeclarationKind: 0,
					VariableKind:    ast.VariableKindVariable,
					ArgumentLabels:  nil,
				},
			},
			ConstructorParameters: []*sema.Parameter{
				{
					TypeAnnotation: &sema.TypeAnnotation{
						Type: &sema.Int8Type{},
					},
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

		variable := &sema.Variable{
			Identifier:      identifier,
			DeclarationKind: common.DeclarationKindStructure,
			Pos:             &position,
		}

		ex, err := language.Convert(ty, program, variable)
		assert.NoError(t, err)

		assert.IsType(t, language.StructType{}, ex)
		s := ex.(language.StructType)

		assert.Equal(t, identifier, s.Identifier)
		require.Len(t, s.Fields, 1)

		assert.Equal(t, "fieldA", s.Fields[0].Identifier)
		assert.IsType(t, language.IntType{}, s.Fields[0].Type)
	})

	t.Run("string", func(t *testing.T) {
		ty := &sema.StringType{}

		ex, err := language.Convert(ty, nil, nil)
		assert.NoError(t, err)

		assert.IsType(t, language.StringType{}, ex)
	})

	t.Run("events", func(t *testing.T) {
		position := ast.Position{
			Offset: 2, Line: 1, Column: 37,
		}

		ty := &sema.CompositeType{
			Location:   testLocation,
			Kind:       common.CompositeKindEvent,
			Identifier: "MagicEvent",
			Members:    map[string]*sema.Member{},
			ConstructorParameters: []*sema.Parameter{
				{
					TypeAnnotation: &sema.TypeAnnotation{
						Type: &sema.StringType{},
					},
				},
				{
					TypeAnnotation: &sema.TypeAnnotation{
						Type: &sema.IntType{},
					},
				},
			},
		}

		ty.Members["who"] = &sema.Member{
			ContainerType:   ty,
			Identifier:      ast.Identifier{Identifier: "who"},
			TypeAnnotation:  sema.NewTypeAnnotation(&sema.StringType{}),
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
		}

		ty.Members["where"] = &sema.Member{
			ContainerType:   ty,
			Identifier:      ast.Identifier{Identifier: "where"},
			TypeAnnotation:  sema.NewTypeAnnotation(&sema.IntType{}),
			DeclarationKind: common.DeclarationKindField,
			VariableKind:    ast.VariableKindConstant,
		}

		program := &ast.Program{
			Declarations: []ast.Declaration{
				&ast.CompositeDeclaration{
					CompositeKind: common.CompositeKindEvent,
					Identifier: ast.Identifier{
						Identifier: "MagicEvent",
						Pos:        position,
					},
					Members: &ast.Members{
						SpecialFunctions: []*ast.SpecialFunctionDeclaration{
							{
								DeclarationKind: common.DeclarationKindInitializer,
								FunctionDeclaration: &ast.FunctionDeclaration{
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
						},
					},
				},
			},
		}

		variable := &sema.Variable{
			Identifier: "MagicEvent",
			Pos:        &position,
		}

		ex, err := language.Convert(ty, program, variable)
		assert.NoError(t, err)

		assert.IsType(t, language.EventType{}, ex)

		event := ex.(language.EventType)

		require.Len(t, event.Fields, 2)
		assert.Equal(t, "where", event.Fields[0].Identifier)
		assert.IsType(t, language.IntType{}, event.Fields[0].Type)

		assert.Equal(t, "who", event.Fields[1].Identifier)
		assert.IsType(t, language.StringType{}, event.Fields[1].Type)

		require.Len(t, event.Initializers[0], 2)
		assert.Equal(t, "magic_caster", event.Initializers[0][0].Label)
		assert.Equal(t, "magic_place", event.Initializers[0][1].Label)
	})
}
