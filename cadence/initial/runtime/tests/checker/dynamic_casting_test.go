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
	"github.com/onflow/cadence/runtime/sema"
)

var dynamicCastingOperations = []ast.Operation{
	ast.OperationFailableCast,
	ast.OperationForceCast,
}

func TestCheckDynamicCastingAnyStruct(t *testing.T) {

	t.Parallel()

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			t.Run("struct", func(t *testing.T) {
				checker, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          struct S {}

                          let a: AnyStruct = S()
                          let s = a %s S
                        `,
						operation.Symbol(),
					),
				)

				require.NoError(t, err)

				assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
			})

			t.Run("resource", func(t *testing.T) {
				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          struct S {}

                          resource R {}

                          let a: AnyStruct = S()
                          let r <- a %s @R
                        `,
						operation.Symbol(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.AlwaysFailingResourceCastingTypeError{}, errs[0])
			})
		})
	}
}

func TestCheckDynamicCastingAnyResource(t *testing.T) {

	t.Parallel()

	t.Run("resource", func(t *testing.T) {

		t.Run("as?", func(t *testing.T) {

			checker, err := ParseAndCheck(t, `

              resource R {}

              fun test() {
                  let a: @AnyResource <- create R()
                  if let r <- a as? @R {
                      destroy r
                  } else {
                      destroy a
                  }
              }
            `)

			require.NoError(t, err)

			assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
		})

		t.Run("as!", func(t *testing.T) {

			checker, err := ParseAndCheck(t, `

              resource R {}

              fun test() {
                  let a: @AnyResource <- create R()
                  let r <- a as! @R
                  destroy r
              }
            `)

			require.NoError(t, err)

			assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
		})
	})

	t.Run("struct", func(t *testing.T) {

		t.Run("as?", func(t *testing.T) {

			_, err := ParseAndCheck(t, `

              resource R {}

              struct S {}

              fun test() {
                  let a: @AnyResource <- create R()
                  if let s = a as? S {

                  } else {
                      destroy a
                  }
              }
            `)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.AlwaysFailingNonResourceCastingTypeError{}, errs[0])
		})

		t.Run("as!", func(t *testing.T) {

			_, err := ParseAndCheck(t, `

              resource R {}

              struct S {}

              fun test() {
                  let a: @AnyResource <- create R()
                  let s = a as! S
              }
            `)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.AlwaysFailingNonResourceCastingTypeError{}, errs[0])
		})
	})
}

func TestCheckDynamicCastingNumber(t *testing.T) {

	t.Parallel()

	type test struct {
		ty    sema.Type
		value string
	}

	tests := []test{}

	for _, integerType := range sema.AllIntegerTypes {
		tests = append(tests, test{ty: integerType, value: "42"})
	}

	for _, fixedPointType := range sema.AllFixedPointTypes {
		tests = append(tests, test{ty: fixedPointType, value: "1.23"})
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, test := range tests {

				t.Run(test.ty.String(), func(t *testing.T) {

					types := []sema.Type{
						&sema.AnyStructType{},
						test.ty,
					}
					for _, fromType := range types {
						for _, targetType := range types {

							t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

								_, err := ParseAndCheck(t,
									fmt.Sprintf(
										`
                                          let x: %[1]s = %[2]s
                                          let y: %[3]s = x
                                          let z: %[4]s? = y %[5]s %[4]s
                                        `,
										test.ty,
										test.value,
										fromType,
										targetType,
										operation.Symbol(),
									),
								)

								require.NoError(t, err)
							})
						}

						for _, otherType := range []sema.Type{
							&sema.BoolType{},
							&sema.StringType{},
							&sema.VoidType{},
						} {

							t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

								_, err := ParseAndCheck(t,
									fmt.Sprintf(
										`
                                          let x: %[1]s = %[2]s
                                          let y: %[3]s = x
                                          let z: %[4]s? = y %[5]s %[4]s
                                        `,
										test.ty,
										test.value,
										fromType,
										otherType,
										operation.Symbol(),
									),
								)

								require.NoError(t, err)
							})
						}
					}
				})
			}
		})
	}
}

