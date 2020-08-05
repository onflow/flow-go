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

package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/sema"
	. "github.com/onflow/cadence/runtime/tests/utils"
)

func TestCheckFailableCastingWithResourceAnnotation(t *testing.T) {

	t.Parallel()

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
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidFailableResourceDowncastOutsideOptionalBindingError{}, errs[0])

			case common.CompositeKindStructure, common.CompositeKindContract:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[1])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

// NOTE: variable type instead of function parameter
func TestCheckFunctionTypeParameterWithoutResourceAnnotation(t *testing.T) {

	t.Parallel()

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

                      let test: ((T): Void) = fun (r: %[3]sT) {
                          %[4]s r
                      }
                    `,
					kind.Keyword(),
					body,
					kind.Annotation(),
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

	t.Parallel()

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

				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[1])

			case common.CompositeKindEvent:
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidEventUsageError{}, errs[1])
				assert.IsType(t, &sema.InvalidResourceAnnotationError{}, errs[2])

			default:
				panic(errors.NewUnreachableError())
			}
		})
	}
}

func TestCheckFunctionTypeReturnTypeWithoutResourceAnnotation(t *testing.T) {

	t.Parallel()

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
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[1])

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

	t.Parallel()

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
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.MissingResourceAnnotationError{}, errs[0])
				assert.IsType(t, &sema.InvalidFailableResourceDowncastOutsideOptionalBindingError{}, errs[1])

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

func TestCheckUnaryMove(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          destroy create X()
      }
    `)

	require.NoError(t, err)
}

func TestCheckIndirectDestroy(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- X()
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingCreateError{}, errs[0])

}

func TestCheckInvalidDestroy(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

	checker, err := ParseAndCheck(t, `
      pub resource R {}
	`)

	require.NoError(t, err)

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

	assert.IsType(t, &sema.InvalidResourceCreationError{}, errs[0])
}

func TestCheckResourceCreationInContracts(t *testing.T) {

	t.Parallel()

	t.Run("in sibling contract", func(t *testing.T) {

		_, err := ParseAndCheck(t,
			`
              contract A {
                  resource R {}
              }

              contract B {

                  pub fun test() {
                      destroy create A.R()
                  }
              }
            `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidResourceCreationError{}, errs[0])
	})

	t.Run("in same contract", func(t *testing.T) {

		_, err := ParseAndCheck(t,
			`
              contract A {
                  resource R {}

                  pub fun test() {
                      destroy create R()
                  }
              }
            `,
		)

		require.NoError(t, err)
	})
}

func TestCheckInvalidResourceLoss(t *testing.T) {

	t.Parallel()

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

	t.Run("ImmediateComparisonOptionalNil", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource Foo {}

            pub fun foo(): @Foo? {
                return <- create Foo()
            }

            pub let isNil = foo() == nil
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	})

	t.Run("ImmediateComparisonArray", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource Foo {}

            let empty: @[Foo] <- []
            let isEmpty = [<- create Foo()] == empty
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidBinaryOperandsError{}, errs[0])
		assert.IsType(t, &sema.ResourceLossError{}, errs[1])
	})

	t.Run("Optional chaining", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
            resource R {}

            struct S {
                fun use(_ r: @R) {
                    destroy r
                }
            }

            fun test() {
                let r <- create R()
                let s: S? = S()
                s?.use(<-r)
            }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	})

}

func TestCheckResourceReturn(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test(): @X {
          return <-create X()
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceReturnMissingMove(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let y <- x
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceVariableDeclarationIncorrectTransfer(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let y <- x
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.IncorrectTransferOperationError{}, errs[0])
}

func TestCheckInvalidResourceAssignmentTransfer(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

func TestCheckResourceAssignmentForceTransfer(t *testing.T) {

	t.Parallel()

	t.Run("new to nil", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource X {}

          fun test() {
              var x: @X? <- nil
              x <-! create X()
              destroy x
          }
        `)

		require.NoError(t, err)
	})

	t.Run("new to non-nil", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource X {}

          fun test() {
              var x: @X? <- create X()
              x <-! create X()
              destroy x
          }
        `)

		require.NoError(t, err)
	})

	t.Run("existing to nil", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource X {}

          fun test() {
              let x <- create X()
              var x2: @X? <- nil
              x2 <-! x
              destroy x2
          }
        `)

		require.NoError(t, err)
	})

	t.Run("existing to non-nil", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource X {}

          fun test() {
              let x <- create X()
              var x2: @X? <- create X()
              x2 <-! x
              destroy x2
          }
        `)

		require.NoError(t, err)
	})

	t.Run("to non-optional", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource X {}

          fun test() {
              let x <- create X()
              var x2 <- create X()
              destroy x2
              x2 <-! x
          }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidResourceAssignmentError{}, errs[0])
	})
}

func TestCheckInvalidResourceLossThroughVariableDeclaration(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [<-x]
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceMoveIntoArrayMissingMoveOperation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [x]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
}

