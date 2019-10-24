package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckConditionalExpressionTest(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = true ? 1 : 2
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidConditionalExpressionTest(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = 1 ? 2 : 3
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidConditionalExpressionElse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = true ? 2 : y
      }
	`)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])

	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckInvalidConditionalExpressionTypes(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = true ? 2 : false
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

// TODO: return common super type for conditional
func TestCheckInvalidAnyConditional(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x: Any = true
      let y = true ? 1 : x
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}