func TestCheckDynamicCastingVoid(t *testing.T) {

	t.Parallel()

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.VoidType{},
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  fun f() {}

                                  let x: %[1]s = f()
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				for _, otherType := range []sema.Type{
					&sema.BoolType{},
					&sema.StringType{},
					&sema.IntType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  fun f() {}

                                  let x: %[1]s = f()
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								otherType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}

func TestCheckDynamicCastingString(t *testing.T) {

	t.Parallel()

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.StringType{},
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  let x: %[1]s = "test"
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				for _, otherType := range []sema.Type{
					&sema.BoolType{},
					&sema.VoidType{},
					&sema.IntType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  let x: String = "test"
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								otherType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}

func TestCheckDynamicCastingBool(t *testing.T) {

	t.Parallel()

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.BoolType{},
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  let x: %[1]s = true
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				for _, otherType := range []sema.Type{
					&sema.StringType{},
					&sema.VoidType{},
					&sema.IntType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  let x: Bool = true
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								otherType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}

func TestCheckDynamicCastingAddress(t *testing.T) {

	t.Parallel()

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.AddressType{},
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  let x: Address = 0x1
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				for _, otherType := range []sema.Type{
					&sema.StringType{},
					&sema.VoidType{},
					&sema.IntType{},
					&sema.BoolType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(`
                                  let x: Address = 0x1
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								otherType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}

func TestCheckDynamicCastingStruct(t *testing.T) {

	t.Parallel()

	types := []string{
		"AnyStruct",
		"S",
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  struct S {}

                                  let x: %[1]s = S()
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				t.Run(fmt.Sprintf("invalid: from %s to T", fromType), func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              struct S {}

                              struct T {}

                              let x: S = S()
                              let y: %[1]s = x
                              let z: T? = y %[2]s T
                            `,
							fromType,
							operation.Symbol(),
						),
					)

					require.NoError(t, err)
				})

				for _, otherType := range []sema.Type{
					&sema.StringType{},
					&sema.VoidType{},
					&sema.IntType{},
					&sema.BoolType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(`
                                  struct S {}

                                  let x: S = S()
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								otherType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}

func TestCheckDynamicCastingResource(t *testing.T) {

	t.Parallel()

	types := []string{
		"AnyResource",
		"R",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				t.Run("as?", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              resource R {}

                              fun test(): @%[2]s? {
                                  let r: @%[1]s <- create R()
                                  if let r2 <- r as? @%[2]s {
                                      return <-r2
                                  } else {
                                      destroy r
                                      return nil
                                  }
                              }
                            `,
							fromType,
							targetType,
						),
					)

					require.NoError(t, err)
				})

				t.Run("as!", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              resource R {}

                              fun test(): @%[2]s? {
                                  let r: @%[1]s <- create R()
                                  let r2 <- r as! @%[2]s
                                  return <-r2
                              }
                            `,
							fromType,
							targetType,
						),
					)

					require.NoError(t, err)
				})
			})
		}

		t.Run(fmt.Sprintf("invalid: from %s to T", fromType), func(t *testing.T) {

			t.Run("as?", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          resource R {}

                          resource T {}

                          fun test(): @T? {
                              let r: @%s <- create R()
                              if let r2 <- r as? @T {
                                  return <-r2
                              } else {
                                  destroy r
                                  return nil
                              }
                          }
                        `,
						fromType,
					),
				)

				require.NoError(t, err)
			})

			t.Run("as!", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          resource R {}

                          resource T {}

                          fun test(): @T? {
                              let r: @%s <- create R()
                              let r2 <- r as! @T
                              return <-r2
                          }
                        `,
						fromType,
					),
				)

				require.NoError(t, err)
			})

		})
	}
}

func TestCheckDynamicCastingStructInterface(t *testing.T) {

	t.Parallel()

	types := []string{
		"AnyStruct",
		"S",
		"AnyStruct{I}",
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  struct interface I {}

                                  struct S: I {}

                                  let i: %[1]s = S()
                                  let s: %[2]s? = i %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				t.Run(fmt.Sprintf("invalid: from %s to other struct", fromType), func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              struct interface I {}

                              struct S: I {}

                              struct T: I {}

                              let i: %[1]s = S()
                              let s: T? = i %[2]s T
                            `,
							fromType,
							operation.Symbol(),
						),
					)

					require.NoError(t, err)
				})

				t.Run(fmt.Sprintf("invalid: from %s to other struct interface", fromType), func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              struct interface I {}

                              struct S: I {}

                              struct interface I2 {}

                              let i: %[1]s = S()
                              let s: AnyStruct{I2}? = i %[2]s AnyStruct{I2}
                            `,
							fromType,
							operation.Symbol(),
						),
					)

					if fromType == "S" {
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
}

func TestCheckDynamicCastingResourceInterface(t *testing.T) {

	t.Parallel()

	types := []string{
		"AnyResource",
		"R",
		"AnyResource{I}",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				t.Run("as?", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              resource interface I {}

                              resource R: I {}

                              fun test(): @%[2]s? {
                                  let i: @%[1]s <- create R()
                                  if let r <- i as? @%[2]s {
                                      return <-r
                                  } else {
                                      destroy i
                                      return nil
                                  }
                              }
                            `,
							fromType,
							targetType,
						),
					)

					require.NoError(t, err)
				})

				t.Run("as!", func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              resource interface I {}

                              resource R: I {}

                              fun test(): @%[2]s? {
                                  let i: @%[1]s <- create R()
                                  let r <- i as! @%[2]s
                                  return <-r
                              }
                            `,
							fromType,
							targetType,
						),
					)

					require.NoError(t, err)
				})
			})
		}

		t.Run(fmt.Sprintf("invalid: from %s to other resource", fromType), func(t *testing.T) {

			t.Run("as?", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          resource interface I {}

                          resource R: I {}

                          resource T: I {}

                          fun test(): @T? {
                              let i: @%s <- create R()
                              if let r <- i as? @T {
                                  return <-r
                              } else {
                                  destroy i
                                  return nil
                              }
                          }
                        `,
						fromType,
					),
				)

				require.NoError(t, err)
			})

			t.Run("as!", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          resource interface I {}

                          resource R: I {}

                          resource T: I {}

                          fun test(): @T? {
                              let i: @%s <- create R()
                              let r <- i as! @T
                              return <-r
                          }
                        `,
						fromType,
					),
				)

				require.NoError(t, err)
			})
		})

		t.Run(fmt.Sprintf("invalid: from %s to other resource interface", fromType), func(t *testing.T) {

			t.Run("as?", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          resource interface I {}

                          resource R: I {}

                          resource interface I2 {}

                          fun test(): @AnyResource{I2}? {
                              let i: @%s <- create R()
                              if let r <- i as? @AnyResource{I2} {
                                  return <-r
                              } else {
                                  destroy i
                                  return nil
                              }
                          }
                        `,
						fromType,
					),
				)

				if fromType == "R" {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
				} else {
					require.NoError(t, err)
				}
			})

			t.Run("as!", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          resource interface I {}

                          resource R: I {}

                          resource interface I2 {}

                          fun test(): @AnyResource{I2}? {
                              let i: @%s <- create R()
                              let r <- i as! @AnyResource{I2}
                              return <-r
                          }
                        `,
						fromType,
					),
				)

				if fromType == "R" {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
				} else {
					require.NoError(t, err)
				}
			})
		})
	}
}

func TestCheckDynamicCastingSome(t *testing.T) {

	t.Parallel()

	types := []sema.Type{
		&sema.OptionalType{Type: &sema.IntType{}},
		&sema.OptionalType{Type: &sema.AnyStructType{}},
		&sema.AnyStructType{},
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(`
                                 let x: Int? = 42
                                 let y: %[1]s = x
                                 let z: %[2]s? = y %[3]s %[2]s
                               `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				for _, otherType := range []sema.Type{
					&sema.OptionalType{Type: &sema.StringType{}},
					&sema.OptionalType{Type: &sema.VoidType{}},
					&sema.OptionalType{Type: &sema.BoolType{}},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(`
	                              let x: %[1]s = 42
	                              let y: %[2]s? = x %[3]s %[2]s
	                            `,
								fromType,
								otherType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}

func TestCheckDynamicCastingArray(t *testing.T) {

	t.Parallel()

	types := []sema.Type{
		&sema.VariableSizedType{Type: &sema.IntType{}},
		&sema.VariableSizedType{Type: &sema.AnyStructType{}},
		&sema.AnyStructType{},
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  let x: %[1]s = [42]
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				for _, otherType := range []sema.Type{
					&sema.StringType{},
					&sema.VoidType{},
					&sema.BoolType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(`
		                         let x: %[1]s = [42]
		                         let y: [%[2]s]? = x %[3]s [%[2]s]
		                        `,
								fromType,
								otherType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}

func TestCheckDynamicCastingDictionary(t *testing.T) {

	t.Parallel()

	types := []sema.Type{
		&sema.DictionaryType{
			KeyType:   &sema.StringType{},
			ValueType: &sema.IntType{},
		},
		&sema.DictionaryType{
			KeyType:   &sema.StringType{},
			ValueType: &sema.AnyStructType{},
		},
	}

	for _, operation := range dynamicCastingOperations {

		t.Run(operation.Symbol(), func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(`
                                  let x: {String: Int} = {"test": 42}
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}

				for _, otherType := range []sema.Type{
					&sema.StringType{},
					&sema.VoidType{},
					&sema.BoolType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
	                              let x: {String: Int} = {"test": 42}
	                              let y: %[1]s = x
	                              let z: {String: %[2]s}? = y %[3]s {String: %[2]s}
	                            `,
								fromType,
								otherType,
								operation.Symbol(),
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}
