package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckFailableCastingWithResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test %[3]s %[4]s T%[5]s as? @T
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidFailableResourceDowncastOutsideOptionalBindingError{}, errs[0])

				// TODO: add support for non-Any types in failable casting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[1])

			case common.CompositeKindStructure, common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

				// TODO: add support for non-Any types in failable casting

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[1])

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])
				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionDeclarationParameterWithResourceAnnotation(t *testing.T) {

	for _, kind := range common.AllCompositeKinds {

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      fun test(r: @T) {
                          %[3]s r
                      }
                    `,
					kind.Keyword(),
					body,
					kind.DestructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindStructure,
				common.CompositeKindContract,
				common.CompositeKindEvent:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionDeclarationParameterWithoutResourceAnnotation(t *testing.T) {

	for _, kind := range common.AllCompositeKinds {

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      fun test(r: T) {
                          %[3]s r
                      }
                    `,
					kind.Keyword(),
					body,
					kind.DestructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])

			case common.CompositeKindStructure,
				common.CompositeKindContract,
				common.CompositeKindEvent:

				require.NoError(t, err)

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionDeclarationReturnTypeWithResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      fun test(): @T {
                          return %[3]s %[4]s T%[5]s
                      }
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindStructure:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidMoveError{}, errs[1])

			case common.CompositeKindEvent:

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionDeclarationReturnTypeWithoutResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      fun test(): T {
                          return %[3]s %[4]s T%[5]s
                      }
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])

			case common.CompositeKindStructure,
				common.CompositeKindContract:

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckVariableDeclarationWithResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test: @T %[3]s %[4]s T%[5]s
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindStructure:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidMoveError{}, errs[1])

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])
				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckVariableDeclarationWithoutResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test: T %[3]s %[4]s T%[5]s
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])

			case common.CompositeKindStructure,
				common.CompositeKindContract:

				require.NoError(t, err)

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFieldDeclarationWithResourceAnnotation(t *testing.T) {

	for _, kind := range common.CompositeKindsWithBody {

		t.Run(kind.Keyword(), func(t *testing.T) {

			destructor := ""
			if kind == common.CompositeKindResource {
				destructor = `
                  destroy() {
                      destroy self.t
                  }
                `
			}

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T {}

                      %[1]s U {
                          let t: @T
                          init(t: @T) {
                              self.t %[2]s t
                          }

                          %[3]s
                      }
                    `,
					kind.Keyword(),
					kind.TransferOperator(),
					destructor,
				),
			)

			switch kind {
			case common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindStructure:
				errs := ExpectCheckerErrors(t, err, 2)

				// NOTE: one invalid resource annotation error for field, one for parameter

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[1])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 3)

				// NOTE: one invalid resource annotation error for field, one for parameter

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[1])
				assert.IsType(t, &sema.InvalidMoveError{}, errs[2])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFieldDeclarationWithoutResourceAnnotation(t *testing.T) {
	for _, kind := range common.CompositeKindsWithBody {
		t.Run(kind.Keyword(), func(t *testing.T) {

			destructor := ""
			if kind == common.CompositeKindResource {
				destructor = `
                  destroy() {
                      destroy self.t
                  }
                `
			}

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T {}

                      %[1]s U {
                          let t: T
                          init(t: T) {
                              self.t %[2]s t
                          }

                          %[3]s
                      }
                    `,
					kind.Keyword(),
					kind.TransferOperator(),
					destructor,
				),
			)

			switch kind {
			case common.CompositeKindResource:
				// NOTE: one missing resource annotation error for field, one for parameter

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[1])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidMoveError{}, errs[0])

			case common.CompositeKindStructure:
				require.NoError(t, err)

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionExpressionParameterWithResourceAnnotation(t *testing.T) {

	for _, kind := range common.AllCompositeKinds {

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test = fun (r: @T) {
                          %[3]s r
                      }
                    `,
					kind.Keyword(),
					body,
					kind.DestructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindStructure,
				common.CompositeKindContract,
				common.CompositeKindEvent:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionExpressionParameterWithoutResourceAnnotation(t *testing.T) {

	for _, kind := range common.AllCompositeKinds {

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test = fun (r: T) {
                          %[3]s r
                      }
                    `,
					kind.Keyword(),
					body,
					kind.DestructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindResource:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])

			case common.CompositeKindStructure,
				common.CompositeKindContract,
				common.CompositeKindEvent:

				require.NoError(t, err)

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionExpressionReturnTypeWithResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test = fun (): @T {
                          return %[3]s %[4]s T%[5]s
                      }
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindStructure:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

			case common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidMoveError{}, errs[1])

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionExpressionReturnTypeWithoutResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test = fun (): T {
                          return %[3]s %[4]s T%[5]s
                      }
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])

			case common.CompositeKindStructure,
				common.CompositeKindContract:

				require.NoError(t, err)

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionTypeParameterWithResourceAnnotation(t *testing.T) {

	for _, kind := range common.AllCompositeKinds {

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test: ((@T): Void) = fun (r: @T) {
                          %[3]s r
                      }
                    `,
					kind.Keyword(),
					body,
					kind.DestructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindStructure,
				common.CompositeKindContract,
				common.CompositeKindEvent:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionTypeParameterWithoutResourceAnnotation(t *testing.T) {

	for _, kind := range common.AllCompositeKinds {

		body := "{}"
		if kind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test: ((T): Void) = fun (r: T) {
                          %[3]s r
                      }
                    `,
					kind.Keyword(),
					body,
					kind.DestructionKeyword(),
				),
			)

			switch kind {
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])

			case common.CompositeKindStructure,
				common.CompositeKindContract,
				common.CompositeKindEvent:

				require.NoError(t, err)

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test: ((): @T) = fun (): @T {
                          return %[3]s %[4]s T%[5]s
                      }
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				require.NoError(t, err)

			case common.CompositeKindStructure,
				common.CompositeKindContract:

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithoutResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		if compositeKind == common.CompositeKindContract {
			continue
		}

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test: ((): T) = fun (): T {
                          return %[3]s %[4]s T%[5]s
                      }
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.MoveOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])

			case common.CompositeKindStructure,
				common.CompositeKindContract:

				require.NoError(t, err)

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFailableCastingWithoutResourceAnnotation(t *testing.T) {

	for _, compositeKind := range common.AllCompositeKinds {

		body := "{}"
		if compositeKind == common.CompositeKindEvent {
			body = "()"
		}

		t.Run(compositeKind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %[1]s T %[2]s

                      let test %[3]s %[4]s T%[5]s as? T
                    `,
					compositeKind.Keyword(),
					body,
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					constructorArguments(compositeKind),
				),
			)

			switch compositeKind {
			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])

				assert.IsType(t, &sema.InvalidFailableResourceDowncastOutsideOptionalBindingError{}, errs[1])

				// TODO: add support for non-Any types in failable downcasting
				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[2])

			case common.CompositeKindStructure,
				common.CompositeKindContract:

				// TODO: add support for non-Any types in failable casting

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckUnaryMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun foo(x: @X): @X {
          return <-x
      }

      fun bar() {
          let x <- foo(x: <-create X())
          destroy x
      }
    `)

	require.NoError(t, err)

}

func TestCheckImmediateDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          destroy create X()
      }
    `)

	require.NoError(t, err)
}

func TestCheckIndirectDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceCreationWithoutCreate(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- X()
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingCreateError{}, errs[0])

}

func TestCheckInvalidDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun test() {
          destroy X()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDestructionError{}, errs[0])
}

func TestCheckUnaryCreateAndDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          var x <- create X()
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckUnaryCreateAndDestroyWithInitializer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let x: Int
          init(x: Int) {
              self.x = x
          }
      }

      fun test() {
          var x <- create X(x: 1)
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidUnaryCreateAndDestroyWithWrongInitializerArguments(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let x: Int
          init(x: Int) {
              self.x = x
          }
      }

      fun test() {
          var x <- create X(y: true)
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	assert.IsType(t, &sema.IncorrectArgumentLabelError{}, errs[1])
}

func TestCheckInvalidUnaryCreateStruct(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun test() {
          create X()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidConstructionError{}, errs[0])
}

func TestCheckInvalidCreateImportedResource(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      pub resource R {}
	`)

	require.Nil(t, err)

	_, err = ParseAndCheckWithOptions(t,
		`
          import R from "imported"

          pub fun test() {
              destroy create R()
          }
        `,
		ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				return checker.Program, nil
			},
		},
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.CreateImportedResourceError{}, errs[0])
}

