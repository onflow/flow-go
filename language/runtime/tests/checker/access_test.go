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

func expectSuccess(t *testing.T, err error) {
	assert.NoError(t, err)
}

func expectConformanceError(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
}

func expectInvalidAccessError(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
}

func expectInvalidAssignmentAccessError(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[0])
}

func expectAccessErrors(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
	assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])
}

func expectConformanceAndInvalidAccessErrors(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ConformanceError{}, errs[0])
	assert.IsType(t, &sema.InvalidAccessError{}, errs[1])
}

func expectTwoInvalidAssignmentAccessErrors(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[0])
	assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])
}

func expectTwoAccessErrors(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 4)

	assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
	assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])
	assert.IsType(t, &sema.InvalidAccessError{}, errs[2])
	assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[3])
}

func TestCheckAccessModifierCompositeFunctionDeclaration(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		isAuthAllowed := compositeKind == common.CompositeKindResource

		tests := map[ast.Access]bool{
			ast.AccessNotSpecified:   true,
			ast.AccessPrivate:        true,
			ast.AccessAuthorized:     isAuthAllowed,
			ast.AccessPublic:         true,
			ast.AccessPublicSettable: false,
		}

		require.Len(t, tests, len(ast.Accesses))

		for access, expectSuccess := range tests {
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
					access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              %[1]s %[2]s Test {
                                  %[3]s fun test() %[4]s
                              }
	                        `,
							compositeKind.Keyword(),
							interfaceKeyword,
							access.Keyword(),
							body,
						),
					)

					if expectSuccess {
						assert.NoError(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
					}
				})
			}
		}
	}
}

func TestCheckAccessModifierCompositeConstantFieldDeclaration(t *testing.T) {

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        true,
		ast.AccessAuthorized:     true,
		ast.AccessPublic:         true,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.Accesses))

	for access, expectSuccess := range tests {
		for _, compositeKind := range common.CompositeKinds {
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

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              %[1]s %[2]s Test {
                                  %[3]s let test: Int
                                  %[4]s
                              }
	                        `,
							compositeKind.Keyword(),
							interfaceKeyword,
							access.Keyword(),
							initializer,
						),
					)

					if expectSuccess {
						assert.NoError(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
					}
				})
			}
		}
	}
}

func TestCheckAccessModifierCompositeVariableFieldDeclaration(t *testing.T) {

	for _, access := range ast.Accesses {
		for _, compositeKind := range common.CompositeKinds {
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

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              %[1]s %[2]s Test {
                                  %[3]s var test: Int
                                  %[4]s
                              }
	                        `,
							compositeKind.Keyword(),
							interfaceKeyword,
							access.Keyword(),
							initializer,
						),
					)

					assert.NoError(t, err)
				})
			}
		}
	}
}

func TestCheckAccessModifierGlobalFunctionDeclaration(t *testing.T) {

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        true,
		ast.AccessAuthorized:     false,
		ast.AccessPublic:         true,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.Accesses))

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s fun test() {}
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				assert.NoError(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckAccessModifierGlobalVariableDeclaration(t *testing.T) {

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        true,
		ast.AccessAuthorized:     false,
		ast.AccessPublic:         true,
		ast.AccessPublicSettable: true,
	}

	require.Len(t, tests, len(ast.Accesses))

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s var test = 1
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				assert.NoError(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckAccessModifierGlobalConstantDeclaration(t *testing.T) {

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        true,
		ast.AccessAuthorized:     false,
		ast.AccessPublic:         true,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.Accesses))

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s let test = 1
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				assert.NoError(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckAccessModifierLocalVariableDeclaration(t *testing.T) {

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        false,
		ast.AccessAuthorized:     false,
		ast.AccessPublic:         false,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.Accesses))

	for _, variableKind := range ast.VariableKinds {

		for access, expectSuccess := range tests {

			testName := fmt.Sprintf(
				"%s/%s",
				variableKind.Keyword(),
				access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          fun test() {
                              %s %s foo = 1
                          }
	                    `,
						access.Keyword(),
						variableKind.Keyword(),
					),
				)

				if expectSuccess {
					assert.NoError(t, err)
				} else {
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
				}
			})
		}
	}
}

