package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
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

	assert.IsType(t,
		&sema.MissingReturnStatementError{},
		errs[0],
	)
}