func TestCheckInvalidResourceLoss(t *testing.T) {
	t.Run("UnassignedResource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource X {}

            fun test() {
                create X()
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	})

	t.Run("ImmediateMemberAccess", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource Foo {
                fun bar(): Int {
                    return 42
                }
            }

            fun test() {
                let x = (create Foo()).bar()
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	})

	t.Run("ImmediateMemberAccessFunctionInvocation", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource Foo {
                fun bar(): Int {
                    return 42
                }
            }

            fun createResource(): @Foo {
                return <-create Foo()
            }

            fun test() {
                let x = createResource().bar()
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	})

	t.Run("ImmediateIndexing", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource Foo {}

            fun test() {
                let x <- [<-create Foo(), <-create Foo()][0]
                destroy x
            }
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
		assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[1])
	})

	t.Run("ImmediateIndexingFunctionInvocation", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource Foo {}

            fun test() {
                let x <- makeFoos()[0]
                destroy x
            }

            fun makeFoos(): @[Foo] {
                return <-[
                    <-create Foo(),
                    <-create Foo()
                ]
            }
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
		assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[1])
	})
}

func TestCheckResourceReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): @X {
          return <-create X()
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceReturnMissingMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): @X {
          return create X()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
}

func TestCheckInvalidResourceReturnMissingMoveInvalidReturnType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): Y {
          return create X()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[1])
}

func TestCheckInvalidNonResourceReturnWithMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun test(): X {
          return <-X()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckResourceArgument(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun foo(_ x: @X) {
          destroy x
      }

      fun bar() {
          foo(<-create X())
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceArgumentMissingMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun foo(_ x: @X) {
          destroy x
      }

      fun bar() {
          foo(create X())
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
}

func TestCheckInvalidResourceArgumentMissingMoveInvalidParameterType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun foo(_ x: Y) {}

      fun bar() {
          foo(create X())
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[1])
}

func TestCheckInvalidNonResourceArgumentWithMove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      fun foo(_ x: X) {}

      fun bar() {
          foo(<-X())
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckResourceVariableDeclarationTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let y <- x
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceVariableDeclarationIncorrectTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x = create X()
      let y = x
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[1])
}

func TestCheckInvalidNonResourceVariableDeclarationMoveTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let y <- x
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
}

func TestCheckInvalidResourceAssignmentTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          var x2 <- create X()
          destroy x2
          x2 <- x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[0])
}

func TestCheckInvalidResourceAssignmentIncorrectTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          var x2 <- create X()
          destroy x2
          x2 = x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
	assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[1])
}

func TestCheckInvalidNonResourceAssignmentMoveTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      fun test() {
        var x2 = X()
        x2 <- x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
}

