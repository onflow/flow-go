package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckFailableCastingAnyStruct(t *testing.T) {

	t.Run("struct", func(t *testing.T) {
		checker, err := ParseAndCheck(t, `

          struct S {}

          let a: AnyStruct = S()
          let s = a as? S
        `)

		require.NoError(t, err)

		assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
	})

	t.Run("resource", func(t *testing.T) {
		_, err := ParseAndCheck(t, `

          struct S {}

          resource R {}

          let a: AnyStruct = S()
          let r <- a as? @R
        `)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.AlwaysFailingResourceCastingTypeError{}, errs[0])
	})
}

func TestCheckFailableCastingAnyResource(t *testing.T) {

	t.Run("resource", func(t *testing.T) {
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

	t.Run("struct", func(t *testing.T) {
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
}

func TestCheckFailableCastingNumber(t *testing.T) {

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
                                  let z: %[4]s? = y as? %[4]s
                                `,
								test.ty,
								test.value,
								fromType,
								targetType,
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
                                  let z: %[4]s? = y as? %[4]s
                                `,
								test.ty,
								test.value,
								fromType,
								otherType,
							),
						)

						require.NoError(t, err)
					})
				}
			}
		})
	}
}

func TestCheckFailableCastingVoid(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.VoidType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         fun f() {}

                         let x: %[1]s = f()
                         let y: %[2]s? = x as? %[2]s
                       `,
						fromType,
						targetType,
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
                         let y: %[2]s? = x as? %[2]s
                       `,
						fromType,
						otherType,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckFailableCastingString(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.StringType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         let x: %[1]s = "test"
                         let y: %[2]s? = x as? %[2]s
                       `,
						fromType,
						targetType,
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
                         let z: %[2]s? = y as? %[2]s
                       `,
						fromType,
						otherType,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckFailableCastingBool(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.BoolType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         let x: %[1]s = true
                         let y: %[2]s? = x as? %[2]s
                       `,
						fromType,
						targetType,
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
                         let z: %[2]s? = y as? %[2]s
                       `,
						fromType,
						otherType,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckFailableCastingAddress(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.AddressType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         let x: Address = 0x1
                         let y: %[1]s = x
                         let z: %[2]s? = y as? %[2]s
                       `,
						fromType,
						targetType,
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
                         let z: %[2]s? = y as? %[2]s
                       `,
						fromType,
						otherType,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckFailableCastingStruct(t *testing.T) {

	types := []string{
		"AnyStruct",
		"S",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         struct S {}

                         let x: %[1]s = S()
                         let y: %[2]s? = x as? %[2]s
                       `,
						fromType,
						targetType,
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
                     let y: %s = x
                     let z: T? = y as? T
                   `,
					fromType,
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
                         let z: %[2]s? = y as? %[2]s
                       `,
						fromType,
						otherType,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckFailableCastingResource(t *testing.T) {

	types := []string{
		"AnyResource",
		"R",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         resource R {}

                         fun test(): @%[2]s? {
                             let x: @%[1]s <- create R()
                             if let y <- x as? @%[2]s {
                                 return <-y
                             } else {
                                 destroy x
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
		}

		t.Run(fmt.Sprintf("invalid: from %s to T", fromType), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                     resource R {}

                     resource T {}

                     fun test(): @T? {
                         let x: @%s <- create R()
                         if let y <- x as? @T {
                             return <-y
                         } else {
                             destroy x
                             return nil
                         }
                     }
                   `,
					fromType,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckFailableCastingStructInterface(t *testing.T) {

	types := []string{
		"AnyStruct",
		"S",
		"I",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         struct interface I {}

                         struct S: I {}

                         let i: %[1]s = S()
                         let s: %[2]s? = i as? %[2]s
                       `,
						fromType,
						targetType,
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

                     let i: %s = S()
                     let s: T? = i as? T
                   `,
					fromType,
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

                     let i: %s = S()
                     let s: I2? = i as? I2
                   `,
					fromType,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckFailableCastingResourceInterface(t *testing.T) {

	types := []string{
		"AnyResource",
		"R",
		"AnyResource{I}",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

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
		}

		t.Run(fmt.Sprintf("invalid: from %s to other resource", fromType), func(t *testing.T) {

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

		t.Run(fmt.Sprintf("invalid: from %s to other resource interface", fromType), func(t *testing.T) {

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

			require.NoError(t, err)
		})
	}
}

func TestCheckFailableCastingSome(t *testing.T) {

	types := []sema.Type{
		&sema.OptionalType{Type: &sema.IntType{}},
		&sema.OptionalType{Type: &sema.AnyStructType{}},
		&sema.AnyStructType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(`
                         let x: Int? = 42
                         let y: %[1]s = x
                         let z: %[2]s? = y as? %[2]s
                       `,
						fromType,
						targetType,
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
	                      let y: %[2]s? = x as? %[2]s
	                    `,
						fromType,
						otherType,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckFailableCastingArray(t *testing.T) {

	types := []sema.Type{
		&sema.VariableSizedType{Type: &sema.IntType{}},
		&sema.VariableSizedType{Type: &sema.AnyStructType{}},
		&sema.AnyStructType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                         let x: %[1]s = [42]
                         let y: %[2]s? = x as? %[2]s
                       `,
						fromType,
						targetType,
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
		                 let y: [%[2]s]? = x as? [%[2]s]
		                `,
						fromType,
						otherType,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}

func TestCheckFailableCastingDictionary(t *testing.T) {

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

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(`
                         let x: {String: Int} = {"test": 42}
                         let y: %[1]s = x
                         let z: %[2]s? = y as? %[2]s
                       `,
						fromType,
						targetType,
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
	                      let y: %s = x
	                      let z: {String: %[1]s}? = y as? {String: %[1]s}
	                    `,
						fromType,
						otherType,
					),
				)

				require.NoError(t, err)
			})
		}
	}
}
