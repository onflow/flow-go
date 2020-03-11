package checker

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/language/runtime/ast"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckEventDeclaration(t *testing.T) {

	t.Run("ValidEvent", func(t *testing.T) {
		checker, err := ParseAndCheck(t, `
            event Transfer(to: Int, from: Int)
        `)

		require.NoError(t, err)

		transferType := checker.GlobalTypes["Transfer"].Type

		require.IsType(t, &sema.CompositeType{}, transferType)
		transferCompositeType := transferType.(*sema.CompositeType)

		require.Len(t, transferCompositeType.Members, 2)
		assert.Equal(t, &sema.IntType{}, transferCompositeType.Members["to"].TypeAnnotation.Type)
		assert.Equal(t, &sema.IntType{}, transferCompositeType.Members["from"].TypeAnnotation.Type)
	})

	t.Run("InvalidEventNonPrimitiveParameterType", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            struct Token {
              let ID: String

              init(ID: String) {
                self.ID = ID
              }
            }

            event Transfer(token: Token)
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidEventParameterTypeError{}, errs[0])
	})

	t.Run("EventParameterType", func(t *testing.T) {

		validTypes := append(
			[]sema.Type{
				&sema.StringType{},
				&sema.CharacterType{},
				&sema.BoolType{},
				&sema.AddressType{},
			},
			sema.AllNumberTypes...,
		)

		tests := validTypes[:]

		for _, validType := range validTypes {
			tests = append(tests,
				&sema.OptionalType{Type: validType},
				&sema.VariableSizedType{Type: validType},
				&sema.ConstantSizedType{Type: validType},
				&sema.DictionaryType{KeyType: validType, ValueType: validType},
			)
		}

		for _, ty := range tests {

			t.Run(ty.String(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          event Transfer(_ value: %s)
                        `,
						ty,
					),
				)

				require.NoError(t, err)
			})
		}
	})

	t.Run("RedeclaredEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            event Transfer(to: Int, from: Int)
            event Transfer(to: Int)
		`)

		// NOTE: two redeclaration errors: one for type, one for function

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.RedeclarationError{}, errs[0])
		assert.IsType(t, &sema.RedeclarationError{}, errs[1])

	})
}

func TestCheckEmitEvent(t *testing.T) {

	t.Run("ValidEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            event Transfer(to: Int, from: Int)

            fun test() {
                emit Transfer(to: 1, from: 2)
            }
        `)

		require.NoError(t, err)
	})

	t.Run("MissingEmitStatement", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            event Transfer(to: Int, from: Int)

            fun test() {
                Transfer(to: 1, from: 2)
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])
	})

	t.Run("EmitNonEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            fun notAnEvent(): Int { return 1 }

            fun test() {
                emit notAnEvent()
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.EmitNonEventError{}, errs[0])
	})

	t.Run("EmitNotDeclared", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
            fun test() {
              emit notAnEvent()
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	})

	t.Run("EmitImported", func(t *testing.T) {
		checker, err := ParseAndCheck(t, `
            pub event Transfer(to: Int, from: Int)
        `)
		require.NoError(t, err)

		_, err = ParseAndCheckWithOptions(t, `
              import Transfer from "imported"

              pub fun test() {
                  emit Transfer(to: 1, from: 2)
              }
            `,
			ParseAndCheckOptions{
				ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
					return checker.Program, nil
				},
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.EmitImportedEventError{}, errs[0])
	})
}