func TestCheckInvalidResourceLossThroughVariableDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
        let x <- create X()
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceLossThroughVariableDeclarationAfterCreation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          let y <- x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceLossThroughAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          var x <- create X()
          let y <- create X()
          x <- y
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckResourceMoveThroughReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): @X {
          let x <- create X()
          return <-x
      }
    `)

	require.NoError(t, err)
}

func TestCheckResourceMoveThroughArgumentPassing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          absorb(<-x)
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceUseAfterMoveToFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          absorb(<-x)
          absorb(<-x)
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceUseAfterMoveToVariable(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          let y <- x
          let z <- x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])

	// NOTE: still two resource losses reported for `y` and `z`

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])

	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
}

func TestCheckInvalidResourceFieldUseAfterMoveToVariable(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test(): Int {
          let x <- create X(id: 1)
          absorb(<-x)
          return x.id
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckResourceUseAfterMoveInIfStatementThenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
              absorb(<-x)
          }
          absorb(<-x)
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckResourceUseInIfStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
              absorb(<-x)
          } else {
              absorb(<-x)
          }
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckResourceUseInNestedIfStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
              if 2 > 1 {
                  absorb(<-x)
              }
          } else {
              absorb(<-x)
          }
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	require.NoError(t, err)
}

////

func TestCheckInvalidResourceUseAfterIfStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): @X {
          let x <- create X()
          if 1 > 2 {
              absorb(<-x)
          } else {
              absorb(<-x)
          }
          return <-x
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])

	assert.ElementsMatch(t,
		errs[0].(*sema.ResourceUseAfterInvalidationError).Invalidations,
		[]sema.ResourceInvalidation{
			{
				Kind:     sema.ResourceInvalidationKindMove,
				StartPos: ast.Position{Offset: 164, Line: 9, Column: 23},
				EndPos:   ast.Position{Offset: 164, Line: 9, Column: 23},
			},
			{
				Kind:     sema.ResourceInvalidationKindMove,
				StartPos: ast.Position{Offset: 119, Line: 7, Column: 23},
				EndPos:   ast.Position{Offset: 119, Line: 7, Column: 23},
			},
		},
	)
}

func TestCheckInvalidResourceLossAfterDestroyInIfStatementThenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          if 1 > 2 {
             destroy x
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceLossAndUseAfterDestroyInIfStatementThenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          if 1 > 2 {
             destroy x
          }
          x.id
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckResourceMoveIntoArray(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [<-x]
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceMoveIntoArrayMissingMoveOperation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [x]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
}

func TestCheckInvalidNonResourceMoveIntoArray(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let xs = [<-x]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckInvalidUseAfterResourceMoveIntoArray(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [<-x, <-x]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckResourceMoveIntoDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {"x": <-x}
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceMoveIntoDictionaryMissingMoveOperation(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {"x": x}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
}

func TestCheckInvalidNonResourceMoveIntoDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let xs = {"x": <-x}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckInvalidUseAfterResourceMoveIntoDictionary(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {
          "x": <-x,
          "x2": <-x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidUseAfterResourceMoveIntoDictionaryAsKey(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {<-x: <-x}
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.InvalidDictionaryKeyTypeError{}, errs[1])
}

func TestCheckInvalidResourceUseAfterMoveInWhileStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          while true {
              destroy x
          }
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckResourceUseInWhileStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              x.id
          }
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceUseInWhileStatementAfterDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              x.id
              destroy x
          }
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[2])
}

func TestCheckInvalidResourceUseInWhileStatementAfterDestroyAndLoss(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          while true {
              destroy x
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceUseInNestedWhileStatementAfterDestroyAndLoss1(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              while true {
                  x.id
                  destroy x
              }
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
}

func TestCheckInvalidResourceUseInNestedWhileStatementAfterDestroyAndLoss2(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              while true {
                  x.id
              }
              destroy x
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
}

func TestCheckResourceUseInNestedWhileStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int
          init(id: Int) {
              self.id = id
          }
      }

      fun test() {
          let x <- create X(id: 1)
          while true {
              while true {
                  x.id
              }
          }
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceLossThroughReturn(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let x <- create X()
          return
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	assert.IsType(t, &sema.UnreachableStatementError{}, errs[1])
}

func TestCheckInvalidResourceLossThroughReturnInIfStatementThrenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(y: Int) {
          let x <- create X()
          if y == 42 {
              return
          }
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceLossThroughReturnInIfStatementBranches(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(y: Int) {
          let x <- create X()
          if y == 42 {
              absorb(<-x)
              return
          } else {
              return
          }
          destroy x
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	assert.IsType(t, &sema.UnreachableStatementError{}, errs[1])
}

func TestCheckResourceWithMoveAndReturnInIfStatementThenAndDestroyInElse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(y: Int) {
          let x <- create X()
          if y == 42 {
              absorb(<-x)
              return
          } else {
              destroy x
          }
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckResourceWithMoveAndReturnInIfStatementThenBranch(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(y: Int) {
          let x <- create X()
          if y == 42 {
              absorb(<-x)
              return
          }
          destroy x
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	require.NoError(t, err)
}

func TestCheckResourceNesting(t *testing.T) {
	interfacePossibilities := []bool{true, false}

	for _, innerCompositeKind := range common.AllCompositeKinds {

		// Don't test contract fields/parameters: contracts can't be passed by value
		if innerCompositeKind == common.CompositeKindContract {
			continue
		}

		for _, innerIsInterface := range interfacePossibilities {

			if !innerCompositeKind.SupportsInterfaces() && innerIsInterface {
				continue
			}

			for _, outerCompositeKind := range common.CompositeKindsWithBody {
				for _, outerIsInterface := range interfacePossibilities {

					if !outerCompositeKind.SupportsInterfaces() && outerIsInterface {
						continue
					}

					testResourceNesting(
						t,
						innerCompositeKind,
						innerIsInterface,
						outerCompositeKind,
						outerIsInterface,
					)
				}
			}
		}
	}
}

func testResourceNesting(
	t *testing.T,
	innerCompositeKind common.CompositeKind,
	innerIsInterface bool,
	outerCompositeKind common.CompositeKind,
	outerIsInterface bool,
) {
	innerInterfaceKeyword := ""
	if innerIsInterface {
		innerInterfaceKeyword = "interface"
	}

	outerInterfaceKeyword := ""
	if outerIsInterface {
		outerInterfaceKeyword = "interface"
	}

	testName := fmt.Sprintf(
		"%s %s/%s %s",
		innerCompositeKind.Keyword(),
		innerInterfaceKeyword,
		outerCompositeKind.Keyword(),
		outerInterfaceKeyword,
	)

	t.Run(testName, func(t *testing.T) {

		// Prepare the initializer, if needed.
		// `outerCompositeKind` is the container composite kind.
		// If it is concrete, i.e. not an interface, it needs an initializer.

		initializer := ""
		if !outerIsInterface {
			initializer = fmt.Sprintf(
				`
                  init(t: %[1]sT) {
                      self.t %[2]s t
                  }
                `,
				innerCompositeKind.Annotation(),
				innerCompositeKind.TransferOperator(),
			)
		}

		destructor := ""
		if !outerIsInterface &&
			outerCompositeKind == common.CompositeKindResource &&
			innerCompositeKind == common.CompositeKindResource {

			destructor = `
              destroy() {
                  destroy self.t
              }
            `
		}

		innerBody := "{}"
		if innerCompositeKind == common.CompositeKindEvent {
			innerBody = "()"
		}

		// Prepare the full program defining an empty composite,
		// and a second composite which contains the first

		program := fmt.Sprintf(
			`
              %[1]s %[2]s T %[3]s

              %[4]s %[5]s U {
                  let t: %[6]sT
                  %[7]s
                  %[8]s
              }
            `,
			innerCompositeKind.Keyword(),
			innerInterfaceKeyword,
			innerBody,
			outerCompositeKind.Keyword(),
			outerInterfaceKeyword,
			innerCompositeKind.Annotation(),
			initializer,
			destructor,
		)

		_, err := ParseAndCheck(t, program)

		switch outerCompositeKind {
		case common.CompositeKindStructure,
			common.CompositeKindContract:

			switch innerCompositeKind {
			case common.CompositeKindStructure,
				common.CompositeKindEvent:

				require.NoError(t, err)

			case common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceFieldError{}, errs[0])

			default:
				panic(errors.NewUnreachableError())
			}

		case common.CompositeKindResource:
			require.NoError(t, err)

		default:
			panic(errors.NewUnreachableError())
		}
	})
}

// TestCheckResourceInterfaceConformance tests the check
// of conformance of resources to resource interfaces.
//
func TestCheckResourceInterfaceConformance(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface X {
          fun test()
      }

      resource Y: X {
          fun test() {}
      }
    `)

	require.NoError(t, err)
}

