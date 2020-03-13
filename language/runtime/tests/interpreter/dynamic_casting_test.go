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

var dynamicCastingOperators = map[string]bool{"as?": true, "as!": false}

func TestInterpretDynamicCastingNumber(t *testing.T) {

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

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

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
                                          let z: %[4]s? = y %[5]s %[4]s
                                        `,
										test.ty,
										test.value,
										fromType,
										targetType,
										operator,
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
                                          fun test(): %[4]s? {
                                              let x: %[1]s = %[2]s
                                              let y: %[3]s = x
                                              return y %[5]s %[4]s
                                          }
                                        `,
										test.ty,
										test.value,
										fromType,
										otherType,
										operator,
									),
								)

								result, err := inter.Invoke("test")

								if returnsOptional {
									assert.Equal(t,
										interpreter.NilValue{},
										result,
									)
								} else {
									assert.IsType(t,
										&interpreter.TypeMismatchError{},
										err,
									)
								}
							})
						}
					}
				})
			}
		})
	}
}

func TestInterpretDynamicCastingVoid(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.VoidType{},
	}

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  fun f() {}

                                  let x: %[1]s = f()
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
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

                                  fun test(): %[2]s? {
                                      let x: %[1]s = f()
                                      return x %[3]s %[2]s
                                  }
                                `,
								fromType,
								otherType,
								operator,
							),
						)

						result, err := inter.Invoke("test")

						if returnsOptional {
							assert.Equal(t,
								interpreter.NilValue{},
								result,
							)
						} else {
							assert.IsType(t,
								&interpreter.TypeMismatchError{},
								err,
							)
						}
					})
				}
			}
		})
	}
}

func TestInterpretDynamicCastingString(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.StringType{},
	}

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  let x: %[1]s = "test"
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
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
                                  fun test(): %[2]s? { 
                                      let x: String = "test"
                                      let y: %[1]s = x
                                      return y %[3]s %[2]s
                                  }
                                `,
								fromType,
								otherType,
								operator,
							),
						)

						result, err := inter.Invoke("test")

						if returnsOptional {
							assert.Equal(t,
								interpreter.NilValue{},
								result,
							)
						} else {
							assert.IsType(t,
								&interpreter.TypeMismatchError{},
								err,
							)
						}
					})
				}
			}
		})
	}
}

