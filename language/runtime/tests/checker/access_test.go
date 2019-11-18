package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
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

	for _, variableKind := range []ast.VariableKind{
		ast.VariableKindConstant,
		ast.VariableKindVariable,
	} {

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