// TestCheckInvalidResourceInterfaceConformance tests the check
// of conformance of resources to resource interfaces.
//
func TestCheckInvalidResourceInterfaceConformance(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface X {
          fun test()
      }

      resource Y: X {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
}

// TestCheckResourceInterfaceUseAsType tests if a resource interface
// can be used as a type, and if a resource is a subtype of the interface
// if it conforms to it.
//
func TestCheckResourceInterfaceUseAsType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface X {}

      resource Y: X {}

      let x: @X <- create Y()
    `)

	require.NoError(t, err)
}

func TestCheckResourceArrayIndexing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {
          var bar: Int
          init(bar: Int) {
              self.bar = bar
          }
      }

      fun test(): Int {
        let foo <- create Foo(bar: 1)
        let foos <- [<-[<-foo]]
        let bar = foos[0][0].bar
        destroy foos
        return bar
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceLossReturnResourceAndMemberAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {
          let id: Int

          init(id: Int) {
              self.id = id
          }
      }

      fun test(): Int {
          return createX().id
      }

      fun createX(): @X {
          return <-create X(id: 1)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceLossAfterMoveThroughArrayIndexing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs <- [<-create X()]
          foo(x: <-xs[0])
      }

      fun foo(x: @X) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)
	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceLossThroughFunctionResultAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {
          var bar: Int
          init(bar: Int) {
              self.bar = bar
          }
      }

      fun createFoo(): @Foo {
          return <- create Foo(bar: 1)
      }

      fun test(): Int {
          return createFoo().bar
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

// TestCheckResourceInterfaceDestruction tests if resources
// can be passed to resource interface parameters,
// and if resource interfaces can be destroyed.
//
func TestCheckResourceInterfaceDestruction(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface X {}

      resource Y: X {}

      fun foo(x: @X) {
          destroy x
      }

      fun bar() {
          foo(x: <-create Y())
      }
    `)

	require.NoError(t, err)
}

