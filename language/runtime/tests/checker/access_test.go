package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckCompositeFunctionDeclarationAccessModifier(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, true},
		{ast.AccessPublic, true},
		{ast.AccessPublicSettable, false},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			for _, isInterface := range []bool{true, false} {

				interfaceKeyword := ""
				body := ""
				if isInterface {
					interfaceKeyword = "interface"
				} else {
					body = "{}"
				}

				testName := fmt.Sprintf("%s %s/%s",
					compositeKind.Keyword(),
					interfaceKeyword,
					test.access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheck(t, fmt.Sprintf(`
                          %[1]s %[2]s Test {
                              %[3]s fun test() %[4]s
                          }
	                    `,
						compositeKind.Keyword(),
						interfaceKeyword,
						test.access.Keyword(),
						body,
					))

					if test.expectSuccess {
						assert.Nil(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
					}
				})
			}
		}
	}
}

func TestCheckCompositeConstantFieldDeclarationAccessModifier(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, true},
		{ast.AccessPublic, true},
		{ast.AccessPublicSettable, false},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			for _, isInterface := range []bool{true, false} {

				interfaceKeyword := ""
				initializer := ""
				if isInterface {
					interfaceKeyword = "interface"
				} else {
					initializer = "init() { self.test = 0 }"
				}

				testName := fmt.Sprintf("%s %s/%s",
					compositeKind.Keyword(),
					interfaceKeyword,
					test.access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheck(t, fmt.Sprintf(`
                          %[1]s %[2]s Test {
                              %[3]s let test: Int
                              %[4]s
                          }
	                    `,
						compositeKind.Keyword(),
						interfaceKeyword,
						test.access.Keyword(),
						initializer,
					))

					if test.expectSuccess {
						assert.Nil(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
					}
				})
			}
		}
	}
}

func TestCheckCompositeVariableFieldDeclarationAccessModifier(t *testing.T) {

	for _, access := range ast.Accesses {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			for _, isInterface := range []bool{true, false} {

				interfaceKeyword := ""
				initializer := ""
				if isInterface {
					interfaceKeyword = "interface"
				} else {
					initializer = "init() { self.test = 0 }"
				}

				testName := fmt.Sprintf("%s %s/%s",
					compositeKind.Keyword(),
					interfaceKeyword,
					access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheck(t, fmt.Sprintf(`
                          %[1]s %[2]s Test {
                              %[3]s var test: Int
                              %[4]s
                          }
	                    `,
						compositeKind.Keyword(),
						interfaceKeyword,
						access.Keyword(),
						initializer,
					))

					assert.Nil(t, err)
				})
			}
		}
	}
}

func TestCheckGlobalFunctionDeclarationAccessModifier(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, true},
		{ast.AccessPublic, true},
		{ast.AccessPublicSettable, false},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {

		t.Run(test.access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  %s fun test() {}
	            `,
				test.access.Keyword(),
			))

			if test.expectSuccess {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckGlobalVariableDeclarationAccessModifier(t *testing.T) {

	for _, access := range ast.Accesses {

		t.Run(access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  %s var test = 1
	            `,
				access.Keyword(),
			))

			assert.Nil(t, err)
		})
	}
}

func TestCheckGlobalConstantDeclarationAccessModifier(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, true},
		{ast.AccessPublic, true},
		{ast.AccessPublicSettable, false},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {

		t.Run(test.access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  %s let test = 1
	            `,
				test.access.Keyword(),
			))

			if test.expectSuccess {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckLocalVariableDeclarationAccessModifier(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, false},
		{ast.AccessPublicSettable, false},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, variableKind := range ast.VariableKinds {

		for _, test := range tests {

			testName := fmt.Sprintf(
				"%s/%s",
				variableKind.Keyword(),
				test.access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                  fun test() {
                      %s %s foo = 1
                  }
	            `,
					test.access.Keyword(),
					variableKind.Keyword(),
				))

				if test.expectSuccess {
					assert.Nil(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
				}
			})
		}
	}
}

func TestCheckLocalOptionalBindingAccessModifier(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, false},
		{ast.AccessPublicSettable, false},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {

		t.Run(test.access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  fun test() {
                      let opt: Int? = 1
                      if %s let value = opt { } 
                  }
	            `,
				test.access.Keyword(),
			))

			if test.expectSuccess {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckLocalFunctionDeclarationAccessModifier(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, false},
		{ast.AccessPublicSettable, false},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {

		t.Run(test.access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  fun test() {
                      %s fun foo() {}
                  }
	            `,
				test.access.Keyword(),
			))

			if test.expectSuccess {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckGlobalCompositeDeclarationAccessModifier(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, true},
		{ast.AccessPublic, true},
		{ast.AccessPublicSettable, false},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			for _, isInterface := range []bool{true, false} {

				interfaceKeyword := ""
				if isInterface {
					interfaceKeyword = "interface"
				}

				testName := fmt.Sprintf("%s %s/%s",
					compositeKind.Keyword(),
					interfaceKeyword,
					test.access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheck(t, fmt.Sprintf(`
                          %[1]s %[2]s %[3]s Test {}
	                    `,
						test.access.Keyword(),
						compositeKind.Keyword(),
						interfaceKeyword,
					))

					if test.expectSuccess {
						assert.Nil(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
					}
				})
			}
		}
	}
}

