package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckAccessModifierCompositeFunctionDeclaration(t *testing.T) {

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

func TestCheckAccessModifierCompositeConstantFieldDeclaration(t *testing.T) {

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

func TestCheckAccessModifierCompositeVariableFieldDeclaration(t *testing.T) {

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

func TestCheckAccessModifierGlobalFunctionDeclaration(t *testing.T) {

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

func TestCheckAccessModifierGlobalVariableDeclaration(t *testing.T) {

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

func TestCheckAccessModifierGlobalConstantDeclaration(t *testing.T) {

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

func TestCheckAccessModifierLocalVariableDeclaration(t *testing.T) {

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

func TestCheckAccessModifierLocalOptionalBinding(t *testing.T) {

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

func TestCheckAccessModifierLocalFunctionDeclaration(t *testing.T) {

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

func TestCheckAccessModifierGlobalCompositeDeclaration(t *testing.T) {

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

func TestCheckAccessImportGlobalValue(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]func(*testing.T, error){
		sema.AccessCheckModeStrict: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 2)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "x")

			require.IsType(t, &sema.InvalidAccessError{}, errs[1])
			assert.Equal(t, errs[1].(*sema.InvalidAccessError).Name, "z")
		},
		sema.AccessCheckModeNotSpecifiedRestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 2)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "x")

			require.IsType(t, &sema.InvalidAccessError{}, errs[1])
			assert.Equal(t, errs[1].(*sema.InvalidAccessError).Name, "z")
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 1)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "x")
		},
		sema.AccessCheckModeNone: func(t *testing.T, err error) {
			require.Nil(t, err)
		},
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
                      priv fun x() {}
                      pub fun y() {}
                      %s fun z() {}
                    `,
					lastAccessModifier,
				),
			}

			for _, variableKind := range ast.VariableKinds {

				tests = append(tests,
					fmt.Sprintf(
						`
                           priv %[1]s x = 1
                           pub %[1]s y = 2
                           %[2]s %[1]s z = 3
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
                       import x, y, z from "imported"
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
			assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "A")

			require.IsType(t, &sema.InvalidAccessError{}, errs[1])
			assert.Equal(t, errs[1].(*sema.InvalidAccessError).Name, "C")
		},
		sema.AccessCheckModeNotSpecifiedRestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 2)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "A")

			require.IsType(t, &sema.InvalidAccessError{}, errs[1])
			assert.Equal(t, errs[1].(*sema.InvalidAccessError).Name, "C")
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 1)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "A")
		},
		sema.AccessCheckModeNone: func(t *testing.T, err error) {
			require.Nil(t, err)
		},
	}

	for _, compositeKind := range common.CompositeKinds {

		// TODO: add support for contracts
		if compositeKind == common.CompositeKindContract {
			continue
		}

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

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]int{
		sema.AccessCheckModeStrict: {
			ast.AccessPrivate: 1,
			ast.AccessPublic:  0,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified: 1,
			ast.AccessPrivate:      1,
			ast.AccessPublic:       0,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified: 0,
			ast.AccessPrivate:      1,
			ast.AccessPublic:       0,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified: 0,
			ast.AccessPrivate:      0,
			ast.AccessPublic:       0,
		},
	}

	for _, compositeKind := range common.CompositeKinds {

		// TODO: add support for contracts
		if compositeKind == common.CompositeKindContract {
			continue
		}

		for checkMode, checkModeTests := range checkModeTests {
			for access, expectedErrors := range checkModeTests {

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

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
                                  let test %[3]s %[4]s Test()
                                  test.test()
                                  %[5]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					if expectedErrors == 0 {
						assert.Nil(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, expectedErrors)

						for i := 0; i < expectedErrors; i++ {
							assert.IsType(t, &sema.InvalidAccessError{}, errs[i])
						}

						// TODO: check line numbers to ensure errors where for outside composite
					}
				})
			}
		}
	}
}