// TestCheckInvalidResourceFieldMoveThroughVariableDeclaration tests if resources nested
// as a field in another resource cannot be moved out of the containing resource through
// a variable declaration. This would partially invalidate the containing resource
//
func TestCheckInvalidResourceFieldMoveThroughVariableDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {}

      resource Bar {
          let foo: @Foo

          init(foo: @Foo) {
              self.foo <- foo
          }

          destroy() {
              destroy self.foo
          }
      }

      fun test(): @[Foo] {
          let foo <- create Foo()
          let bar <- create Bar(foo: <-foo)
          let foo2 <- bar.foo
          let foo3 <- bar.foo
          destroy bar
          return <-[<-foo2, <-foo3]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[0])
	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[1])
}

// TestCheckInvalidResourceFieldMoveThroughParameter tests if resources nested
// as a field in another resource cannot be moved out of the containing resource
// by passing the field as an argument to a function. This would partially invalidate
// the containing resource
//
func TestCheckInvalidResourceFieldMoveThroughParameter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {}

      resource Bar {
          let foo: @Foo

          init(foo: @Foo) {
              self.foo <- foo
          }

          destroy() {
              destroy self.foo
          }
      }

      fun identity(_ foo: @Foo): @Foo {
          return <-foo
      }

      fun test(): @[Foo] {
          let foo <- create Foo()
          let bar <- create Bar(foo: <-foo)
          let foo2 <- identity(<-bar.foo)
          let foo3 <- identity(<-bar.foo)
          destroy bar
          return <-[<-foo2, <-foo3]
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[0])
	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[1])
}

func TestCheckInvalidResourceFieldMoveSelf(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Y {}

      resource X {

          var y: @Y

          init() {
              self.y <- create Y()
          }

          fun test() {
             absorb(<-self.y)
          }

          destroy() {
              destroy self.y
          }
      }

      fun absorb(_ y: @Y) {
          destroy y
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[0])
}

func TestCheckInvalidResourceFieldUseAfterDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Y {}

      resource X {

          var y: @Y

          init() {
              self.y <- create Y()
          }

          destroy() {
              destroy self.y
              destroy self.y
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckResourceArrayAppend(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @[X] <- []
          xs.append(<-create X())
          destroy xs
      }
    `)

	require.NoError(t, err)
}

func TestCheckResourceArrayInsert(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @[X] <- []
          xs.insert(at: 0, <-create X())
          destroy xs
      }
    `)

	require.NoError(t, err)
}

func TestCheckResourceArrayRemove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @[X] <- [<-create X()]
          let x <- xs.remove(at: 0)
          destroy x
          destroy xs
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceArrayRemoveResourceLoss(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @[X] <- [<-create X()]
          xs.remove(at: 0)
          destroy xs
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckResourceArrayRemoveFirst(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @[X] <- [<-create X()]
          let x <- xs.removeFirst()
          destroy x
          destroy xs
      }
    `)

	require.NoError(t, err)
}

func TestCheckResourceArrayRemoveLast(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @[X] <- [<-create X()]
          let x <- xs.removeLast()
          destroy x
          destroy xs
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceArrayContains(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @[X] <- [<-create X()]
          xs.contains(<-create X())
          destroy xs
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidResourceArrayMemberError{}, errs[0])
	assert.IsType(t, &sema.NotEquatableTypeError{}, errs[1])
}

func TestCheckResourceArrayLength(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): Int {
          let xs: @[X] <- [<-create X()]
          let count = xs.length
          destroy xs
          return count
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceArrayConcat(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @[X] <- [<-create X()]
          let xs2 <- [<-create X()]
          let xs3 <- xs.concat(<-xs2)
          destroy xs
          destroy xs3
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidResourceArrayMemberError{}, errs[0])
}

func TestCheckResourceDictionaryRemove(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @{String: X} <- {"x1": <-create X()}
          let x <- xs.remove(key: "x1")
          destroy x
          destroy xs
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceDictionaryRemoveResourceLoss(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @{String: X} <- {"x1": <-create X()}
          xs.remove(key: "x1")
          destroy xs
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckResourceDictionaryInsert(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @{String: X} <- {}
          let old <- xs.insert(key: "x1", <-create X())
          destroy old
          destroy xs
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceDictionaryInsertResourceLoss(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs: @{String: X} <- {}
          xs.insert(key: "x1", <-create X())
          destroy xs
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckResourceDictionaryLength(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): Int {
          let xs: @{String: X} <- {"x1": <-create X()}
          let count = xs.length
          destroy xs
          return count
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceDictionaryKeys(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs <- {<-create X(): "x1"}
          let keys <- xs.keys
          destroy keys
          destroy xs
      }
    `)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.InvalidDictionaryKeyTypeError{}, errs[0])
	assert.IsType(t, &sema.InvalidResourceDictionaryMemberError{}, errs[1])
	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[2])
}

