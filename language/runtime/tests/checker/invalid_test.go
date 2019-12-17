package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

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