func TestCheckAccessModifierLocalOptionalBinding(t *testing.T) {

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        false,
		ast.AccessAuthorized:     false,
		ast.AccessPublic:         false,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.Accesses))

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      fun test() {
                          let opt: Int? = 1
                          if %s let value = opt { }
                      }
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				assert.NoError(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckAccessModifierLocalFunctionDeclaration(t *testing.T) {

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        false,
		ast.AccessAuthorized:     false,
		ast.AccessPublic:         false,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.Accesses))

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      fun test() {
                          %s fun foo() {}
                      }
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				assert.NoError(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
			}
		})
	}
}

func TestCheckAccessModifierGlobalCompositeDeclaration(t *testing.T) {

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        true,
		ast.AccessAuthorized:     false,
		ast.AccessPublic:         true,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.Accesses))

	for access, expectSuccess := range tests {
		for _, compositeKind := range common.CompositeKinds {
			for _, isInterface := range []bool{true, false} {

				interfaceKeyword := ""
				if isInterface {
					interfaceKeyword = "interface"
				}

				testName := fmt.Sprintf("%s %s/%s",
					compositeKind.Keyword(),
					interfaceKeyword,
					access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheck(t,
						fmt.Sprintf(
							`
                              %[1]s %[2]s %[3]s Test {}
	                        `,
							access.Keyword(),
							compositeKind.Keyword(),
							interfaceKeyword,
						),
					)

					if expectSuccess {
						assert.NoError(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
					}
				})
			}
		}
	}
}