func TestCheckImportGlobalValueAccess(t *testing.T) {

	tests := []string{
		`
        priv fun x() {}
        pub fun y() {}
        `,
	}

	for _, variableKind := range ast.VariableKinds {

		tests = append(tests,
			fmt.Sprintf(
				`
                   priv %[1]s x = 1
                   pub %[1]s y = 2
                `,
				variableKind.Keyword(),
			),
		)
	}

	for _, test := range tests {
		// NOTE: only parse, don't check imported program.
		// will be checked by checker checking importing program

		imported, _, err := parser.ParseProgram(test)

		require.Nil(t, err)

		_, err = ParseAndCheckWithOptions(t,
			`
               import x, y from "imported"
            `,
			ParseAndCheckOptions{
				ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
					return imported, nil
				},
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		require.IsType(t, &sema.InvalidAccessError{}, errs[0])
		assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "x")

	}
}

func TestCheckImportGlobalTypeAccess(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		// TODO: add support for contracts
		if compositeKind == common.CompositeKindContract {
			continue
		}

		// NOTE: only parse, don't check imported program.
		// will be checked by checker checking importing program

		imported, _, err := parser.ParseProgram(fmt.Sprintf(
			`
               priv %[1]s A {}
               pub %[1]s B {}
            `,
			compositeKind.Keyword(),
		))

		require.Nil(t, err)

		_, err = ParseAndCheckWithOptions(t,
			`
               import A, B from "imported"
            `,
			ParseAndCheckOptions{
				ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
					return imported, nil
				},
			},
		)

		errs := ExpectCheckerErrors(t, err, 1)

		require.IsType(t, &sema.InvalidAccessError{}, errs[0])
		assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "A")
	}
}

func TestCheckCompositeFunctionAccess(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, true},
	}

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			testName := fmt.Sprintf("%s/%s",
				compositeKind.Keyword(),
				test.access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                      %[1]s Test {
                          %[2]s fun test() {}

                          fun test2() {
                              self.test()
                          }
                      }

                      fun test() {
                          let test %[3]s %[4]s Test()
                          test.test()
                          %[5]s test
                      }
	                `,
					compositeKind.Keyword(),
					test.access.Keyword(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					compositeKind.DestructionKeyword(),
				))

				if test.expectSuccess {
					assert.Nil(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
				}
			})
		}
	}
}

func TestCheckInterfaceFunctionAccess(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, true},
	}

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			testName := fmt.Sprintf("%s/%s",
				compositeKind.Keyword(),
				test.access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                      %[1]s interface Test {
                          %[2]s fun test()
                      }

                      %[1]s TestImpl: Test {
                          %[2]s fun test() {}

                          fun test2() {
                              self.test()
                          }
                      }

                      fun test() {
                          let test: %[3]sTest %[4]s %[5]s TestImpl()
                          test.test()
                          %[6]s test
                      }
	                `,
					compositeKind.Keyword(),
					test.access.Keyword(),
					compositeKind.Annotation(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					compositeKind.DestructionKeyword(),
				))

				if test.expectSuccess {
					assert.Nil(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
				}
			})
		}
	}
}

func TestCheckCompositeFieldReadAccess(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, true},
		{ast.AccessPublicSettable, true},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			testName := fmt.Sprintf("%s/%s",
				compositeKind.Keyword(),
				test.access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                      %[1]s Test {
                          %[2]s var test: Int

                          init() {
                              self.test = 0
                          }

                          fun test2() {
                              self.test
                          }
                      }

                      fun test() {
                          let test %[3]s %[4]s Test()
                          test.test
                          %[5]s test
                      }
	                `,
					compositeKind.Keyword(),
					test.access.Keyword(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					compositeKind.DestructionKeyword(),
				))

				if test.expectSuccess {
					assert.Nil(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
				}
			})
		}
	}
}

func TestCheckInterfaceFieldReadAccess(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, true},
		{ast.AccessPublicSettable, true},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			testName := fmt.Sprintf("%s/%s",
				compositeKind.Keyword(),
				test.access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                     %[1]s interface Test {
                         %[2]s var test: Int
                     }

                     %[1]s TestImpl: Test {
                         %[2]s var test: Int

                         init() {
                             self.test = 0
                         }

                         fun test2() {
                             self.test
                         }
                     }

                     fun test() {
                         let test: %[3]sTest %[4]s %[5]s TestImpl()
                         test.test
                         %[6]s test
                     }
	                `,
					compositeKind.Keyword(),
					test.access.Keyword(),
					compositeKind.Annotation(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					compositeKind.DestructionKeyword(),
				))

				if test.expectSuccess {
					assert.Nil(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
				}
			})
		}
	}
}

