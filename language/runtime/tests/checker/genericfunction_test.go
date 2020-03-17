package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func parseAndCheckWithTestValue(t *testing.T, code string, ty sema.Type) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t,
		code,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(map[string]sema.ValueDeclaration{
					"test": stdlib.StandardLibraryValue{
						Name:       "test",
						Type:       ty,
						Kind:       common.DeclarationKindConstant,
						IsConstant: true,
					},
				}),
			},
		},
	)
}

func TestCheckGenericFunction(t *testing.T) {

	t.Run("invalid: no type parameters, one type argument, no parameters, no arguments, no return type: not generic", func(t *testing.T) {

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test<X>() 
            `,
			&sema.FunctionType{
				Parameters:            nil,
				ReturnTypeAnnotation:  sema.NewTypeAnnotation(&sema.VoidType{}),
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidTypeArgumentsError{}, errs[0])
	})

	t.Run("valid: no type parameters, no type arguments, no parameters, no arguments, no return type", func(t *testing.T) {

		for _, variant := range []string{"", "<>"} {

			checker, err := parseAndCheckWithTestValue(t,
				fmt.Sprintf(
					`
                      let res = test%s() 
                    `,
					variant,
				),
				&sema.GenericFunctionType{
					TypeParameters: nil,
					Parameters:     nil,
					ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
						TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
					},
					RequiredArgumentCount: nil,
				},
			)

			require.NoError(t, err)

			assert.Equal(t,
				&sema.VoidType{},
				checker.GlobalValues["res"].Type,
			)
		}
	})

	t.Run("invalid: no type parameters, one type argument, no parameters, no arguments, no return type: too many type arguments", func(t *testing.T) {

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test<X>() 
            `,
			&sema.GenericFunctionType{
				TypeParameters: nil,
				Parameters:     nil,
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidTypeArgumentCountError{}, errs[0])
	})

	t.Run("invalid: one type parameter, no type argument, no parameters, no arguments: missing explicit type argument", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test() 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: nil,
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
	})

	t.Run("valid: one type parameter, one type argument, no parameters, no arguments", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		checker, err := parseAndCheckWithTestValue(t,
			`
              let res = test<Int>() 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: nil,
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		require.NoError(t, err)

		invocationExpression :=
			checker.Program.Declarations[0].(*ast.VariableDeclaration).Value.(*ast.InvocationExpression)

		typeParameterTypes := checker.Elaboration.InvocationExpressionTypeParameterTypes[invocationExpression]

		assert.IsType(t,
			&sema.IntType{},
			typeParameterTypes[typeParameter],
		)
	})

	t.Run("valid: one type parameter, no type argument, one parameter, one arguments", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		checker, err := parseAndCheckWithTestValue(t,
			`
              let res = test(1) 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: []*sema.GenericParameter{
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "value",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
				},
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		require.NoError(t, err)

		invocationExpression :=
			checker.Program.Declarations[0].(*ast.VariableDeclaration).Value.(*ast.InvocationExpression)

		typeParameterTypes := checker.Elaboration.InvocationExpressionTypeParameterTypes[invocationExpression]

		assert.IsType(t,
			&sema.IntType{},
			typeParameterTypes[typeParameter],
		)
	})

	t.Run("invalid: one type parameter, no type argument, one parameter, no argument", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test() 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: []*sema.GenericParameter{
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "value",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
				},
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.ArgumentCountError{}, errs[0])
		assert.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[1])
	})

	t.Run("invalid: one type parameter, one type argument, one parameter, one arguments: type mismatch", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test<Int>("1") 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: []*sema.GenericParameter{
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "value",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
				},
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("valid: one type parameter, one type argument, one parameter, one arguments", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test<Int>(1) 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: []*sema.GenericParameter{
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "value",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
				},
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		require.NoError(t, err)
	})

	t.Run("valid: one type parameter, no type argument, two parameters, two argument: matching argument types", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		checker, err := parseAndCheckWithTestValue(t,
			`
              let res = test(1, 2) 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: []*sema.GenericParameter{
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "first",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "second",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
				},
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		require.NoError(t, err)

		invocationExpression :=
			checker.Program.Declarations[0].(*ast.VariableDeclaration).Value.(*ast.InvocationExpression)

		typeParameterTypes := checker.Elaboration.InvocationExpressionTypeParameterTypes[invocationExpression]

		assert.IsType(t,
			&sema.IntType{},
			typeParameterTypes[typeParameter],
		)
	})

	t.Run("invalid: one type parameter, no type argument, two parameters, two argument: not matching argument types", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test(1, "2") 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: []*sema.GenericParameter{
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "first",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "second",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
				},
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("invalid: one type parameter, no type argument, no parameters, no arguments, return type", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test() 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: nil,
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeParameter: typeParameter,
				},
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
	})

	t.Run("valid: one type parameter, one type argument, no parameters, no arguments, return type", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		checker, err := parseAndCheckWithTestValue(t,
			`
              let res = test<Int>() 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: nil,
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeParameter: typeParameter,
				},
				RequiredArgumentCount: nil,
			},
		)

		require.NoError(t, err)

		invocationExpression :=
			checker.Program.Declarations[0].(*ast.VariableDeclaration).Value.(*ast.InvocationExpression)

		typeParameterTypes := checker.Elaboration.InvocationExpressionTypeParameterTypes[invocationExpression]

		assert.IsType(t,
			&sema.IntType{},
			typeParameterTypes[typeParameter],
		)

		assert.IsType(t,
			&sema.IntType{},
			checker.GlobalValues["res"].Type,
		)
	})

	t.Run("valid: one type parameter, one type argument, one parameter, one argument, return type", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: nil,
		}

		checker, err := parseAndCheckWithTestValue(t,
			`
              let res = test(1) 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: []*sema.GenericParameter{
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "value",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
				},
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeParameter: typeParameter,
				},
				RequiredArgumentCount: nil,
			},
		)

		require.NoError(t, err)

		invocationExpression :=
			checker.Program.Declarations[0].(*ast.VariableDeclaration).Value.(*ast.InvocationExpression)

		typeParameterTypes := checker.Elaboration.InvocationExpressionTypeParameterTypes[invocationExpression]

		assert.IsType(t,
			&sema.IntType{},
			typeParameterTypes[typeParameter],
		)

		assert.IsType(t,
			&sema.IntType{},
			checker.GlobalValues["res"].Type,
		)
	})

	t.Run("valid: one type parameter with type bound, one type argument, no parameters, no arguments", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: &sema.NumberType{},
		}

		checker, err := parseAndCheckWithTestValue(t,
			`
              let res = test<Int>() 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: nil,
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		require.NoError(t, err)

		invocationExpression :=
			checker.Program.Declarations[0].(*ast.VariableDeclaration).Value.(*ast.InvocationExpression)

		typeParameterTypes := checker.Elaboration.InvocationExpressionTypeParameterTypes[invocationExpression]

		assert.IsType(t,
			&sema.IntType{},
			typeParameterTypes[typeParameter],
		)
	})

	t.Run("invalid: one type parameter with type bound, one type argument, no parameters, no arguments: bound not satisfied", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: &sema.NumberType{},
		}

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test<String>() 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: nil,
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("invalid: one type parameter with type bound, no type argument, one parameter, one argument: bound not satisfied", func(t *testing.T) {

		typeParameter := &sema.TypeParameter{
			Name: "T",
			Type: &sema.NumberType{},
		}

		_, err := parseAndCheckWithTestValue(t,
			`
              let res = test("test") 
            `,
			&sema.GenericFunctionType{
				TypeParameters: []*sema.TypeParameter{
					typeParameter,
				},
				Parameters: []*sema.GenericParameter{
					{
						Label:      sema.ArgumentLabelNotRequired,
						Identifier: "value",
						TypeAnnotation: &sema.GenericTypeAnnotation{
							TypeParameter: typeParameter,
						},
					},
				},
				ReturnTypeAnnotation: &sema.GenericTypeAnnotation{
					TypeAnnotation: sema.NewTypeAnnotation(&sema.VoidType{}),
				},
				RequiredArgumentCount: nil,
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

}