func TestCheckInvalidResourceDictionaryValues(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs <- {"x1": <-create X()}
          let values <- xs.values
          destroy values
          destroy xs
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidResourceDictionaryMemberError{}, errs[0])
	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[1])
}

func TestCheckInvalidResourceLossAfterMoveThroughDictionaryIndexing(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs <- {"x": <-create X()}
          foo(x: <-xs["x"])
      }

      fun foo(x: @X?) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)
	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
         var x <- create X()
         x <-> create X()
         destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[0])
}

func TestCheckInvalidResourceConstantResourceFieldSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {}

      resource Bar {
          let foo: @Foo

          init(foo: @Foo) {
              self.foo <- foo
          }

          destroy() {
              destroy self.foo
          }
      }

      fun test() {
          let foo <- create Foo()
          let bar <- create Bar(foo: <-foo)
          var foo2 <- create Foo()
          bar.foo <-> foo2
          destroy bar
          destroy foo2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
}

func TestCheckResourceVariableResourceFieldSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource Foo {}

      resource Bar {
          var foo: @Foo

          init(foo: @Foo) {
              self.foo <- foo
          }

          destroy() {
              destroy self.foo
          }
      }

      fun test() {
          let foo <- create Foo()
          let bar <- create Bar(foo: <-foo)
          var foo2 <- create Foo()
          bar.foo <-> foo2
          destroy bar
          destroy foo2
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceFieldDestroy(t *testing.T) {

	_, err := ParseAndCheck(t, `
     resource Foo {}

     resource Bar {
         var foo: @Foo

         init(foo: @Foo) {
             self.foo <- foo
         }

         destroy() {
             destroy self.foo
         }
     }

     fun test() {
         let foo <- create Foo()
         let bar <- create Bar(foo: <-foo)
         destroy bar.foo
     }
   `)

	errs := ExpectCheckerErrors(t, err, 2)

	// TODO: maybe have dedicated error

	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckResourceParameterInInterfaceNoResourceLossError(t *testing.T) {

	declarationKinds := []common.DeclarationKind{
		common.DeclarationKindInitializer,
		common.DeclarationKindFunction,
	}

	for _, compositeKind := range common.CompositeKindsWithBody {
		for _, declarationKind := range declarationKinds {
			for _, hasCondition := range []bool{true, false} {

				testName := fmt.Sprintf(
					"%s %s/hasCondition=%v",
					compositeKind,
					declarationKind,
					hasCondition,
				)

				innerDeclaration := ""
				switch declarationKind {
				case common.DeclarationKindInitializer:
					innerDeclaration = declarationKind.Keywords()

				case common.DeclarationKindFunction:
					innerDeclaration = fmt.Sprintf("%s test", declarationKind.Keywords())
				}

				functionBlock := ""
				if hasCondition {
					functionBlock = "{ pre { true } }"
				}

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheck(t, fmt.Sprintf(
						`
                          resource X {}

                          %[1]s interface Y {

                              // Should not result in a resource loss error
                              %[2]s(from: @X) %[3]s
                          }
                        `,
						compositeKind.Keyword(),
						innerDeclaration,
						functionBlock,
					))

					require.NoError(t, err)
				})
			}
		}
	}
}

