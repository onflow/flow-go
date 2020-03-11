package checker

import (
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"

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
		require.Len(t, transferType.(*sema.CompositeType).Members, 2)

		assert.Equal(t, &sema.IntType{}, transferType.(*sema.CompositeType).Members["to"].TypeAnnotation.Type)
		assert.Equal(t, &sema.IntType{}, transferType.(*sema.CompositeType).Members["from"].TypeAnnotation.Type)
	})

	t.Run("InvalidEventNonPrimitiveTypeComposite", func(t *testing.T) {

		for _, compositeKind := range common.CompositeKindsWithBody {
			if compositeKind == common.CompositeKindContract {
				continue
			}

			t.Run(compositeKind.Name(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s Token {
                            let id: String

                            init(id: String) {
                              self.id = id
                            }
                          }

                          event Transfer(token: %[2]sToken)
                        `,
						compositeKind.Keyword(),
						compositeKind.Annotation(),
					),
				)

				switch compositeKind {
				case common.CompositeKindResource:
					errs := ExpectCheckerErrors(t, err, 3)

					assert.IsType(t, &sema.ResourceLossError{}, errs[0])
					assert.IsType(t, &sema.InvalidEventParameterTypeError{}, errs[1])
					assert.IsType(t, &sema.InvalidResourceFieldError{}, errs[2])

				case common.CompositeKindStructure:
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidEventParameterTypeError{}, errs[0])

				default:
					panic(errors.NewUnreachableError())
				}
			})
		}
	})

	t.Run("PrimitiveTypedFields", func(t *testing.T) {

		validTypes := append(
			sema.AllNumberTypes,
			&sema.StringType{},
			&sema.BoolType{},
		)

		for _, ty := range validTypes {

			t.Run(ty.String(), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          event Transfer(value: %s)
                        `,
						ty.String(),
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
