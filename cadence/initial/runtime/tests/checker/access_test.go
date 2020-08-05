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
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/sema"
	. "github.com/onflow/cadence/runtime/tests/utils"
)

func expectSuccess(t *testing.T, err error) {
	assert.NoError(t, err)
}

func expectInvalidAccessModifierError(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
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

func expectInvalidAccessModifierAndInvalidAccessErrors(t *testing.T, err error) {
	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
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

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		tests := map[ast.Access]bool{
			ast.AccessNotSpecified:   true,
			ast.AccessPrivate:        true,
			ast.AccessPublic:         true,
			ast.AccessPublicSettable: false,
		}

		require.Len(t, tests, len(ast.BasicAccesses))

		for access, expectSuccess := range tests {

			testName := fmt.Sprintf(
				"%s/%s",
				compositeKind.Keyword(),
				access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          %[1]s Test {
                              %[2]s fun test() {}
                          }
	                    `,
						compositeKind.Keyword(),
						access.Keyword(),
					),
				)

				if expectSuccess {
					assert.NoError(t, err)
				} else {
					expectInvalidAccessModifierError(t, err)
				}
			})
		}
	}
}

func TestCheckAccessModifierInterfaceFunctionDeclaration(t *testing.T) {

	t.Parallel()

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]error{
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   &sema.MissingAccessModifierError{},
			ast.AccessPrivate:        &sema.InvalidAccessModifierError{},
			ast.AccessPublic:         nil,
			ast.AccessPublicSettable: &sema.InvalidAccessModifierError{},
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        &sema.InvalidAccessModifierError{},
			ast.AccessPublic:         nil,
			ast.AccessPublicSettable: &sema.InvalidAccessModifierError{},
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        &sema.InvalidAccessModifierError{},
			ast.AccessPublic:         nil,
			ast.AccessPublicSettable: &sema.InvalidAccessModifierError{},
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        &sema.InvalidAccessModifierError{},
			ast.AccessPublic:         nil,
			ast.AccessPublicSettable: &sema.InvalidAccessModifierError{},
		},
	}

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

	for checkMode, tests := range checkModeTests {
		require.Len(t, tests, len(ast.BasicAccesses))

		for access, expectedErr := range tests {

			for _, compositeKind := range common.CompositeKindsWithBody {

				testName := fmt.Sprintf(
					"%s/%s interface/%s",
					checkMode,
					compositeKind.Keyword(),
					access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(
							`
                              pub %[1]s interface Test {
                                  %[2]s fun test()
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					if expectedErr == nil {
						assert.NoError(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, 1)

						assert.IsType(t, expectedErr, errs[0])
					}
				})
			}
		}
	}
}

func TestCheckAccessModifierCompositeConstantFieldDeclaration(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]func(isInterface bool) bool{
		ast.AccessNotSpecified: func(_ bool) bool {
			return true
		},
		ast.AccessPrivate: func(isInterface bool) bool {
			return !isInterface
		},
		ast.AccessPublic: func(_ bool) bool {
			return true
		},
		ast.AccessPublicSettable: func(_ bool) bool {
			return false
		},
	}

	require.Len(t, tests, len(ast.BasicAccesses))

	for access, expectSuccess := range tests {
		for _, compositeKind := range common.CompositeKindsWithBody {
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

					if expectSuccess(isInterface) {
						assert.NoError(t, err)
					} else {
						expectInvalidAccessModifierError(t, err)
					}
				})
			}
		}
	}
}

func TestCheckAccessModifierCompositeVariableFieldDeclaration(t *testing.T) {

	t.Parallel()

	for _, access := range ast.BasicAccesses {
		for _, compositeKind := range common.CompositeKindsWithBody {
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

					// private fields in interfaces are invalid

					if isInterface && access == ast.AccessPrivate {
						expectInvalidAccessModifierError(t, err)
					} else {
						assert.NoError(t, err)
					}
				})
			}
		}
	}
}

