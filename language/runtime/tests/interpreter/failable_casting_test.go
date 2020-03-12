package interpreter_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

func TestInterpretFailableCastingNumber(t *testing.T) {

	type test struct {
		ty       sema.Type
		value    string
		expected interpreter.Value
	}

	tests := []test{
		{&sema.IntType{}, "42", interpreter.NewIntValue(42)},
		{&sema.UIntType{}, "42", interpreter.NewUIntValue(42)},
		{&sema.Int8Type{}, "42", interpreter.Int8Value(42)},
		{&sema.Int16Type{}, "42", interpreter.Int16Value(42)},
		{&sema.Int32Type{}, "42", interpreter.Int32Value(42)},
		{&sema.Int64Type{}, "42", interpreter.Int64Value(42)},
		{&sema.Int128Type{}, "42", interpreter.Int128Value{Int: big.NewInt(42)}},
		{&sema.Int256Type{}, "42", interpreter.Int256Value{Int: big.NewInt(42)}},
		{&sema.UInt8Type{}, "42", interpreter.UInt8Value(42)},
		{&sema.UInt16Type{}, "42", interpreter.UInt16Value(42)},
		{&sema.UInt32Type{}, "42", interpreter.UInt32Value(42)},
		{&sema.UInt64Type{}, "42", interpreter.UInt64Value(42)},
		{&sema.UInt128Type{}, "42", interpreter.UInt128Value{Int: big.NewInt(42)}},
		{&sema.UInt256Type{}, "42", interpreter.UInt256Value{Int: big.NewInt(42)}},
		{&sema.Word8Type{}, "42", interpreter.Word8Value(42)},
		{&sema.Word16Type{}, "42", interpreter.Word16Value(42)},
		{&sema.Word32Type{}, "42", interpreter.Word32Value(42)},
		{&sema.Word64Type{}, "42", interpreter.Word64Value(42)},
		{&sema.Fix64Type{}, "1.23", interpreter.Fix64Value(123000000)},
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

						inter := parseCheckAndInterpret(t,
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

						assert.Equal(t,
							test.expected,
							inter.Globals["x"].Value,
						)

						assert.Equal(t,
							test.expected,
							inter.Globals["y"].Value,
						)

						assert.Equal(t,
							interpreter.NewSomeValueOwningNonCopying(
								test.expected,
							),
							inter.Globals["z"].Value,
						)
					})
				}

				for _, otherType := range []sema.Type{
					&sema.BoolType{},
					&sema.StringType{},
					&sema.VoidType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
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

						assert.Equal(t,
							interpreter.NilValue{},
							inter.Globals["z"].Value,
						)
					})
				}
			}
		})
	}
}

func TestInterpretFailableCastingVoid(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.VoidType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				assert.Equal(t,
					interpreter.VoidValue{},
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					interpreter.NewSomeValueOwningNonCopying(
						interpreter.VoidValue{},
					),
					inter.Globals["y"].Value,
				)
			})
		}

		for _, otherType := range []sema.Type{
			&sema.BoolType{},
			&sema.StringType{},
			&sema.IntType{},
		} {

			t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				assert.Equal(t,
					interpreter.NilValue{},
					inter.Globals["y"].Value,
				)
			})
		}
	}
}

func TestInterpretFailableCastingString(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.StringType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
                          let x: %[1]s = "test"
                          let y: %[2]s? = x as? %[2]s
                        `,
						fromType,
						targetType,
					),
				)

				assert.Equal(t,
					interpreter.NewStringValue("test"),
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					interpreter.NewSomeValueOwningNonCopying(
						interpreter.NewStringValue("test"),
					),
					inter.Globals["y"].Value,
				)
			})
		}

		for _, otherType := range []sema.Type{
			&sema.BoolType{},
			&sema.VoidType{},
			&sema.IntType{},
		} {

			t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				assert.Equal(t,
					interpreter.NilValue{},
					inter.Globals["z"].Value,
				)
			})
		}
	}
}

func TestInterpretFailableCastingBool(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.BoolType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
                          let x: %[1]s = true
                          let y: %[2]s? = x as? %[2]s
                        `,
						fromType,
						targetType,
					),
				)

				assert.Equal(t,
					interpreter.BoolValue(true),
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					interpreter.NewSomeValueOwningNonCopying(
						interpreter.BoolValue(true),
					),
					inter.Globals["y"].Value,
				)
			})
		}

		for _, otherType := range []sema.Type{
			&sema.StringType{},
			&sema.VoidType{},
			&sema.IntType{},
		} {

			t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				assert.Equal(t,
					interpreter.NilValue{},
					inter.Globals["z"].Value,
				)
			})
		}
	}
}