func TestCheckAccessImportGlobalValue(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]func(*testing.T, error){
		sema.AccessCheckModeStrict: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 2)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t,
				"a",
				errs[0].(*sema.InvalidAccessError).Name,
			)

			require.IsType(t, &sema.InvalidAccessError{}, errs[1])
			assert.Equal(t,
				"c",
				errs[1].(*sema.InvalidAccessError).Name,
			)
		},
		sema.AccessCheckModeNotSpecifiedRestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 2)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t,
				"a",
				errs[0].(*sema.InvalidAccessError).Name,
			)

			require.IsType(t, &sema.InvalidAccessError{}, errs[1])
			assert.Equal(t,
				"c",
				errs[1].(*sema.InvalidAccessError).Name,
			)
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 1)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t,
				"a",
				errs[0].(*sema.InvalidAccessError).Name,
			)
		},
		sema.AccessCheckModeNone: expectSuccess,
	}

	for checkMode, check := range checkModeTests {

		t.Run(checkMode.String(), func(t *testing.T) {

			lastAccessModifier := ""
			if checkMode == sema.AccessCheckModeStrict {
				lastAccessModifier = "priv"
			}

			tests := []string{
				fmt.Sprintf(
					`
                      priv fun a() {}
                      pub fun b() {}
                      %s fun c() {}
                    `,
					lastAccessModifier,
				),
			}

			for _, variableKind := range ast.VariableKinds {

				tests = append(tests,
					fmt.Sprintf(
						`
                           priv %[1]s a = 1
                           pub %[1]s b = 2
                           %[2]s %[1]s c = 3
                        `,
						variableKind.Keyword(),
						lastAccessModifier,
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
                       import a, b, c from "imported"
                    `,
					ParseAndCheckOptions{
						ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
							return imported, nil
						},
						Options: []sema.Option{
							sema.WithAccessCheckMode(checkMode),
						},
					},
				)

				check(t, err)
			}
		})
	}
}

func TestCheckAccessImportGlobalType(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]func(*testing.T, error){
		sema.AccessCheckModeStrict: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 2)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t,
				"A",
				errs[0].(*sema.InvalidAccessError).Name,
			)

			require.IsType(t, &sema.InvalidAccessError{}, errs[1])
			assert.Equal(t,
				"C",
				errs[1].(*sema.InvalidAccessError).Name,
			)
		},
		sema.AccessCheckModeNotSpecifiedRestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 2)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t,
				"A",
				errs[0].(*sema.InvalidAccessError).Name,
			)

			require.IsType(t, &sema.InvalidAccessError{}, errs[1])
			assert.Equal(t,
				"C",
				errs[1].(*sema.InvalidAccessError).Name,
			)
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 1)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t,
				"A",
				errs[0].(*sema.InvalidAccessError).Name,
			)
		},
		sema.AccessCheckModeNone: func(t *testing.T, err error) {
			require.Nil(t, err)
		},
	}

	for _, compositeKind := range common.CompositeKinds {
		for checkMode, check := range checkModeTests {

			testName := fmt.Sprintf("%s/%s",
				compositeKind.Keyword(),
				checkMode,
			)

			t.Run(testName, func(t *testing.T) {

				// NOTE: only parse, don't check imported program.
				// will be checked by checker checking importing program

				lastAccessModifier := ""
				if checkMode == sema.AccessCheckModeStrict {
					lastAccessModifier = "priv"
				}

				imported, _, err := parser.ParseProgram(fmt.Sprintf(
					`
                       priv %[1]s A {}
                       pub %[1]s B {}
                       %[2]s %[1]s C {}
                    `,
					compositeKind.Keyword(),
					lastAccessModifier,
				))

				require.Nil(t, err)

				_, err = ParseAndCheckWithOptions(t,
					`
                       import A, B, C from "imported"
                    `,
					ParseAndCheckOptions{
						ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
							return imported, nil
						},
						Options: []sema.Option{
							sema.WithAccessCheckMode(checkMode),
						},
					},
				)

				check(t, err)
			})
		}
	}
}

func TestCheckAccessCompositeFunction(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		isAuthAllowed := compositeKind == common.CompositeKindResource
		authExpectation := expectSuccess
		if !isAuthAllowed {
			authExpectation = nil
		}

		checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
			sema.AccessCheckModeStrict: {
				ast.AccessNotSpecified:   nil,
				ast.AccessPrivate:        expectInvalidAccessError,
				ast.AccessAuthorized:     authExpectation,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNotSpecifiedRestricted: {
				ast.AccessNotSpecified:   expectInvalidAccessError,
				ast.AccessPrivate:        expectInvalidAccessError,
				ast.AccessAuthorized:     authExpectation,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNotSpecifiedUnrestricted: {
				ast.AccessNotSpecified:   expectSuccess,
				ast.AccessPrivate:        expectInvalidAccessError,
				ast.AccessAuthorized:     authExpectation,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNone: {
				ast.AccessNotSpecified:   expectSuccess,
				ast.AccessPrivate:        expectSuccess,
				ast.AccessAuthorized:     authExpectation,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
		}

		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.Accesses))

			for access, check := range checkModeTests {

				if check == nil {
					continue
				}

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				arguments := ""
				if compositeKind != common.CompositeKindContract {
					arguments = "()"
				}

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(
							`
                              pub %[1]s Test {
                                  %[2]s fun test() {}

                                  pub fun test2() {
                                      self.test()
                                  }
                              }

                              pub fun test() {
                                  let test %[3]s %[4]s Test%[5]s
                                  test.test()
                                  %[6]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							arguments,
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					check(t, err)
				})
			}
		}
	}
}

func TestCheckAccessInterfaceFunction(t *testing.T) {

	for _, compositeKind := range common.CompositeKinds {

		isAuthAllowed := compositeKind == common.CompositeKindResource
		authExpectation := expectSuccess
		if !isAuthAllowed {
			authExpectation = nil
		}

		checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
			sema.AccessCheckModeStrict: {
				ast.AccessNotSpecified:   nil,
				ast.AccessPrivate:        expectConformanceAndInvalidAccessErrors,
				ast.AccessAuthorized:     authExpectation,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNotSpecifiedRestricted: {
				ast.AccessNotSpecified:   expectInvalidAccessError,
				ast.AccessPrivate:        expectConformanceAndInvalidAccessErrors,
				ast.AccessAuthorized:     authExpectation,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNotSpecifiedUnrestricted: {
				ast.AccessNotSpecified:   expectSuccess,
				ast.AccessPrivate:        expectConformanceAndInvalidAccessErrors,
				ast.AccessAuthorized:     authExpectation,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNone: {
				ast.AccessNotSpecified:   expectSuccess,
				ast.AccessPrivate:        expectConformanceError,
				ast.AccessAuthorized:     authExpectation,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
		}

		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.Accesses))

			for access, check := range checkModeTests {

				if check == nil {
					continue
				}

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				arguments := ""
				if compositeKind != common.CompositeKindContract {
					arguments = "()"
				}

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(
							`
                              pub %[1]s interface Test {
                                  %[2]s fun test()
                              }

                              pub %[1]s TestImpl: Test {
                                  %[2]s fun test() {}

                                  pub fun test2() {
                                      self.test()
                                  }
                              }

                              pub fun test() {
                                  let test: %[3]sTest %[4]s %[5]s TestImpl%[6]s
                                  test.test()
                                  %[7]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.Annotation(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							arguments,
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					check(t, err)
				})
			}
		}
	}
}

func TestCheckAccessCompositeFieldRead(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectInvalidAccessError,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectInvalidAccessError,
			ast.AccessPrivate:        expectInvalidAccessError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	for _, compositeKind := range common.CompositeKinds {
		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.Accesses))

			for access, check := range checkModeTests {

				if check == nil {
					continue
				}

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				arguments := ""
				if compositeKind != common.CompositeKindContract {
					arguments = "()"
				}

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(
							`
                              pub %[1]s Test {
                                  %[2]s var test: Int

                                  init() {
                                      self.test = 0
                                  }

                                  pub fun test2() {
                                      self.test
                                  }
                              }

                              pub fun test() {
                                  let test %[3]s %[4]s Test%[5]s
                                  test.test
                                  %[6]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							arguments,
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					check(t, err)
				})
			}
		}
	}
}

func TestCheckAccessInterfaceFieldRead(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectConformanceAndInvalidAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectInvalidAccessError,
			ast.AccessPrivate:        expectConformanceAndInvalidAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectConformanceAndInvalidAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectConformanceError,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	for _, compositeKind := range common.CompositeKinds {
		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.Accesses))

			for access, check := range checkModeTests {

				if check == nil {
					continue
				}

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				arguments := ""
				if compositeKind != common.CompositeKindContract {
					arguments = "()"
				}

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(
							`
                              pub %[1]s interface Test {
                                  %[2]s var test: Int
                              }

                              pub %[1]s TestImpl: Test {
                                  %[2]s var test: Int

                                  init() {
                                      self.test = 0
                                  }

                                  pub fun test2() {
                                      self.test
                                  }
                              }

                              pub fun test() {
                                  let test: %[3]sTest %[4]s %[5]s TestImpl%[6]s
                                  test.test
                                  %[7]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.Annotation(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							arguments,
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					check(t, err)
				})
			}
		}
	}
}

func TestCheckAccessCompositeFieldAssignmentAndSwap(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectTwoAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectTwoAccessErrors,
			ast.AccessPrivate:        expectTwoAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectTwoAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectSuccess,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	for _, compositeKind := range common.CompositeKinds {
		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.Accesses))

			for access, check := range checkModeTests {

				if check == nil {
					continue
				}

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				arguments := ""
				if compositeKind != common.CompositeKindContract {
					arguments = "()"
				}

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(
							`
                              pub %[1]s Test {
                                  %[2]s var test: Int

                                  init() {
                                      self.test = 0
                                  }

                                  pub fun test2() {
                                      self.test = 1
                                      var temp = 2
                                      self.test <-> temp
                                  }
                              }

                              pub fun test() {
                                  let test %[3]s %[4]s Test%[5]s
                                  test.test = 3
                                  var temp = 4
                                  test.test <-> temp
                                  %[6]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							arguments,
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					check(t, err)
				})
			}
		}
	}
}