func TestInterpretDynamicCastingBool(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.BoolType{},
	}

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  let x: %[1]s = true
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
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
                                  fun test(): %[2]s? {
                                      let x: Bool = true
                                      let y: %[1]s = x
                                      return y %[3]s %[2]s
                                  }
                                `,
								fromType,
								otherType,
								operator,
							),
						)

						result, err := inter.Invoke("test")

						if returnsOptional {
							assert.Equal(t,
								interpreter.NilValue{},
								result,
							)
						} else {
							assert.IsType(t,
								&interpreter.TypeMismatchError{},
								err,
							)
						}
					})
				}
			}
		})
	}
}

func TestInterpretDynamicCastingAddress(t *testing.T) {

	types := []sema.Type{
		&sema.AnyStructType{},
		&sema.AddressType{},
	}

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  let x: Address = 0x1
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
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
							fmt.Sprintf(
								`
                                  fun test(): %[2]s? {
                                      let x: Address = 0x1
                                      let y: %[1]s = x
                                      return y %[3]s %[2]s
                                  }
                                `,
								fromType,
								otherType,
								operator,
							),
						)

						result, err := inter.Invoke("test")

						if returnsOptional {
							assert.Equal(t,
								interpreter.NilValue{},
								result,
							)
						} else {
							assert.IsType(t,
								&interpreter.TypeMismatchError{},
								err,
							)
						}
					})
				}
			}
		})
	}
}

func TestInterpretDynamicCastingStruct(t *testing.T) {

	types := []string{
		"AnyStruct",
		"S",
	}

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  struct S {}

                                  let x: %[1]s = S()
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
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

                              fun test(): T? {
                                  let x: S = S()
                                  let y: %[1]s = x
                                  return y %[2]s T
                              }
                            `,
							fromType,
							operator,
						),
					)

					result, err := inter.Invoke("test")

					if returnsOptional {
						assert.Equal(t,
							interpreter.NilValue{},
							result,
						)
					} else {
						assert.IsType(t,
							&interpreter.TypeMismatchError{},
							err,
						)
					}
				})

				for _, otherType := range []sema.Type{
					&sema.StringType{},
					&sema.VoidType{},
					&sema.IntType{},
					&sema.BoolType{},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  struct S {}

                                  fun test(): %[2]s? {
                                      let x: S = S()
                                      let y: %[1]s = x
                                      return y %[3]s %[2]s
                                  }
                                `,
								fromType,
								otherType,
								operator,
							),
						)

						result, err := inter.Invoke("test")

						if returnsOptional {
							assert.Equal(t,
								interpreter.NilValue{},
								result,
							)
						} else {
							assert.IsType(t,
								&interpreter.TypeMismatchError{},
								err,
							)
						}
					})
				}
			}
		})
	}
}

func TestInterpretDynamicCastingResource(t *testing.T) {

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

			value, err := inter.Invoke("test")

			require.NoError(t, err)

			require.IsType(t,
				interpreter.NilValue{},
				value,
			)
		})
	}
}

func TestInterpretDynamicCastingStructInterface(t *testing.T) {

	types := []string{
		"AnyStruct",
		"S",
		"I",
	}

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  struct interface I {}

                                  struct S: I {}

                                  let i: %[1]s = S()
                                  let s: %[2]s? = i %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
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

                              fun test(): T? {
                                  let i: %[1]s = S()
                                  return i %[2]s T
                              }
                            `,
							fromType,
							operator,
						),
					)

					result, err := inter.Invoke("test")

					if returnsOptional {
						assert.Equal(t,
							interpreter.NilValue{},
							result,
						)
					} else {
						assert.IsType(t,
							&interpreter.TypeMismatchError{},
							err,
						)
					}
				})

				t.Run(fmt.Sprintf("invalid: from %s to other struct interface", fromType), func(t *testing.T) {

					inter := parseCheckAndInterpret(t,
						fmt.Sprintf(
							`
                              struct interface I {}

                              struct S: I {}

                              struct interface I2 {}

                              fun test(): I2? {
                                  let i: %[1]s = S()
                                  return i %[2]s I2
                              }
                            `,
							fromType,
							operator,
						),
					)

					result, err := inter.Invoke("test")

					if returnsOptional {
						assert.Equal(t,
							interpreter.NilValue{},
							result,
						)
					} else {
						assert.IsType(t,
							&interpreter.TypeMismatchError{},
							err,
						)
					}
				})
			}
		})
	}
}