func TestCheckAccessInterfaceFunction(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]int{
		sema.AccessCheckModeStrict: {
			ast.AccessPrivate: 1,
			ast.AccessPublic:  0,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified: 1,
			ast.AccessPrivate:      1,
			ast.AccessPublic:       0,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified: 0,
			ast.AccessPrivate:      1,
			ast.AccessPublic:       0,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified: 0,
			ast.AccessPrivate:      0,
			ast.AccessPublic:       0,
		},
	}

	for _, compositeKind := range common.CompositeKinds {

		// TODO: add support for contracts
		if compositeKind == common.CompositeKindContract {
			continue
		}

		for checkMode, checkModeTests := range checkModeTests {
			for access, expectedErrors := range checkModeTests {

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

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
                                  let test: %[3]sTest %[4]s %[5]s TestImpl()
                                  test.test()
                                  %[6]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.Annotation(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					if expectedErrors == 0 {
						assert.Nil(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, expectedErrors)

						for i := 0; i < expectedErrors; i++ {
							assert.IsType(t, &sema.InvalidAccessError{}, errs[i])
						}

						// TODO: check line numbers to ensure errors where for outside composite
					}
				})
			}
		}
	}
}

func TestCheckAccessCompositeFieldRead(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]int{
		sema.AccessCheckModeStrict: {
			ast.AccessPrivate:        1,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   1,
			ast.AccessPrivate:        1,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        1,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        0,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
	}

	for _, compositeKind := range common.CompositeKinds {

		// TODO: add support for contracts
		if compositeKind == common.CompositeKindContract {
			continue
		}

		for checkMode, checkModeTests := range checkModeTests {
			for access, expectedErrors := range checkModeTests {

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

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
                                  let test %[3]s %[4]s Test()
                                  test.test
                                  %[5]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					if expectedErrors == 0 {
						assert.Nil(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, expectedErrors)

						for i := 0; i < expectedErrors; i++ {
							assert.IsType(t, &sema.InvalidAccessError{}, errs[i])
						}

						// TODO: check line numbers to ensure errors where for outside composite
					}
				})
			}
		}
	}
}

func TestCheckAccessInterfaceFieldRead(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]int{
		sema.AccessCheckModeStrict: {
			ast.AccessPrivate:        1,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   1,
			ast.AccessPrivate:        1,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        1,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        0,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
	}

	for _, compositeKind := range common.CompositeKinds {

		// TODO: add support for contracts
		if compositeKind == common.CompositeKindContract {
			continue
		}

		for checkMode, checkModeTests := range checkModeTests {
			for access, expectedErrors := range checkModeTests {

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(`
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
                                  let test: %[3]sTest %[4]s %[5]s TestImpl()
                                  test.test
                                  %[6]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.Annotation(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					if expectedErrors == 0 {
						assert.Nil(t, err)
					} else {
						errs := ExpectCheckerErrors(t, err, expectedErrors)

						for i := 0; i < expectedErrors; i++ {
							assert.IsType(t, &sema.InvalidAccessError{}, errs[i])
						}

						// TODO: check line numbers to ensure errors where for outside composite
					}
				})
			}
		}
	}
}

func TestCheckAccessCompositeFieldAssignmentAndSwap(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]int{
		sema.AccessCheckModeStrict: {
			ast.AccessPrivate:        4,
			ast.AccessPublic:         2,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   4,
			ast.AccessPrivate:        4,
			ast.AccessPublic:         2,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        4,
			ast.AccessPublic:         2,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        0,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
	}

	for _, compositeKind := range common.CompositeKinds {

		// TODO: add support for contracts
		if compositeKind == common.CompositeKindContract {
			continue
		}

		for checkMode, checkModeTests := range checkModeTests {
			for access, expectedErrors := range checkModeTests {

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(`
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
                                  let test %[3]s %[4]s Test()
                                  test.test = 3
                                  var temp = 4
                                  test.test <-> temp
                                  %[5]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					// TODO: check line numbers to ensure errors where for outside composite

					switch expectedErrors {
					case 0:
						assert.Nil(t, err)

					case 2:
						errs := ExpectCheckerErrors(t, err, 2)

						assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[0])
						assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])

					case 4:
						errs := ExpectCheckerErrors(t, err, 4)

						assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
						assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])
						assert.IsType(t, &sema.InvalidAccessError{}, errs[2])
						assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[3])

					default:
						panic(errors.NewUnreachableError())
					}
				})
			}
		}
	}
}

