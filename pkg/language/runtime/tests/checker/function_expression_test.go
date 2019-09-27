package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckInvalidFunctionExpressionReturnValue(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      let test = fun (): Int {
          return true
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckFunctionExpressionsAndScope(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       let x = 10

       // check first-class functions and scope inside them
       let y = (fun (x: Int): Int { return x })(42)
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}