func TestInterpretDynamicCastingResourceInterface(t *testing.T) {

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

func TestInterpretDynamicCastingSome(t *testing.T) {

	types := []sema.Type{
		&sema.OptionalType{Type: &sema.IntType{}},
		&sema.OptionalType{Type: &sema.AnyStructType{}},
		&sema.AnyStructType{},
	}

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  let x: Int? = 42
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
							),
						)

						expectedValue := interpreter.NewSomeValueOwningNonCopying(
							interpreter.NewIntValue(42),
						)

						assert.Equal(t,
							expectedValue,
							inter.Globals["y"].Value,
						)

						if _, ok := targetType.(*sema.AnyStructType); ok && !returnsOptional {

							assert.Equal(t,
								expectedValue,
								inter.Globals["z"].Value,
							)

						} else {
							assert.Equal(t,
								interpreter.NewSomeValueOwningNonCopying(
									expectedValue,
								),
								inter.Globals["z"].Value,
							)
						}

					})
				}

				for _, otherType := range []sema.Type{
					&sema.OptionalType{Type: &sema.StringType{}},
					&sema.OptionalType{Type: &sema.VoidType{}},
					&sema.OptionalType{Type: &sema.BoolType{}},
				} {

					t.Run(fmt.Sprintf("invalid: from %s to %s", fromType, otherType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  fun test(): %[2]s? {
	                                  let x: %[1]s = 42	
	                                  return x %[3]s %[2]s
	                              }
	                            `,
								fromType,
								otherType,
								operator,
							),
						)

						result, err := inter.Invoke("test")

						if returnsOptional {
							assert.Equal(t,
								interpreter.NilValue{},
								result,
							)
						} else {
							assert.IsType(t,
								&interpreter.TypeMismatchError{},
								err,
							)
						}
					})
				}
			}
		})
	}
}

func TestInterpretDynamicCastingArray(t *testing.T) {

	types := []sema.Type{
		&sema.VariableSizedType{Type: &sema.IntType{}},
		&sema.VariableSizedType{Type: &sema.AnyStructType{}},
		&sema.AnyStructType{},
	}

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  let x: %[1]s = [42]
                                  let y: %[2]s? = x %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
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
							fmt.Sprintf(
								`
		                          fun test(): [%[2]s]? {
		                              let x: %[1]s = [42]
		                              return x %[3]s [%[2]s]
		                          }
		                        `,
								fromType,
								otherType,
								operator,
							),
						)

						result, err := inter.Invoke("test")

						if returnsOptional {
							assert.Equal(t,
								interpreter.NilValue{},
								result,
							)
						} else {
							assert.IsType(t,
								&interpreter.TypeMismatchError{},
								err,
							)
						}
					})
				}
			}
		})
	}
}

func TestInterpretDynamicCastingDictionary(t *testing.T) {

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

	for operator, returnsOptional := range dynamicCastingOperators {

		t.Run(operator, func(t *testing.T) {

			for _, fromType := range types {
				for _, targetType := range types {

					t.Run(fmt.Sprintf("valid: from %s to %s", fromType, targetType), func(t *testing.T) {

						inter := parseCheckAndInterpret(t,
							fmt.Sprintf(
								`
                                  let x: {String: Int} = {"test": 42}
                                  let y: %[1]s = x
                                  let z: %[2]s? = y %[3]s %[2]s
                                `,
								fromType,
								targetType,
								operator,
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
	                              fun test(): {String: %[2]s}? { 
	                                  let x: {String: Int} = {"test": 42}
	                                  let y: %[1]s = x
	                                  return y %[3]s {String: %[2]s}
	                              }
	                            `,
								fromType,
								otherType,
								operator,
							),
						)

						result, err := inter.Invoke("test")

						if returnsOptional {
							assert.Equal(t,
								interpreter.NilValue{},
								result,
							)
						} else {
							assert.IsType(t,
								&interpreter.TypeMismatchError{},
								err,
							)
						}
					})
				}
			}
		})
	}
}