func TestInterpretFailableCastingAddress(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.AddressType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				addressValue := interpreter.AddressValue{
					0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1,
				}
				assert.Equal(t,
					addressValue,
					inter.Globals["y"].Value,
				)

				assert.Equal(t,
					interpreter.NewSomeValueOwningNonCopying(
						addressValue,
					),
					inter.Globals["z"].Value,
				)
			})
		}

		for _, otherType := range []sema.Type{
			&sema.StringType{},
			&sema.VoidType{},
			&sema.IntType{},
			&sema.BoolType{},
		} {

			t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(`
                          let x: Address = 0x1
                          let y: %[1]s = x
                          let z: %[2]s? = y as? %[2]s
                        `,
						fromType,
						otherType,
					),
				)

				assert.Equal(t,
					interpreter.NilValue{},
					inter.Globals["z"].Value,
				)
			})
		}
	}
}

func TestInterpretFailableCastingStruct(t *testing.T) {

	types := []string{
		"AnyStruct",
		"S",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				assert.IsType(t,
					&interpreter.CompositeValue{},
					inter.Globals["x"].Value,
				)

				require.IsType(t,
					&interpreter.SomeValue{},
					inter.Globals["y"].Value,
				)

				require.IsType(t,
					&interpreter.CompositeValue{},
					inter.Globals["y"].Value.(*interpreter.SomeValue).Value,
				)
			})
		}

		t.Run(fmt.Sprintf("invalid: from %s to T", fromType), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
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

			assert.Equal(t,
				interpreter.NilValue{},
				inter.Globals["z"].Value,
			)
		})

		for _, otherType := range []sema.Type{
			&sema.StringType{},
			&sema.VoidType{},
			&sema.IntType{},
			&sema.BoolType{},
		} {

			t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				assert.Equal(t,
					interpreter.NilValue{},
					inter.Globals["z"].Value,
				)
			})
		}
	}
}

func TestInterpretFailableCastingResource(t *testing.T) {

	types := []string{
		"AnyResource",
		"R",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				value, err := inter.Invoke("test")

				require.NoError(t, err)

				require.IsType(t,
					&interpreter.SomeValue{},
					value,
				)

				require.IsType(t,
					&interpreter.CompositeValue{},
					value.(*interpreter.SomeValue).Value,
				)
			})
		}

		t.Run(fmt.Sprintf("invalid: from %s to T", fromType), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
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

			value, err := inter.Invoke("test")

			require.NoError(t, err)

			require.IsType(t,
				interpreter.NilValue{},
				value,
			)
		})
	}
}

func TestInterpretFailableCastingStructInterface(t *testing.T) {

	types := []string{
		"AnyStruct",
		"S",
		"I",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				require.IsType(t,
					&interpreter.SomeValue{},
					inter.Globals["s"].Value,
				)

				require.IsType(t,
					&interpreter.CompositeValue{},
					inter.Globals["s"].Value.(*interpreter.SomeValue).Value,
				)
			})
		}

		t.Run(fmt.Sprintf("invalid: from %s to other struct", fromType), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
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

			require.IsType(t,
				interpreter.NilValue{},
				inter.Globals["s"].Value,
			)
		})

		t.Run(fmt.Sprintf("invalid: from %s to other struct interface", fromType), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
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

			require.IsType(t,
				interpreter.NilValue{},
				inter.Globals["s"].Value,
			)
		})
	}
}