func TestCheckAccessModifierGlobalFunctionDeclaration(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        true,
		ast.AccessPublic:         true,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.BasicAccesses))

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
				expectInvalidAccessModifierError(t, err)
			}
		})
	}
}

func TestCheckAccessModifierGlobalVariableDeclaration(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        true,
		ast.AccessPublic:         true,
		ast.AccessPublicSettable: true,
	}

	require.Len(t, tests, len(ast.BasicAccesses))

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
				expectInvalidAccessModifierError(t, err)
			}
		})
	}
}

func TestCheckAccessModifierGlobalConstantDeclaration(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        true,
		ast.AccessPublic:         true,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.BasicAccesses))

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
				expectInvalidAccessModifierError(t, err)
			}
		})
	}
}

func TestCheckAccessModifierLocalVariableDeclaration(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        false,
		ast.AccessPublic:         false,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.BasicAccesses))

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
					expectInvalidAccessModifierError(t, err)
				}
			})
		}
	}
}

// TODO: remove
func TestCheckAccessModifierLocalOptionalBinding(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        false,
		ast.AccessPublic:         false,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.BasicAccesses))

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheckWithOptions(t,
				fmt.Sprintf(
					`
                      fun test() {
                          let opt: Int? = 1
                          if %s let value = opt { }
                      }
	                `,
					access.Keyword(),
				),
				ParseAndCheckOptions{
					SkipNewParser: true,
				},
			)

			if expectSuccess {
				assert.NoError(t, err)
			} else {
				expectInvalidAccessModifierError(t, err)
			}
		})
	}
}

func TestCheckAccessModifierLocalFunctionDeclaration(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessNotSpecified:   true,
		ast.AccessPrivate:        false,
		ast.AccessPublic:         false,
		ast.AccessPublicSettable: false,
	}

	require.Len(t, tests, len(ast.BasicAccesses))

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
				expectInvalidAccessModifierError(t, err)
			}
		})
	}
}