func TestCheckResourceFieldUseAndDestruction(t *testing.T) {

	_, err := ParseAndCheck(t, `
     resource interface RI {}

     resource R {
         var ris: @{String: RI}

         init(_ ri: @RI) {
             self.ris <- {"first": <-ri}
         }

         pub fun use() {
            let ri <- self.ris.remove(key: "first")
            absorb(<-ri)
         }

         destroy() {
             destroy self.ris
         }
     }

     fun absorb(_ ri: @RI?) {
         destroy ri
     }
   `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceMethodBinding(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test(): ((@R): Void) {
          let rs <- [<-create R()]
          let append = rs.append
          destroy rs
          return append
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceMethodBindingError{}, errs[0])
}

func TestCheckInvalidResourceMethodCall(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let rs <- [<-create R()]
          rs.append(<-create R())
          destroy rs
      }
    `)

	require.NoError(t, err)
}

func TestCheckResourceOptionalBinding(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let maybeR: @R? <- create R()
          if let r <- maybeR {
              destroy r
          } else {
              destroy maybeR
          }
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceOptionalBindingResourceLossInThen(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let maybeR: @R? <- create R()
          if let r <- maybeR {
              // resource loss of r
          } else {
              destroy maybeR
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceOptionalBindingResourceLossInElse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let maybeR: @R? <- create R()
          if let r <- maybeR {
              destroy r
          } else {
              // resource loss of maybeR
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceOptionalBindingResourceUseAfterInvalidationInThen(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let maybeR: @R? <- create R()
          if let r <- maybeR {
              destroy r
              destroy maybeR
          } else {
              destroy maybeR
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceOptionalBindingResourceUseAfterInvalidationAfterBranches(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let maybeR: @R? <- create R()
          if let r <- maybeR {
              destroy r
          } else {
              destroy maybeR
          }
          f(<-maybeR)
      }

      fun f(_ r: @R?) {
          destroy r
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckResourceOptionalBindingFailableCast(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @RI <- create R()
             if let r <- ri as? @R {
                 destroy r
             } else {
                 destroy ri
             }
         }
    `)

	// TODO: remove once supported

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])
}

func TestCheckInvalidResourceOptionalBindingFailableCastResourceUseAfterInvalidationInThen(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @RI <- create R()
             if let r <- ri as? @R {
                 destroy r
                 destroy ri
             } else {
                 destroy ri
             }
         }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	// TODO: remove once supported
	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckInvalidResourceOptionalBindingFailableCastResourceUseAfterInvalidationAfterBranches(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @RI <- create R()
             if let r <- ri as? @R {
                 destroy r
             }
             destroy ri
         }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	// TODO: remove once supported
	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckInvalidResourceOptionalBindingFailableCastResourceLossMissingElse(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @RI <- create R()
             if let r <- ri as? @R {
                 destroy r
             }
         }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	// TODO: remove once supported
	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])

	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
}

func TestCheckInvalidResourceOptionalBindingFailableCastResourceUseAfterInvalidationAfterThen(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @RI <- create R()
             if let r <- ri as? @R {
                 destroy r
             }
             destroy ri
         }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	// TODO: remove once supported
	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[0])

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[1])
}

func TestCheckInvalidResourceFailableCastOutsideOptionalBinding(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @RI <- create R()
             let r <- ri as? @R
             destroy r
         }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidFailableResourceDowncastOutsideOptionalBindingError{}, errs[0])

	// TODO: remove once supported
	assert.IsType(t, &sema.UnsupportedTypeError{}, errs[1])
}

func TestCheckInvalidUnaryMoveAndCopyTransfer(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let r = <- create R()
          destroy r
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
}

func TestCheckInvalidResourceSelfMoveToFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `

      resource X {

          fun test() {
              absorb(<-self)
          }
      }

      fun absorb(_ x: @X) {
          destroy x
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceSelfMoveInVariableDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `

      resource X {

          fun test() {
              let x <- self
              destroy x
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceSelfDestruction(t *testing.T) {

	_, err := ParseAndCheck(t, `

      resource X {

          fun test() {
              destroy self
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceSelfMoveReturnFromFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `

      resource X {

          fun test(): @X {
              return <-self
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceSelfMoveIntoArrayLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `

      resource X {

          fun test(): @[X] {
              return <-[<-self]
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceSelfMoveIntoDictionaryLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `

      resource X {

          fun test(): @{String: X} {
              return <-{"self": <-self}
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceSelfMoveSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `

      resource X {

          fun test() {
              var x: @X? <- nil
              let oldX <- x <- self
              destroy x
              destroy oldX
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSelfInvalidationError{}, errs[0])
}

func TestCheckResourceCreationAndInvalidationInLoop(t *testing.T) {

	_, err := ParseAndCheck(t, `

      resource X {}

      fun loop() {
          var i = 0
          while i < 10 {
              let x <- create X()
              destroy x
              i = i + 1
          }
      }
    `)

	require.NoError(t, err)
}
