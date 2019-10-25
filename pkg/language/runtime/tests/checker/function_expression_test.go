package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckInvalidFunctionExpressionReturnValue(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let test = fun (): Int {
          return true
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckFunctionExpressionsAndScope(t *testing.T) {

	_, err := ParseAndCheck(t, `
       let x = 10

       // check first-class functions and scope inside them
       let y = (fun (x: Int): Int { return x })(42)
    `)

	assert.Nil(t, err)
}