func TestCheckAccessInterfaceFieldWrite(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]int{
		sema.AccessCheckModeStrict: {
			ast.AccessPrivate:        4,
			ast.AccessPublic:         2,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   4,
			ast.AccessPrivate:        4,
			ast.AccessPublic:         2,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        4,
			ast.AccessPublic:         2,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        0,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
	}

	for _, compositeKind := range common.CompositeKinds {

		// TODO: add support for contracts
		if compositeKind == common.CompositeKindContract {
			continue
		}

		for checkMode, checkModeTests := range checkModeTests {
			for access, expectedErrors := range checkModeTests {

				testName := fmt.Sprintf(
					"%s/%s/%s",
					compositeKind.Keyword(),
					checkMode,
					access.Keyword(),
				)

				t.Run(testName, func(t *testing.T) {

					_, err := ParseAndCheckWithOptions(t,
						fmt.Sprintf(`
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
                                  let test: %[3]sTest %[4]s %[5]s TestImpl()
                                  test.test = 3
                                  var temp = 4
                                  test.test <-> temp
                                  %[6]s test
                              }
	                        `,
							compositeKind.Keyword(),
							access.Keyword(),
							compositeKind.Annotation(),
							compositeKind.TransferOperator(),
							compositeKind.ConstructionKeyword(),
							compositeKind.DestructionKeyword(),
						),
						ParseAndCheckOptions{
							Options: []sema.Option{
								sema.WithAccessCheckMode(checkMode),
							},
						},
					)

					// TODO: check line numbers to ensure errors where for outside composite

					switch expectedErrors {
					case 0:
						assert.Nil(t, err)

					case 2:
						errs := ExpectCheckerErrors(t, err, 2)

						assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[0])
						assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])

					case 4:
						errs := ExpectCheckerErrors(t, err, 4)

						assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
						assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])
						assert.IsType(t, &sema.InvalidAccessError{}, errs[2])
						assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[3])

					default:
						panic(errors.NewUnreachableError())
					}
				})
			}
		}
	}
}

func TestCheckAccessCompositeFieldVariableDeclarationWithSecondValue(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]int{
		sema.AccessCheckModeStrict: {
			ast.AccessPrivate:        2,
			ast.AccessPublic:         1,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   2,
			ast.AccessPrivate:        2,
			ast.AccessPublic:         1,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        2,
			ast.AccessPublic:         1,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        0,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
	}

	for checkMode, checkModeTests := range checkModeTests {
		for access, expectedErrors := range checkModeTests {

			testName := fmt.Sprintf(
				"%s/%s",
				checkMode,
				access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheckWithOptions(t,
					fmt.Sprintf(`
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

				// TODO: check line numbers to ensure errors where for outside composite

				switch expectedErrors {
				case 0:
					assert.Nil(t, err)

				case 1:
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[0])

				case 2:
					errs := ExpectCheckerErrors(t, err, 2)

					assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
					assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])

				default:
					panic(errors.NewUnreachableError())
				}
			})
		}
	}
}

func TestCheckAccessInterfaceFieldVariableDeclarationWithSecondValue(t *testing.T) {

	checkModeTests := map[sema.AccessCheckMode]map[ast.Access]int{
		sema.AccessCheckModeStrict: {
			ast.AccessPrivate:        2,
			ast.AccessPublic:         1,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedRestricted: {
			ast.AccessNotSpecified:   2,
			ast.AccessPrivate:        2,
			ast.AccessPublic:         1,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNotSpecifiedUnrestricted: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        2,
			ast.AccessPublic:         1,
			ast.AccessPublicSettable: 0,
		},
		sema.AccessCheckModeNone: {
			ast.AccessNotSpecified:   0,
			ast.AccessPrivate:        0,
			ast.AccessPublic:         0,
			ast.AccessPublicSettable: 0,
		},
	}

	for checkMode, checkModeTests := range checkModeTests {
		for access, expectedErrors := range checkModeTests {

			testName := fmt.Sprintf(
				"%s/%s",
				checkMode,
				access.Keyword(),
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheckWithOptions(t,
					fmt.Sprintf(`
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

				// TODO: check line numbers to ensure errors where for outside composite

				switch expectedErrors {
				case 0:
					assert.Nil(t, err)
				case 1:
					errs := ExpectCheckerErrors(t, err, 1)

					assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[0])

				case 2:
					errs := ExpectCheckerErrors(t, err, 2)

					assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
					assert.IsType(t, &sema.InvalidAssignmentAccessError{}, errs[1])
				default:
					panic(errors.NewUnreachableError())
				}
			})
		}
	}
}

