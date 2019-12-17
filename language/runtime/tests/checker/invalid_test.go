package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckSpuriousIdentifierAssignmentInvalidValueTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          fun test() {
              var x = 1
              x = y
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousIdentifierAssignmentInvalidTargetTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          fun test() {
              var x: X = 1
              x = 1
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousIndexAssignmentInvalidValueTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          fun test() {
              let values: {String: Int} = {}
              values["x"] = x
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousIndexAssignmentInvalidElementTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          fun test() {
              let values: {String: X} = {}
              values["x"] = 1
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousMemberAssignmentInvalidValueTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          struct X {
              var x: Int
              init() {
                  self.x = y
              }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousMemberAssignmentInvalidMemberTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
         struct X {
              var y: Y
              init() {
                  self.y = 0
              }
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousReturnWithInvalidValueTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          fun test(): Int {
              return x
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousReturnWithInvalidReturnTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          fun test(): X {
              return 1
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousCastWithInvalidTargetTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          let y = 1 as X
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckSpuriousCastWithInvalidValueTypeMismatch(t *testing.T) {

	_, err := ParseAndCheck(t,
		`
          let y = x as Int
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}