func TestCheckInvalidNonResourceMoveIntoArray(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let xs = [<-x]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckInvalidUseAfterResourceMoveIntoArray(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- [<-x, <-x]
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckResourceMoveIntoDictionary(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {"x": <-x}
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceMoveIntoDictionaryMissingMoveOperation(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource X {}

      let x <- create X()
      let xs <- {"x": x}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingMoveOperationError{}, errs[0])
}

func TestCheckInvalidNonResourceMoveIntoDictionary(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      struct X {}

      let x = X()
      let xs = {"x": <-x}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveOperationError{}, errs[0])
}

func TestCheckInvalidUseAfterResourceMoveIntoDictionary(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

		innerTypeAnnotation := "T"
		if innerIsInterface {
			innerTypeAnnotation = AsInterfaceType("T", innerCompositeKind)
		}

		// Prepare the initializer, if needed.
		// `outerCompositeKind` is the container composite kind.
		// If it is concrete, i.e. not an interface, it needs an initializer.

		initializer := ""
		if !outerIsInterface {
			initializer = fmt.Sprintf(
				`
                  init(t: %[1]s%[2]s) {
                      self.t %[3]s t
                  }
                `,
				innerCompositeKind.Annotation(),
				innerTypeAnnotation,
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
                  let t: %[6]s%[7]s
                  %[8]s
                  %[9]s
              }
            `,
			innerCompositeKind.Keyword(),
			innerInterfaceKeyword,
			innerBody,
			outerCompositeKind.Keyword(),
			outerInterfaceKeyword,
			innerCompositeKind.Annotation(),
			innerTypeAnnotation,
			initializer,
			destructor,
		)

		_, err := ParseAndCheck(t, program)

		switch outerCompositeKind {
		case common.CompositeKindStructure:

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

		case common.CompositeKindResource,
			common.CompositeKindContract:

			require.NoError(t, err)

		default:
			panic(errors.NewUnreachableError())
		}
	})
}

func TestCheckContractResourceField(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource R {}

      contract C {
          let r: @R

          init(r: @R) {
              self.r <- r
          }
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidContractResourceFieldMove(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource R {}

      contract C {
          let r: @R

          init(r: @R) {
              self.r <- r
          }
      }

      fun test() {
          let r <- C.r
          destroy r
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidNestedResourceMoveError{}, errs[0])
}

// TestCheckResourceInterfaceConformance tests the check
// of conformance of resources to resource interfaces.
//
func TestCheckResourceInterfaceConformance(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource interface X {
          fun test()
      }

      resource Y: X {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
}

// TestCheckInvalidResourceInterfaceUseAsType tests that a resource interface
// can not be used as a type
//
func TestCheckInvalidResourceInterfaceUseAsType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource interface I {}

      resource R: I {}

      let r: @I <- create R()
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidInterfaceTypeError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

// TestCheckResourceInterfaceUseAsType test if a resource
// is a subtype of a restricted AnyResource type.
//
func TestCheckResourceInterfaceUseAsType(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource interface I {}

      resource R: I {}

      let r: @{I} <- create R()
    `)

	require.NoError(t, err)
}

func TestCheckResourceArrayIndexing(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

// TestCheckAnyResourceDestruction tests if resources
// can be passed to restricted AnyResources parameters,
// and if the argument can be destroyed.
//
func TestCheckAnyResourceDestruction(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource interface I {}

      resource R: I {}

      fun foo(_ i: @{I}) {
          destroy i
      }

      fun bar() {
          foo(<-create R())
      }
    `)

	require.NoError(t, err)
}

// TestCheckInvalidResourceFieldMoveThroughVariableDeclaration tests if resources nested
// as a field in another resource cannot be moved out of the containing resource through
// a variable declaration. This would partially invalidate the containing resource
//
func TestCheckInvalidResourceFieldMoveThroughVariableDeclaration(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

	_, err := ParseAndCheck(t, `
     resource interface RI {}

     resource R {
         var ris: @{String: {RI}}

         init(_ ri: @{RI}) {
             self.ris <- {"first": <-ri}
         }

         fun use() {
            let ri <- self.ris.remove(key: "first")
            absorb(<-ri)
         }

         destroy() {
             destroy self.ris
         }
     }

     fun absorb(_ ri: @{RI}?) {
         destroy ri
     }
   `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceMethodBinding(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @{RI} <- create R()
             if let r <- ri as? @R {
                 destroy r
             } else {
                 destroy ri
             }
         }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceOptionalBindingFailableCastResourceUseAfterInvalidationInThen(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @{RI} <- create R()
             if let r <- ri as? @R {
                 destroy r
                 destroy ri
             } else {
                 destroy ri
             }
         }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceOptionalBindingFailableCastResourceUseAfterInvalidationAfterBranches(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t,
		`
         resource interface RI {}

         resource R: RI {}

         fun test() {
             let ri: @{RI} <- create R()
             if let r <- ri as? @R {
                 destroy r
             }
             destroy ri
         }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceOptionalBindingFailableCastResourceLossMissingElse(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource interface RI {}

      resource R: RI {}

      fun test() {
          let ri: @{RI} <- create R()
          if let r <- ri as? @R {
              destroy r
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckInvalidResourceOptionalBindingFailableCastResourceUseAfterInvalidationAfterThen(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource interface RI {}

      resource R: RI {}

      fun test() {
          let ri: @{RI} <- create R()
          if let r <- ri as? @R {
              destroy r
          }
          destroy ri
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceUseAfterInvalidationError{}, errs[0])
}

func TestCheckInvalidResourceOptionalBindingFailableCastMissingElse(t *testing.T) {

	t.Parallel()

	t.Run("top-level resource interface to resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          resource interface RI {}

          resource R: RI {}

          fun test(ri: @{RI}) {
              if let r <- ri as? @R {
                  destroy r
              }
          }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	})

	t.Run("contract interface resource to contract to resource", func(t *testing.T) {

		_, err := ParseAndCheck(t, `
          contract interface CI {
              resource R {}
          }

          contract C: CI {
              resource R {}
          }

          fun test(r: @CI.R) {
              if let r2 <- r as? @C.R {
                  destroy r2
              }
          }
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.ResourceLossError{}, errs[0])
	})
}

func TestCheckInvalidResourceFailableCastOutsideOptionalBinding(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource interface RI {}

      resource R: RI {}

      fun test() {
          let ri: @{RI} <- create R()
          let r <- ri as? @R
          destroy r
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidFailableResourceDowncastOutsideOptionalBindingError{}, errs[0])
}

func TestCheckInvalidUnaryMoveAndCopyTransfer(t *testing.T) {

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

	t.Parallel()

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

func TestCheckInvalidResourceOwnerField(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      resource Test {
          let owner: PublicAccount

          init(owner: PublicAccount) {
              self.owner = owner
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceInterfaceOwnerField(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     resource interface Test {
         let owner: PublicAccount
     }
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceOwnerFunction(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     resource Test {
         fun owner() {}
     }
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckInvalidResourceInterfaceOwnerFunction(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     resource interface Test {
         fun owner()
     }
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckResourceOwnerFieldUse(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     resource Test {

         fun test(): PublicAccount? {
             return self.owner
         }
     }
   `)

	require.NoError(t, err)
}

func TestCheckResourceInterfaceOwnerFieldUse(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     resource interface Test {

         fun test() {
             pre { self.owner != nil }
         }
     }
   `)

	require.NoError(t, err)
}

func TestCheckInvalidResourceOwnerFieldInitialization(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
     resource Test {

         init(owner: PublicAccount) {
             self.owner = owner
         }
     }
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
}

func TestCheckInvalidResourceInterfaceType(t *testing.T) {

	t.Parallel()

	t.Run("direct", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
          resource interface RI {}

          resource R: RI {}

          let ri: @RI <- create R()
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidInterfaceTypeError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("in array", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
          resource interface RI {}

          resource R: RI {}

          let ri: @[RI] <- [<-create R()]
        `)

		errs := ExpectCheckerErrors(t, err, 2)

		assert.IsType(t, &sema.InvalidInterfaceTypeError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})
}

func TestCheckRestrictedAnyResourceType(t *testing.T) {

	t.Parallel()

	t.Run("direct", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
          resource interface RI {}

          resource R: RI {}

          let ri: @AnyResource{RI} <- create R()
        `)

		require.NoError(t, err)
	})

	t.Run("in array", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
          resource interface RI {}

          resource R: RI {}

          let ri: @[AnyResource{RI}] <- [<-create R()]
        `)

		require.NoError(t, err)
	})
}

func TestCheckInvalidOptionalResourceNilCoalescingResourceLoss(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheckWithPanic(t, `

      resource R {}

      fun returnResourceOpt() {
          let optR: @R? <- create R()
          let r <- optR ?? panic("no R")
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])
}

func TestCheckOptionalResourceCoalescingAndReturn(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheckWithPanic(t, `

      resource R {}

      fun returnResourceOpt(): @R {
          let optR: @R? <- create R()
          return <- (optR ?? panic("no R"))
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidOptionalResourceCoalescingRightSide(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheckWithPanic(t, `

      resource R {}

      fun returnResourceOpt(): @R {
          let r1: @R? <- create R()
          let r2: @R <- create R()
          return <- (r1 ?? r2)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 3)

	assert.IsType(t, &sema.InvalidNilCoalescingRightResourceOperandError{}, errs[0])
	assert.IsType(t, &sema.ResourceLossError{}, errs[1])
	assert.IsType(t, &sema.ResourceLossError{}, errs[2])
}

// https://github.com/dapperlabs/flow-go/issues/3407
//
// Check that an function's return information
// does not influence another function's return information.
//
func TestCheckInvalidResourceLossInNestedContractResource(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheckWithPanic(t, `

      pub contract C {

          resource R {

              fun foo(r: @R) {
                  if let r2 <- r as? @R {
                      destroy r2
                  }
              }
          }

          pub fun bar() {
              return
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ResourceLossError{}, errs[0])

}