func TestCheckAccessImportGlobalValueAssignmentAndSwap(t *testing.T) {

	worstCase := func(t *testing.T, err error) {
		errs := ExpectCheckerErrors(t, err, 8)

		require.IsType(t, &sema.InvalidAccessError{}, errs[0])
		assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "x")

		require.IsType(t, &sema.InvalidAccessError{}, errs[1])
		assert.Equal(t, errs[1].(*sema.InvalidAccessError).Name, "z")

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
		assert.Equal(t, errs[2].(*sema.AssignmentToConstantError).Name, "x")

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[3])
		assert.Equal(t, errs[3].(*sema.AssignmentToConstantError).Name, "y")

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[4])
		assert.Equal(t, errs[4].(*sema.AssignmentToConstantError).Name, "z")

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[5])
		assert.Equal(t, errs[5].(*sema.AssignmentToConstantError).Name, "x")

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[6])
		assert.Equal(t, errs[6].(*sema.AssignmentToConstantError).Name, "y")

		require.IsType(t, &sema.AssignmentToConstantError{}, errs[7])
		assert.Equal(t, errs[7].(*sema.AssignmentToConstantError).Name, "z")
	}

	checkModeTests := map[sema.AccessCheckMode]func(*testing.T, error){
		sema.AccessCheckModeStrict:                 worstCase,
		sema.AccessCheckModeNotSpecifiedRestricted: worstCase,
		sema.AccessCheckModeNotSpecifiedUnrestricted: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 7)

			require.IsType(t, &sema.InvalidAccessError{}, errs[0])
			assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "x")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[1])
			assert.Equal(t, errs[1].(*sema.AssignmentToConstantError).Name, "x")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
			assert.Equal(t, errs[2].(*sema.AssignmentToConstantError).Name, "y")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[3])
			assert.Equal(t, errs[3].(*sema.AssignmentToConstantError).Name, "z")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[4])
			assert.Equal(t, errs[4].(*sema.AssignmentToConstantError).Name, "x")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[5])
			assert.Equal(t, errs[5].(*sema.AssignmentToConstantError).Name, "y")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[6])
			assert.Equal(t, errs[6].(*sema.AssignmentToConstantError).Name, "z")
		},
		sema.AccessCheckModeNone: func(t *testing.T, err error) {
			errs := ExpectCheckerErrors(t, err, 6)

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
			assert.Equal(t, errs[0].(*sema.AssignmentToConstantError).Name, "x")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[1])
			assert.Equal(t, errs[1].(*sema.AssignmentToConstantError).Name, "y")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
			assert.Equal(t, errs[2].(*sema.AssignmentToConstantError).Name, "z")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[3])
			assert.Equal(t, errs[3].(*sema.AssignmentToConstantError).Name, "x")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[4])
			assert.Equal(t, errs[4].(*sema.AssignmentToConstantError).Name, "y")

			require.IsType(t, &sema.AssignmentToConstantError{}, errs[5])
			assert.Equal(t, errs[5].(*sema.AssignmentToConstantError).Name, "z")
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
				fmt.Sprintf(`
                   priv var x = 1
                   pub var y = 2
                   %s var z = 3
                `,
					lastAccessModifier,
				))

			require.Nil(t, err)

			_, err = ParseAndCheckWithOptions(t,
				`
                  import x, y, z from "imported"

                  pub fun test() {
                      x = 4
                      y = 5
                      z = 6

                      var tempX = 7
                      x <-> tempX

                      var tempY = 8
                      y <-> tempY

                      var tempZ = 9
                      z <-> tempZ
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
	assert.Equal(t, errs[0].(*sema.InvalidAccessError).Name, "x")

	require.IsType(t, &sema.ResourceCapturingError{}, errs[1])

	require.IsType(t, &sema.AssignmentToConstantError{}, errs[2])
	assert.Equal(t, errs[2].(*sema.AssignmentToConstantError).Name, "x")

	require.IsType(t, &sema.ResourceCapturingError{}, errs[3])

	require.IsType(t, &sema.AssignmentToConstantError{}, errs[4])
	assert.Equal(t, errs[4].(*sema.AssignmentToConstantError).Name, "y")
}