func TestInterpretFailableCastingResourceInterface(t *testing.T) {

	types := []string{
		"AnyResource",
		"R",
		"AnyResource{I}",
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				result, err := inter.Invoke("test")
				require.NoError(t, err)

				require.IsType(t,
					&interpreter.SomeValue{},
					result,
				)

				require.IsType(t,
					&interpreter.CompositeValue{},
					result.(*interpreter.SomeValue).Value,
				)
			})
		}

		t.Run(fmt.Sprintf("invalid: from %s to other resource", fromType), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
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

			result, err := inter.Invoke("test")
			require.NoError(t, err)

			require.IsType(t,
				interpreter.NilValue{},
				result,
			)
		})
	}
}

func TestInterpretFailableCastingSome(t *testing.T) {

	types := []sema.Type{
		&sema.OptionalType{Type: &sema.IntType{}},
		&sema.OptionalType{Type: &sema.AnyStructType{}},
		&sema.AnyStructType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(`
                          let x: Int? = 42
                          let y: %[1]s = x
                          let z: %[2]s? = y as? %[2]s
                        `,
						fromType,
						targetType,
					),
				)

				expectedValue := interpreter.NewSomeValueOwningNonCopying(
					interpreter.NewIntValue(42),
				)

				assert.Equal(t,
					expectedValue,
					inter.Globals["y"].Value,
				)

				assert.Equal(t,
					interpreter.NewSomeValueOwningNonCopying(
						expectedValue,
					),
					inter.Globals["z"].Value,
				)
			})
		}

		for _, otherType := range []sema.Type{
			&sema.OptionalType{Type: &sema.StringType{}},
			&sema.OptionalType{Type: &sema.VoidType{}},
			&sema.OptionalType{Type: &sema.BoolType{}},
		} {

			t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(`
	                      let x: %[1]s = 42	
	                      let y: %[2]s? = x as? %[2]s
	                    `,
						fromType,
						otherType,
					),
				)

				assert.Equal(t,
					interpreter.NilValue{},
					inter.Globals["y"].Value,
				)
			})
		}
	}
}

func TestInterpretFailableCastingArray(t *testing.T) {

	types := []sema.Type{
		&sema.VariableSizedType{Type: &sema.IntType{}},
		&sema.VariableSizedType{Type: &sema.AnyStructType{}},
		&sema.AnyStructType{},
	}

	for _, fromType := range types {
		for _, targetType := range types {

			t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(
						`
                          let x: %[1]s = [42]
                          let y: %[2]s? = x as? %[2]s
                        `,
						fromType,
						targetType,
					),
				)

				expectedValue := interpreter.NewArrayValueUnownedNonCopying(
					interpreter.NewIntValue(42),
				)

				assert.Equal(t,
					expectedValue,
					inter.Globals["x"].Value,
				)

				assert.Equal(t,
					interpreter.NewSomeValueOwningNonCopying(
						expectedValue,
					),
					inter.Globals["y"].Value,
				)
			})
		}

		for _, otherType := range []sema.Type{
			&sema.StringType{},
			&sema.VoidType{},
			&sema.BoolType{},
		} {

			t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(`
		                 let x: %[1]s = [42]
		                 let y: [%[2]s]? = x as? [%[2]s]
		                `,
						fromType,
						otherType,
					),
				)

				assert.Equal(t,
					interpreter.NilValue{},
					inter.Globals["y"].Value,
				)
			})
		}
	}
}

func TestInterpretFailableCastingDictionary(t *testing.T) {

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

				inter := parseCheckAndInterpret(t,
					fmt.Sprintf(`
                          let x: {String: Int} = {"test": 42}
                          let y: %[1]s = x
                          let z: %[2]s? = y as? %[2]s
                        `,
						fromType,
						targetType,
					),
				)

				expectedValue := interpreter.NewDictionaryValueUnownedNonCopying(
					interpreter.NewStringValue("test"), interpreter.NewIntValue(42),
				)

				assert.Equal(t,
					expectedValue,
					inter.Globals["y"].Value,
				)

				assert.Equal(t,
					interpreter.NewSomeValueOwningNonCopying(
						expectedValue,
					),
					inter.Globals["z"].Value,
				)
			})
		}

		for _, otherType := range []sema.Type{
			&sema.StringType{},
			&sema.VoidType{},
			&sema.BoolType{},
		} {

			t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

				inter := parseCheckAndInterpret(t,
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

				assert.Equal(t,
					interpreter.NilValue{},
					inter.Globals["z"].Value,
				)
			})
		}
	}
}