func TestCheckAccessModifierGlobalCompositeDeclaration(t *testing.T) {

	t.Parallel()

	expectMissingAccessModifierError := func(t *testing.T, err error) {
		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.MissingAccessModifierError{}, errs[0])
	}

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   expectMissingAccessModifierError,
			ast.AccessPrivate:        expectInvalidAccessModifierError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectInvalidAccessModifierError,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectMissingAccessModifierError,
			ast.AccessPrivate:        expectInvalidAccessModifierError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectInvalidAccessModifierError,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessModifierError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectInvalidAccessModifierError,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessModifierError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectInvalidAccessModifierError,
		},
	}

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

	for checkMode, tests := range checkModeTests {
		require.Len(t, tests, len(ast.BasicAccesses))

		for access, check := range tests {
			for _, compositeKind := range common.AllCompositeKinds {
				for _, isInterface := range []bool{true, false} {

					if !compositeKind.SupportsInterfaces() && isInterface {
						continue
					}

					interfaceKeyword := ""
					if isInterface {
						interfaceKeyword = "interface"
					}

					body := "{}"
					if compositeKind == common.CompositeKindEvent {
						body = "()"
					}

					testName := fmt.Sprintf("%s %s/%s/%s",
						compositeKind.Keyword(),
						interfaceKeyword,
						checkMode,
						access.Keyword(),
					)

					t.Run(testName, func(t *testing.T) {

						_, err := ParseAndCheckWithOptions(t,
							fmt.Sprintf(
								`
                                  %[1]s %[2]s %[3]s Test %[4]s
	                            `,
								access.Keyword(),
								compositeKind.Keyword(),
								interfaceKeyword,
								body,
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
}

func TestCheckAccessImportGlobalValue(t *testing.T) {

	t.Parallel()

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

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

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

				require.NoError(t, err)

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

func TestCheckAccessCompositeFunction(t *testing.T) {

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
			sema.AccessCheckModeStrict: {
				ast.AccessNotSpecified:   nil,
				ast.AccessPrivate:        expectInvalidAccessError,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNotSpecifiedRestricted: {
				ast.AccessNotSpecified:   expectInvalidAccessError,
				ast.AccessPrivate:        expectInvalidAccessError,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNotSpecifiedUnrestricted: {
				ast.AccessNotSpecified:   expectSuccess,
				ast.AccessPrivate:        expectInvalidAccessError,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNone: {
				ast.AccessNotSpecified:   expectSuccess,
				ast.AccessPrivate:        expectSuccess,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
		}

		require.Len(t, checkModeTests, len(sema.AccessCheckModes))

		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.BasicAccesses))

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

				var setupCode, tearDownCode, identifier string
				if compositeKind == common.CompositeKindContract {
					identifier = "Test"
				} else {
					setupCode = fmt.Sprintf(
						`let test %[1]s %[2]s Test%[3]s`,
						compositeKind.TransferOperator(),
						compositeKind.ConstructionKeyword(),
						constructorArguments(compositeKind),
					)
					identifier = "test"
				}

				if compositeKind == common.CompositeKindResource {
					tearDownCode = "destroy test"
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
                                  %[3]s
                                  %[4]s.test()
                                  %[5]s
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							setupCode,
							identifier,
							tearDownCode,
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

	t.Parallel()

	for _, compositeKind := range common.CompositeKindsWithBody {

		checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
			sema.AccessCheckModeStrict: {
				ast.AccessNotSpecified:   nil,
				ast.AccessPrivate:        expectInvalidAccessModifierAndInvalidAccessErrors,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNotSpecifiedRestricted: {
				ast.AccessNotSpecified:   expectConformanceAndInvalidAccessErrors,
				ast.AccessPrivate:        expectInvalidAccessModifierAndInvalidAccessErrors,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNotSpecifiedUnrestricted: {
				ast.AccessNotSpecified:   expectSuccess,
				ast.AccessPrivate:        expectInvalidAccessModifierAndInvalidAccessErrors,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
			sema.AccessCheckModeNone: {
				ast.AccessNotSpecified:   expectSuccess,
				ast.AccessPrivate:        expectInvalidAccessModifierError,
				ast.AccessPublic:         expectSuccess,
				ast.AccessPublicSettable: nil,
			},
		}

		require.Len(t, checkModeTests, len(sema.AccessCheckModes))

		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.BasicAccesses))

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

				var setupCode, tearDownCode, identifier string
				if compositeKind == common.CompositeKindContract {
					identifier = "TestImpl"
				} else {
					interfaceType := AsInterfaceType("Test", compositeKind)

					setupCode = fmt.Sprintf(
						`let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
						compositeKind.Annotation(),
						interfaceType,
						compositeKind.TransferOperator(),
						compositeKind.ConstructionKeyword(),
						constructorArguments(compositeKind),
					)
					identifier = "test"
				}

				if compositeKind == common.CompositeKindResource {
					tearDownCode = "destroy test"
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
                                  %[3]s
                                  %[4]s.test()
                                  %[5]s
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							setupCode,
							identifier,
							tearDownCode,
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

	t.Parallel()

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectInvalidAccessError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectInvalidAccessError,
			ast.AccessPrivate:        expectInvalidAccessError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

	for _, compositeKind := range common.CompositeKindsWithBody {
		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.BasicAccesses))

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

				var setupCode, tearDownCode, identifier string

				if compositeKind == common.CompositeKindContract {
					identifier = "Test"
				} else {
					setupCode = fmt.Sprintf(
						`let test %[1]s %[2]s Test%[3]s`,
						compositeKind.TransferOperator(),
						compositeKind.ConstructionKeyword(),
						constructorArguments(compositeKind),
					)
					identifier = "test"
				}

				if compositeKind == common.CompositeKindResource {
					tearDownCode = `destroy test`
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
                                  %[3]s
                                  %[4]s.test
                                  %[5]s
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							setupCode,
							identifier,
							tearDownCode,
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

	t.Parallel()

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectInvalidAccessModifierAndInvalidAccessErrors,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectConformanceAndInvalidAccessErrors,
			ast.AccessPrivate:        expectInvalidAccessModifierAndInvalidAccessErrors,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessModifierAndInvalidAccessErrors,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessModifierError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

	for _, compositeKind := range common.CompositeKindsWithBody {
		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.BasicAccesses))

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

				var setupCode, tearDownCode, identifier string

				if compositeKind == common.CompositeKindContract {
					identifier = "TestImpl"
				} else {
					interfaceType := AsInterfaceType("Test", compositeKind)

					setupCode = fmt.Sprintf(
						`let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
						compositeKind.Annotation(),
						interfaceType,
						compositeKind.TransferOperator(),
						compositeKind.ConstructionKeyword(),
						constructorArguments(compositeKind),
					)
					identifier = "test"
				}

				if compositeKind == common.CompositeKindResource {
					tearDownCode = `destroy test`
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
                                  %[3]s
                                  %[4]s.test
                                  %[5]s
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							setupCode,
							identifier,
							tearDownCode,
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

	t.Parallel()

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectTwoAccessErrors,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectTwoAccessErrors,
			ast.AccessPrivate:        expectTwoAccessErrors,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectTwoAccessErrors,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

	for _, compositeKind := range common.CompositeKindsWithBody {
		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.BasicAccesses))

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

				var setupCode, tearDownCode, identifier string
				if compositeKind == common.CompositeKindContract {
					identifier = "Test"
				} else {
					setupCode = fmt.Sprintf(
						`let test %[1]s %[2]s Test%[3]s`,
						compositeKind.TransferOperator(),
						compositeKind.ConstructionKeyword(),
						constructorArguments(compositeKind),
					)
					identifier = "test"
				}

				if compositeKind == common.CompositeKindResource {
					tearDownCode = `destroy test`
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
                                  %[3]s

                                  %[4]s.test = 3
                                  var temp = 4
                                  %[4]s.test <-> temp

                                  %[5]s
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							setupCode,
							identifier,
							tearDownCode,
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

	t.Parallel()

	expectConformanceAndAccessErrors := func(t *testing.T, err error) {
		errs := ExpectCheckerErrors(t, err, 5)

		assert.IsType(t, &sema.ConformanceError{}, errs[0])
		assert.IsType(t, &sema.InvalidAccessError{}, errs[1])
		assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[2])
		assert.IsType(t, &sema.InvalidAccessError{}, errs[3])
		assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[4])
	}

	expectInvalidAccessModifierAndAccessErrors := func(t *testing.T, err error) {
		errs := ExpectCheckerErrors(t, err, 5)

		assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
		assert.IsType(t, &sema.InvalidAccessError{}, errs[1])
		assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[2])
		assert.IsType(t, &sema.InvalidAccessError{}, errs[3])
		assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[4])
	}

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectInvalidAccessModifierAndAccessErrors,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectConformanceAndAccessErrors,
			ast.AccessPrivate:        expectInvalidAccessModifierAndAccessErrors,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessModifierAndAccessErrors,
			ast.AccessPublic:         expectTwoInvalidAssignmentAccessErrors,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessModifierError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

	for _, compositeKind := range common.CompositeKindsWithBody {
		for checkMode, checkModeTests := range checkModeTests {
			require.Len(t, checkModeTests, len(ast.BasicAccesses))

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

				var setupCode, tearDownCode, identifier string
				if compositeKind == common.CompositeKindContract {
					identifier = "TestImpl"
				} else {

					interfaceType := AsInterfaceType("Test", compositeKind)

					setupCode = fmt.Sprintf(
						`let test: %[1]s%[2]s %[3]s %[4]s TestImpl%[5]s`,
						compositeKind.Annotation(),
						interfaceType,
						compositeKind.TransferOperator(),
						compositeKind.ConstructionKeyword(),
						constructorArguments(compositeKind),
					)
					identifier = "test"
				}

				if compositeKind == common.CompositeKindResource {
					tearDownCode = "destroy test"
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
                                  %[3]s
                                  %[4]s.test = 3
                                  var temp = 4
                                  %[4]s.test <-> temp
                                  %[5]s
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							setupCode,
							identifier,
							tearDownCode,
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

	t.Parallel()

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectAccessErrors,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   expectAccessErrors,
			ast.AccessPrivate:        expectAccessErrors,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectAccessErrors,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectSuccess,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

	for checkMode, checkModeTests := range checkModeTests {
		require.Len(t, checkModeTests, len(ast.BasicAccesses))

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
                              %[1]s var a: @A

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

	t.Parallel()

	expectPrivateAccessErrors := func(t *testing.T, err error) {
		errs := ExpectCheckerErrors(t, err, 3)

		assert.IsType(t, &sema.InvalidAccessModifierError{}, errs[0])
		assert.IsType(t, &sema.InvalidAccessError{}, errs[1])
		assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[2])
	}

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]func(*testing.T, error){
		sema.AccessCheckModeStrict: {
			ast.AccessNotSpecified:   nil,
			ast.AccessPrivate:        expectPrivateAccessErrors,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified: func(t *testing.T, err error) {
				errs := ExpectCheckerErrors(t, err, 3)

				assert.IsType(t, &sema.ConformanceError{}, errs[0])
				assert.IsType(t, &sema.InvalidAccessError{}, errs[1])
				assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[2])
			},
			ast.AccessPrivate:        expectPrivateAccessErrors,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectPrivateAccessErrors,
			ast.AccessPublic:         expectInvalidAssignmentAccessError,
			ast.AccessPublicSettable: expectSuccess,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   expectSuccess,
			ast.AccessPrivate:        expectInvalidAccessModifierError,
			ast.AccessPublic:         expectSuccess,
			ast.AccessPublicSettable: expectSuccess,
		},
	}

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

	for checkMode, checkModeTests := range checkModeTests {
		require.Len(t, checkModeTests, len(ast.BasicAccesses))

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
                              %[1]s var a: @A
                          }

                          pub resource BImpl: B {
                              %[1]s var a: @A

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
                              let b: @AnyResource{B} <- create BImpl()
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

	t.Parallel()

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

	require.Len(t, checkModeTests, len(sema.AccessCheckModes))

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

			require.NoError(t, err)

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

	t.Parallel()

	// NOTE: only parse, don't check imported program.
	// will be checked by checker checking importing program

	imported, _, err := parser.ParseProgram(`
       pub resource R {}

       pub fun createR(): @R {
           return <-create R()
       }

       priv var x <- createR()
       pub var y <- createR()
    `)

	require.NoError(t, err)

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

func TestCheckContractNestedDeclarationPrivateAccess(t *testing.T) {

	t.Parallel()

	const contract = `
	  contract Outer {
		  priv let num: Int

		  init(num: Int) {
			  self.num = num
		  }

		  resource Inner {
			 fun getNum(): Int {
				return Outer.num
			 }
		  }
	  }
	`

	t.Run("access inside is valid", func(t *testing.T) {
		_, err := ParseAndCheck(t, contract)

		require.NoError(t, err)
	})

	t.Run("access outside is invalid", func(t *testing.T) {
		_, err := ParseAndCheck(t, contract+`
          let num = Outer.num
        `)

		expectInvalidAccessError(t, err)
	})
}

func TestCheckAccessSameContractInnerStructField(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessPrivate:  false,
		ast.AccessContract: true,
		ast.AccessAccount:  true,
		ast.AccessPublic:   true,
	}

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {
			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
	                  contract A {

                          struct B {
                              %s let field: Int

                              init() {
                                  self.field = 42
                              }
                          }

                          fun useB() {
                              let b = A.B()
                              b.field
                          }
                      }
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestCheckAccessSameContractInnerStructInterfaceField(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessPrivate:  false,
		ast.AccessContract: true,
		ast.AccessAccount:  true,
		ast.AccessPublic:   true,
	}

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {
			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
	                  contract A {

                          struct interface B {
                              %[1]s let field: Int
                          }

                          struct BImpl: B {
                              %[1]s let field: Int

                              init() {
                                  self.field = 42
                              }
                          }

                          fun useB() {
                              let b: AnyStruct{B} = A.BImpl()
                              b.field
                          }
                      }
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestCheckAccessOtherContractInnerStructField(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessPrivate:  false,
		ast.AccessContract: false,
		ast.AccessAccount:  true,
		ast.AccessPublic:   true,
	}

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {
			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
	                  contract A {

                          struct B {
                              %s let field: Int

                              init() {
                                  self.field = 42
                              }
                          }
                      }

                      contract C {
                          fun useB() {
                              let b = A.B()
                              b.field
                          }
                      }
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestCheckAccessOtherContractInnerStructInterfaceField(t *testing.T) {

	t.Parallel()

	tests := map[ast.Access]bool{
		ast.AccessPrivate:  false,
		ast.AccessContract: false,
		ast.AccessAccount:  true,
		ast.AccessPublic:   true,
	}

	for access, expectSuccess := range tests {

		t.Run(access.Keyword(), func(t *testing.T) {
			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
	                  contract A {

                          struct interface B {
                              %[1]s let field: Int
                          }

                          struct BImpl: B {
                              %[1]s let field: Int

                              init() {
                                  self.field = 42
                              }
                          }
                      }

                      contract C {
                          fun useB() {
                              let b: AnyStruct{A.B} = A.BImpl()
                              b.field
                          }
                      }
	                `,
					access.Keyword(),
				),
			)

			if expectSuccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestCheckRestrictiveAccessModifier(t *testing.T) {

	t.Parallel()

	for _, access := range ast.AllAccesses {

		if access <= ast.AccessPrivate {
			continue
		}

		t.Run(access.Keyword(), func(t *testing.T) {

			t.Run("type requirement", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          pub contract interface CI {

                              pub resource R {

                                  %[1]s var x: Int
                              }
                          }

                          pub contract C: CI {

                              pub resource R {

                                  %[1]s var x: Int

                                  init () {
                                      self.x = 0
                                  }
                              }
                          }
                        `,
						access.Keyword(),
					),
				)

				require.NoError(t, err)
			})

			t.Run("interface", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          pub resource interface RI {

                              %[1]s var x: Int
                          }

                          pub resource R: RI {

                              %[1]s var x: Int

                              init () {
                                  self.x = 0
                              }
                          }
                        `,
						access.Keyword(),
					),
				)

				require.NoError(t, err)
			})

		})
	}
}

func TestCheckInvalidRestrictiveAccessModifier(t *testing.T) {

	t.Parallel()

	for _, access := range ast.AllAccesses {

		if access == ast.AccessNotSpecified ||
			access > ast.AccessPrivate {

			continue
		}

		t.Run(access.Keyword(), func(t *testing.T) {

			t.Run("type requirement", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          pub contract interface CI {

                              pub resource R {

                                  %[1]s var x: Int
                              }
                          }
                        `,
						access.Keyword(),
					),
				)

				expectInvalidAccessModifierError(t, err)
			})

			t.Run("interface", func(t *testing.T) {

				_, err := ParseAndCheck(t,
					fmt.Sprintf(
						`
                          pub resource interface RI {

                              %[1]s var x: Int
                          }
                        `,
						access.Keyword(),
					),
				)

				expectInvalidAccessModifierError(t, err)
			})
		})
	}
}
