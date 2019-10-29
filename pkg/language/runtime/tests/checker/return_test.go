package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckMissingReturnStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingReturnStatementError{}, errs[0])
}

func TestCheckMissingReturnStatementInterfaceFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
        struct interface Test {
            fun test(x: Int): Int {
                pre {
                    x != 0
                }
            }
        }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidMissingReturnStatementStructFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
        struct Test {
            pub(set) var foo: Int

            init(foo: Int) {
                self.foo = foo
            }

            pub fun getFoo(): Int {
                if 2 > 1 {
                    return 0
                }
            }
        }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.MissingReturnStatementError{}, errs[0])
}

func assertExits(t *testing.T, body string, exits bool) {

}

type exitTest struct {
	body              string
	exits             bool
	valueDeclarations map[string]sema.ValueDeclaration
}

func testExits(t *testing.T, tests []exitTest) {
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			code := fmt.Sprintf("fun test(): Any {%s}", test.body)
			_, err := ParseAndCheckWithOptions(
				t,
				code,
				ParseAndCheckOptions{
					Options: []sema.Option{
						sema.WithPredeclaredValues(test.valueDeclarations),
					},
				},
			)

			if test.exits {
				assert.Nil(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.MissingReturnStatementError{}, errs[0])
			}
		})
	}
}

func TestCheckReturnStatementExits(t *testing.T) {
	testExits(
		t, []exitTest{
			{
				body:  "return 1",
				exits: true,
			},
			{
				body:  "return",
				exits: true,
			},
		},
	)
}

func TestCheckIfStatementExits(t *testing.T) {
	testExits(
		t,
		[]exitTest{
			{
				body: `
                  if true {
                      return 1
                  }
                `,
				exits: false,
			},
			{
				body: `
                  var x = 1
                  if true {
                      x = 2
                  } else {
                      return 2
                  }
                `,
				exits: false,
			},
			{
				body: `
                  var x = 1
                  if false {
                      x = 2
                  } else {
                      return 2
                  }
                `,
				exits: false,
			},
			{
				body: `
                  if true {
                      if true {
                          return 1
                      }
                  }
                `,
				exits: false,
			},
			{
				body: `
                  if 2 > 1 {
                      return 1
                  }
                `,
				exits: false,
			},
			{
				body: `
                  if 2 > 1 {
                      return 1
                  } else {
                      return 2
                  }
                `,
				exits: true,
			},
			{
				body: `
                  if 2 > 1 {
                      return 1
                  }
                  return 2
                `,
				exits: true,
			},
		},
	)
}

func TestCheckWhileStatementExits(t *testing.T) {
	testExits(
		t,
		[]exitTest{
			{
				body: `
                  var x = 1
                  var y = 2
                  while true {
                      x = y
                  }
                `,
				exits: false,
			},
			{
				body: `
                  var x = 1
                  var y = 2
                  while true {
                      x = y
                      break
                  }
                `,
				exits: false,
			},
			{
				body: `
                  var x = 1
                  var y = 2
                  while 1 > 2 {
                      x = y
                  }
                `,
				exits: false,
			},
			{
				body: `
                  var x = 1
                  var y = 2
                  while 1 > 2 {
                      x = y
                      break
                  }
                `,
				exits: false,
			},
			{
				body: `
                  while 2 > 1 {
                      return
                  }
                `,
				exits: false,
			},
			{
				body: `
                  var x = 0
                  while x < 10 {
                      return x
                  }
                `,
				exits: false,
			},
			{
				body: `
                  while true {
                      return
                  }
                `,
				exits: false,
			},
			{
				body: `
                  while true {
                      break
                  }
                `,
				exits: false,
			},
		},
	)
}

func TestCheckNeverInvocationExits(t *testing.T) {
	valueDeclarations := stdlib.StandardLibraryFunctions{
		stdlib.PanicFunction,
	}.ToValueDeclarations()

	testExits(
		t,
		[]exitTest{
			{
				body: `
                  panic("")
                `,
				exits:             true,
				valueDeclarations: valueDeclarations,
			},
			{
				body: `
                  if panic("") {}
                `,
				exits:             true,
				valueDeclarations: valueDeclarations,
			},
			{
				body: `
                  while panic("") {}
                `,
				exits:             true,
				valueDeclarations: valueDeclarations,
			},
			{
				body: `
                  let x: Int? = 1
                  let y = x ?? panic("")
                `,
				exits:             false,
				valueDeclarations: valueDeclarations,
			},
			{
				body: `
                  false || panic("")
                `,
				exits:             false,
				valueDeclarations: valueDeclarations,
			},
		},
	)
}

// TestCheckNestedFunctionExits tests if a function with a return statement
// nested inside another function does not influence the containing function
//
func TestCheckNestedFunctionExits(t *testing.T) {
	testExits(
		t,
		[]exitTest{
			{
				body: `
                  fun (): Int {
                      return 1
                  }
                `,
				// NOTE: inner function returns, but outer does not
				exits: false,
			},
		},
	)
}