func TestInterpretDynamicCastingResourceType(t *testing.T) {

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

	// TODO: should statically fail?
	t.Run("restricted AnyResource -> non-conforming restricted resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      resource T {}

	      fun test(): @T{}? {
	          let r: @AnyResource{RI} <- create R()
	          if let r2 <- r as? @T{} {
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

	t.Run("AnyResource -> conforming restricted resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

          fun test(): @R{RI}? {
	          let r: @AnyResource <- create R()
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

	t.Run("AnyResource -> non-conforming restricted resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

          resource T {}

          fun test(): @R{RI}? {
	          let r: @AnyResource <- create T()
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
			interpreter.NilValue{},
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

	t.Run("AnyResource -> unrestricted resource: same type", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      fun test(): @R? {
	          let r: @AnyResource <- create R()
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

	t.Run("AnyResource -> unrestricted resource: different type", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      resource T: RI {}

	      fun test(): @T? {
	          let r: @AnyResource <- create R()
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

	// Supertype: restricted AnyResource

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

	t.Run("restricted AnyResource -> restricted AnyResource: different restrictions, conforming", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

	      fun test(): @AnyResource{I2}? {
	          let r: @AnyResource{I1} <- create R()
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

	t.Run("restricted AnyResource -> restricted AnyResource: different restrictions, non-conforming", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1 {}

	      fun test(): @AnyResource{I2}? {
	          let r: @AnyResource{I1} <- create R()
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
			interpreter.NilValue{},
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

	t.Run("AnyResource -> restricted AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I {}

	      resource R: I {}

	      fun test(): @AnyResource{I}? {
	          let r: @AnyResource <- create R()
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

	// Supertype: AnyResource

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

func TestInterpretDynamicCastingAuthorizedReferenceType(t *testing.T) {

	// Supertype: Restricted resource

	t.Run("restricted resource -> restricted resource: fewer restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I1 {}

          resource interface I2 {}

          resource R: I1, I2 {}

          let x <- create R()
          let r = &x as auth &R{I1, I2}
          let r2 = r as? &R{I2}
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted resource -> restricted resource: more restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I1 {}

          resource interface I2 {}

          resource R: I1, I2 {}

          let x <- create R()
          let r = &x as auth &R{I1}
          let r2 = r as? &R{I1, I2}
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("unrestricted resource -> restricted resource: same resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I {}

	      resource R: I {}

          let x <- create R()
	      let r = &x as auth &R
	      let r2 = r as? &R{I}
	   `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> conforming restricted resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

          let x <- create R()
	      let r = &x as auth &AnyResource{RI}
	      let r2 = r as? &R{RI}
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	// TODO: should statically fail?
	t.Run("restricted AnyResource -> non-conforming restricted resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      resource T {}

          let x <- create R()
	      let r = &x as auth &AnyResource{RI}
	      let r2 = r as? &T{}
	    `)

		assert.IsType(t,
			interpreter.NilValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("AnyResource -> conforming restricted resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

          let x <- create R()
	      let r = &x as auth &AnyResource
	      let r2 = r as? &R{RI}
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("AnyResource -> non-conforming restricted resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

          resource T {}

          let x <- create T()
	      let r = &x as auth &AnyResource
	      let r2 = r as? &R{RI}
	    `)

		assert.IsType(t,
			interpreter.NilValue{},
			inter.Globals["r2"].Value,
		)
	})

	// Supertype: Resource (unrestricted)

	t.Run("restricted resource -> unrestricted resource: same resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I {}

	      resource R: I {}

          let x <- create R()
	      let r = &x as auth &R{I}
	      let r2 = r as? &R
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> conforming resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

          let x <- create R()
	      let r = &x as auth &AnyResource{RI}
	      let r2 = r as? &R
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> non-conforming resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      resource T: RI {}

          let x <- create R()
	      let r = &x as auth &AnyResource{RI}
	      let r2 = r as? &T
	    `)

		assert.IsType(t,
			interpreter.NilValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("AnyResource -> unrestricted resource: same type", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

          let x <- create R()
	      let r = &x as auth &AnyResource
	      let r2 = r as? &R
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("AnyResource -> unrestricted resource: different type", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

	      resource T: RI {}

          let x <- create R()
	      let r = &x as auth &AnyResource
	      let r2 = r as? &T
	    `)

		assert.IsType(t,
			interpreter.NilValue{},
			inter.Globals["r2"].Value,
		)
	})

	// Supertype: restricted AnyResource

	t.Run("resource -> restricted AnyResource with conformance restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface RI {}

	      resource R: RI {}

          let x <- create R()
	      let r = &x as auth &R
	      let r2 = r as? &AnyResource{RI}
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted resource -> restricted AnyResource with conformance in restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I {}

	      resource R: I {}

          let x <- create R()
	      let r = &x as auth &R{I}
	      let r2 = r as? &AnyResource{I}
	   `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted resource -> restricted AnyResource with conformance not in restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

          let x <- create R()
	      let r = &x as auth &R{I1}
	      let r2 = r as? &AnyResource{I2}
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource: fewer restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

          let x <- create R()
	      let r = &x as auth &AnyResource{I1, I2}
	      let r2 = r as? &AnyResource{I2}
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource: more restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

          let x <- create R()
	      let r = &x as auth &AnyResource{I1}
	      let r2 = r as? &AnyResource{I1, I2}
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource: different restrictions, conforming", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

          let x <- create R()
	      let r = &x as auth &AnyResource{I1}
	      let r2 = r as? &AnyResource{I2}
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource: different restrictions, non-conforming", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1 {}

          let x <- create R()
	      let r = &x as auth &AnyResource{I1}
	      let r2 = r as? &AnyResource{I2}
	    `)

		assert.IsType(t,
			interpreter.NilValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource with non-conformance restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1 {}

          let x <- create R()
	      let r = &x as auth &AnyResource{I1}
	      let r2 = r as? &AnyResource{I1, I2}
	    `)

		assert.IsType(t,
			interpreter.NilValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("AnyResource -> restricted AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I {}

	      resource R: I {}

          let x <- create R()
	      let r = &x as auth &AnyResource
	      let r2 = r as? &AnyResource{I}
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	// Supertype: AnyResource

	t.Run("restricted resource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

          let x <- create R()
	      let r = &x as auth &R{I1}
          let r2 = r as? &AnyResource
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

          let x <- create R()
	      let r = &x as auth &AnyResource{I1}
	      let r2 = r as? &AnyResource
	    `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("unrestricted resource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
	      resource interface I1 {}

	      resource interface I2 {}

	      resource R: I1, I2 {}

          let x <- create R()
	      let r = &x as auth &R
	      let r2 = r as? &AnyResource
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})
}

func TestInterpretDynamicCastingUnauthorizedReferenceType(t *testing.T) {

	// Supertype: Restricted resource

	t.Run("restricted resource -> restricted resource: fewer restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I1 {}

          resource interface I2 {}

          resource R: I1, I2 {}

          let x <- create R()
          let r = &x as &R{I1, I2}
          let r2 = r as? &R{I2}
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("unrestricted resource -> restricted resource: same resource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I {}

          resource R: I {}

          let x <- create R()
          let r = &x as &R
          let r2 = r as? &R{I}
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	// Supertype: restricted AnyResource

	t.Run("resource -> restricted AnyResource with conformance restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface RI {}

          resource R: RI {}

          let x <- create R()
          let r = &x as &R
          let r2 = r as? &AnyResource{RI}
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted resource -> restricted AnyResource with conformance in restriction", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I {}

          resource R: I {}

          let x <- create R()
          let r = &x as &R{I}
          let r2 = r as? &AnyResource{I}
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> restricted AnyResource: fewer restrictions", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I1 {}

          resource interface I2 {}

          resource R: I1, I2 {}

          let x <- create R()
          let r = &x as &AnyResource{I1, I2}
          let r2 = r as? &AnyResource{I2}
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	// Supertype: AnyResource

	t.Run("restricted resource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I1 {}

          resource interface I2 {}

          resource R: I1, I2 {}

          let x <- create R()
          let r = &x as &R{I1}
          let r2 = r as? &AnyResource
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("restricted AnyResource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I1 {}

          resource interface I2 {}

          resource R: I1, I2 {}

          let x <- create R()
          let r = &x as &AnyResource{I1}
          let r2 = r as? &AnyResource
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})

	t.Run("unrestricted resource -> AnyResource", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          resource interface I1 {}

          resource interface I2 {}

          resource R: I1, I2 {}

          let x <- create R()
          let r = &x as &R
          let r2 = r as? &AnyResource
        `)

		assert.IsType(t,
			&interpreter.SomeValue{},
			inter.Globals["r2"].Value,
		)
	})
}