func TestCheckAccessInterfaceFieldWrite(t *testing.T) {

	expectConformanceAndAccessErrors := func(t *testing.T, err error) {
		errs := ExpectCheckerErrors(t, err, 5)

		assert.IsType(t, &sema.ConformanceError{}, errs[0])
		assert.IsType(t, &sema.InvalidAccessError{}, errs[1])
		assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[2])
		assert.IsType(t, &sema.InvalidAccessError{}, errs[3])
		assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[4])
	}

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectConformanceAndAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectTwoAccessErrors,
			ast.AccessPrivate:        expectConformanceAndAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectConformanceAndAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectConformanceError,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	for _, compositeKind := range common.CompositeKinds {
		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.Accesses))

			for access, check := range checkModeTests {

				if check == nil {
					continue
				}

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				arguments := ""
				if compositeKind != common.CompositeKindContract {
					arguments = "()"
				}

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(
							`
                              pub %[1]s interface Test {
                                  %[2]s var test: Int
                              }

                              pub %[1]s TestImpl: Test {
                                  %[2]s var test: Int

                                  init() {
                                      self.test = 0
                                  }

                                  pub fun test2() {
                                       self.test = 1
                                       var temp = 2
                                       self.test <-> temp
                                  }
                              }

                              pub fun test() {
                                  let test: %[3]sTest %[4]s %[5]s TestImpl%[6]s
                                  test.test = 3
                                  var temp = 4
                                  test.test <-> temp
                                  %[7]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.Annotation(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							arguments,
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					check(t, err)
				})
			}
		}
	}
}

func TestCheckAccessCompositeFieldVariableDeclarationWithSecondValue(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectAccessErrors,
			ast.AccessPrivate:        expectAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectSuccess,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	for checkMode, checkModeTests := range checkModeTests {
		require.Len(t, checkModeTests, len(ast.Accesses))

		for access, check := range checkModeTests {

			if check == nil {
				continue
			}

			testName := fmt.Sprintf(
				"%s/%s",
				checkMode,
				access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheckWithOptions(t,
					fmt.Sprintf(
						`
                          pub resource A {}

                          pub resource B {
                              %[1]s var a: <-A

                              init() {
                                  self.a <- create A()
                              }

                              destroy() {
                                  destroy self.a
                              }

                              pub fun test() {
                                  let oldA <- self.a <- create A()
                                  destroy oldA
                              }
                          }

                          pub fun test() {
                              let b <- create B()
                              let oldA <- b.a <- create A()
                              destroy oldA
                              destroy b
                          }
	                    `,
						access.Keyword(),
					),
					ParseAndCheckOptions{
						Options: []sema.Option{
							sema.WithAccessCheckMode(checkMode),
						},
					},
				)

				check(t, err)
			})
		}
	}
}

func TestCheckAccessInterfaceFieldVariableDeclarationWithSecondValue(t *testing.T) {

	expectConformanceAndInvalidAccessAndAssignmentAccessErrors := func(t *testing.T, err error) {
		errs := ExpectCheckerErrors(t, err, 3)

		assert.IsType(t, &sema.ConformanceError{}, errs[0])
		assert.IsType(t, &sema.InvalidAccessError{}, errs[1])
		assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[2])
	}

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectConformanceAndInvalidAccessAndAssignmentAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectAccessErrors,
			ast.AccessPrivate:        expectConformanceAndInvalidAccessAndAssignmentAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectConformanceAndInvalidAccessAndAssignmentAccessErrors,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectConformanceError,
			ast.AccessAuthorized:     expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	for checkMode, checkModeTests := range checkModeTests {
		require.Len(t, checkModeTests, len(ast.Accesses))

		for access, check := range checkModeTests {

			if check == nil {
				continue
			}

			testName := fmt.Sprintf(
				"%s/%s",
				checkMode,
				access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheckWithOptions(t,
					fmt.Sprintf(
						`
                          pub resource A {}

                          pub resource interface B {
                              %[1]s var a: <-A
                          }

                          pub resource BImpl: B {
                              %[1]s var a: <-A

                              init() {
                                  self.a <- create A()
                              }

                              destroy() {
                                  destroy self.a
                              }

                              pub fun test() {
                                  let oldA <- self.a <- create A()
                                  destroy oldA
                              }
                          }

                          pub fun test() {
                              let b: <-B <- create BImpl()
                              let oldA <- b.a <- create A()
                              destroy oldA
                              destroy b
                          }
	                    `,
						access.Keyword(),
					),
					ParseAndCheckOptions{
						Options: []sema.Option{
							sema.WithAccessCheckMode(checkMode),
						},
					},
				)

				check(t, err)
			})
		}
	}
}

func TestCheckAccessImportGlobalValueAssignmentAndSwap(t *testing.T) {

	worstCase := func(t *testing.T, err error) {
		errs := ExpectCheckerErrors(t, err, 8)

		require.IsType(t, &sema.InvalidAccessError{}, errs[0])
		assert.Equal(t,
			"a",
			errs[0].(*sema.InvalidAccessError).Name,
		)

		require.IsType(t, &sema.InvalidAccessError{}, errs[1])
		assert.Equal(t,
			"c",
			errs[1].(*sema.InvalidAccessError).Name,
		)

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
		assert.Equal(t,
			"a",
			errs[2].(*sema.AssignmentToConstantError).Name,
		)

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[3])
		assert.Equal(t,
			"b",
			errs[3].(*sema.AssignmentToConstantError).Name,
		)

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[4])
		assert.Equal(t,
			"c",
			errs[4].(*sema.AssignmentToConstantError).Name,
		)

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[5])
		assert.Equal(t,
			"a",
			errs[5].(*sema.AssignmentToConstantError).Name,
		)

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[6])
		assert.Equal(t,
			"b",
			errs[6].(*sema.AssignmentToConstantError).Name,
		)

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[7])
		assert.Equal(t,
			"c",
			errs[7].(*sema.AssignmentToConstantError).Name,
		)
	}

	checkModeTests := map[sema.AccessCheckMode]func(*testing.T, error){
		sema.AccessCheckModeStrict:                 worstCase,
		sema.AccessCheckModeNotSpecifiedRestricted: worstCase,
		sema.AccessCheckModeNotSpecifiedUnrestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 7)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t,
				"a",
				errs[0].(*sema.InvalidAccessError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[1])
			assert.Equal(t,
				"a",
				errs[1].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
			assert.Equal(t,
				"b",
				errs[2].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[3])
			assert.Equal(t,
				"c",
				errs[3].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[4])
			assert.Equal(t,
				"a",
				errs[4].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[5])
			assert.Equal(t,
				"b",
				errs[5].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[6])
			assert.Equal(t,
				"c",
				errs[6].(*sema.AssignmentToConstantError).Name,
			)
		},
		sema.AccessCheckModeNone: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 6)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
			assert.Equal(t,
				"a",
				errs[0].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[1])
			assert.Equal(t,
				"b",
				errs[1].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
			assert.Equal(t,
				"c",
				errs[2].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[3])
			assert.Equal(t,
				"a",
				errs[3].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[4])
			assert.Equal(t,
				"b",
				errs[4].(*sema.AssignmentToConstantError).Name,
			)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[5])
			assert.Equal(t,
				"c",
				errs[5].(*sema.AssignmentToConstantError).Name,
			)
		},
	}

	for checkMode, check := range checkModeTests {

		t.Run(checkMode.String(), func(t *testing.T) {

			// NOTE: only parse, don't check imported program.
			// will be checked by checker checking importing program

			lastAccessModifier := ""
			if checkMode == sema.AccessCheckModeStrict {
				lastAccessModifier = "priv"
			}

			imported, _, err := parser.ParseProgram(
				fmt.Sprintf(
					`
                       priv var a = 1
                       pub var b = 2
                       %s var c = 3
                    `,
					lastAccessModifier,
				),
			)

			require.Nil(t, err)

			_, err = ParseAndCheckWithOptions(t,
				`
                  import a, b, c from "imported"

                  pub fun test() {
                      a = 4
                      b = 5
                      c = 6

                      var tempA = 7
                      a <-> tempA

                      var tempB = 8
                      b <-> tempB

                      var tempC = 9
                      c <-> tempC
                  }
                `,
				ParseAndCheckOptions{
					ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
						return imported, nil
					},
					Options: []sema.Option{
						sema.WithAccessCheckMode(checkMode),
					},
				},
			)

			check(t, err)
		})
	}
}

func TestCheckAccessImportGlobalValueVariableDeclarationWithSecondValue(t *testing.T) {

	// NOTE: only parse, don't check imported program.
	// will be checked by checker checking importing program

	imported, _, err := parser.ParseProgram(`
       pub resource R {}

       pub fun createR(): <-R {
           return <-create R()
       }

       priv var x <- createR()
       pub var y <- createR()
    `)

	require.Nil(t, err)

	_, err = ParseAndCheckWithOptions(t,
		`
           import x, y, createR from "imported"

           pub fun test() {
               let oldX <- x <- createR()
               destroy oldX

               let oldY <- y <- createR()
               destroy oldY
           }
        `,
		ParseAndCheckOptions{
			ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
				return imported, nil
			},
		},
	)

	errs := ExpectCheckerErrors(t, err, 5)

	require.IsType(t, &sema.InvalidAccessError{}, errs[0])
	assert.Equal(t,
		"x",
		errs[0].(*sema.InvalidAccessError).Name,
	)

	require.IsType(t, &sema.ResourceCapturingError{}, errs[1])

	require.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
	assert.Equal(t,
		"x",
		errs[2].(*sema.AssignmentToConstantError).Name,
	)

	require.IsType(t, &sema.ResourceCapturingError{}, errs[3])

	require.IsType(t, &sema.AssignmentToConstantError{}, errs[4])
	assert.Equal(t,
		"y",
		errs[4].(*sema.AssignmentToConstantError).Name,
	)
}