func TestCheckCompositeFieldAssignmentAndSwapAccess(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, false},
		{ast.AccessPublicSettable, true},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			testName := fmt.Sprintf("%s/%s",
				compositeKind.Keyword(),
				test.access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                      %[1]s Test {
                          %[2]s var test: Int

                          init() {
                              self.test = 0
                          }

                          fun test2() {
                              self.test = 1
                              var temp = 2
                              self.test <-> temp
                          }
                      }

                      fun test() {
                          let test %[3]s %[4]s Test()
                          test.test = 3
                          var temp = 4
                          test.test <-> temp
                          %[5]s test
                      }
	                `,
					compositeKind.Keyword(),
					test.access.Keyword(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					compositeKind.DestructionKeyword(),
				))

				if test.expectSuccess {
					assert.Nil(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 2)

					assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
					assert.IsType(t, &sema.InvalidAccessError{}, errs[1])

					// TODO: check line numbers to ensure errors where for outside composite
				}
			})
		}
	}
}

func TestCheckInterfaceFieldWriteAccess(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, false},
		{ast.AccessPublicSettable, true},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {
		for _, compositeKind := range common.CompositeKinds {

			// TODO: add support for contracts
			if compositeKind == common.CompositeKindContract {
				continue
			}

			testName := fmt.Sprintf("%s/%s",
				compositeKind.Keyword(),
				test.access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                      %[1]s interface Test {
                          %[2]s var test: Int
                      }

                      %[1]s TestImpl: Test {
                          %[2]s var test: Int

                          init() {
                              self.test = 0
                          }

                          fun test2() {
                               self.test = 1
                               var temp = 2
                               self.test <-> temp
                          }
                      }

                      fun test() {
                          let test: %[3]sTest %[4]s %[5]s TestImpl()
                          test.test = 3
                          var temp = 4
                          test.test <-> temp
                          %[6]s test
                      }
	                `,
					compositeKind.Keyword(),
					test.access.Keyword(),
					compositeKind.Annotation(),
					compositeKind.TransferOperator(),
					compositeKind.ConstructionKeyword(),
					compositeKind.DestructionKeyword(),
				))

				if test.expectSuccess {
					assert.Nil(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 2)

					assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
					assert.IsType(t, &sema.InvalidAccessError{}, errs[1])

					// TODO: check line numbers to ensure errors where for outside composite
				}
			})
		}
	}
}

func TestCheckCompositeFieldVariableDeclarationWithSecondValueAccess(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, false},
		{ast.AccessPublicSettable, true},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {

		t.Run(test.access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  resource A {}

                  resource B {
                      %[1]s var a: <-A

                      init() {
                          self.a <- create A()
                      }

                      destroy() {
                          destroy self.a
                      }

                      fun test() {
                          let oldA <- self.a <- create A()
                          destroy oldA
                      }
                  }

                  fun test() {
                      let b <- create B()
                      let oldA <- b.a <- create A()
                      destroy oldA
                      destroy b
                  }
	            `,
				test.access.Keyword(),
			))

			if test.expectSuccess {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessError{}, errs[0])

				// TODO: check line numbers to ensure errors where for outside composite
			}
		})
	}
}

func TestCheckInterfaceFieldVariableDeclarationWithSecondValueAccess(t *testing.T) {

	type test struct {
		access        ast.Access
		expectSuccess bool
	}

	tests := []test{
		{ast.AccessNotSpecified, true},
		{ast.AccessPrivate, false},
		{ast.AccessPublic, false},
		{ast.AccessPublicSettable, true},
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, test := range tests {

		t.Run(test.access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  resource A {}

                  resource interface B {
                      %[1]s var a: <-A
                  }

                  resource BImpl: B {
                      %[1]s var a: <-A

                      init() {
                          self.a <- create A()
                      }

                      destroy() {
                          destroy self.a
                      }

                      fun test() {
                          let oldA <- self.a <- create A()
                          destroy oldA
                      }
                  }

                  fun test() {
                      let b: <-B <- create BImpl()
                      let oldA <- b.a <- create A()
                      destroy oldA
                      destroy b
                  }
	            `,
				test.access.Keyword(),
			))

			if test.expectSuccess {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessError{}, errs[0])

				// TODO: check line numbers to ensure errors where for outside composite
			}
		})
	}
}