func TestInterpretFailableCastingResourceType(t *testing.T) {

	// Supertype: Restricted resource

	t.Run("restricted resource -> restricted resource: fewer restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
            resource interface I1 {}

            resource interface I2 {}

            resource R: I1, I2 {}

            fun test(): @R{I2}? {
                let r: @R{I1, I2} <- create R()
                if let r2 <- r as? @R{I2} {
                    return <-r2
                } else {
                    destroy r
                    return nil
                }
            }
          `,
		)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted resource -> restricted resource: more restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

	      fun test(): @R{I1, I2}? {
	          let r: @R{I1} <- create R()
	          if let r2 <- r as? @R{I1, I2} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("unrestricted resource -> restricted resource: same resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I {}

	      resource R: I {}

	      fun test(): @R{I}? {
	          let r: @R <- create R()
	          if let r2 <- r as? @R{I} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
        `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted AnyResource -> conforming restricted resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      fun test(): @R{RI}? {
	          let r: @AnyResource{RI} <- create R()
	          if let r2 <- r as? @R{RI} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	// Supertype: Resource (unrestricted)

	t.Run("restricted resource -> unrestricted resource: same resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I {}

	      resource R: I {}

	      fun test(): @R? {
	          let r: @R{I} <- create R()
	          if let r2 <- r as? @R {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
        `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted AnyResource -> conforming resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      fun test(): @R? {
	          let r: @AnyResource{RI} <- create R()
	          if let r2 <- r as? @R {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted AnyResource -> non-conforming resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      resource T: RI {}

	      fun test(): @T? {
	          let r: @AnyResource{RI} <- create R()
	          if let r2 <- r as? @T {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			interpreter.NilValue{},
			result,
		)
	})

	t.Run("resource -> restricted AnyResource with conformance restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      fun test(): @AnyResource{RI}? {
	          let r: @R <- create R()
	          if let r2 <- r as? @AnyResource{RI} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted resource -> restricted AnyResource with conformance in restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I {}

	      resource R: I {}

	      fun test(): @AnyResource{I}? {
	          let r: @R{I} <- create R()
	          if let r2 <- r as? @AnyResource{I} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted resource -> restricted AnyResource with conformance not in restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

	      fun test(): @AnyResource{I2}? {
	          let r: @R{I1} <- create R()
	          if let r2 <- r as? @AnyResource{I2} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource: fewer restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

	      fun test(): @AnyResource{I2}? {
	          let r: @AnyResource{I1, I2} <- create R()
	          if let r2 <- r as? @AnyResource{I2} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource: more restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

	      fun test(): @AnyResource{I1, I2}? {
	          let r: @AnyResource{I1} <- create R()
	          if let r2 <- r as? @AnyResource{I1, I2} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource with non-conformance restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1 {}

	      fun test(): @AnyResource{I1, I2}? {
	          let r: @AnyResource{I1} <- create R()
	          if let r2 <- r as? @AnyResource{I1, I2} {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			interpreter.NilValue{},
			result,
		)
	})

	t.Run("restricted resource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

	        fun test(): @AnyResource? {
	            let r: @R{I1} <- create R()
	            if let r2 <- r as? @AnyResource {
	                return <-r2
	            } else {
	                destroy r
	                return nil
	            }
	        }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("restricted AnyResource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

	      fun test(): @AnyResource? {
	          let r: @AnyResource{I1} <- create R()
	          if let r2 <- r as? @AnyResource {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})

	t.Run("unrestricted resource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

	      fun test(): @AnyResource? {
	          let r <- create R()
	          if let r2 <- r as? @AnyResource {
	              return <-r2
	          } else {
	              destroy r
	              return nil
	          }
	      }
	    `)

		result, err := inter.Invoke("test")
		require.NoError(t, err)

		require.IsType(t,
			&interpreter.SomeValue{},
			result,
		)
	})
}
