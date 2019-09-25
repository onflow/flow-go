package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckMissingReturnStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(): Int {}
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingReturnStatementError{}))
}

func TestCheckMissingReturnStatementInterfaceFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      	struct interface Test {
			fun test(x: Int): Int {
				pre {
					x != 0
				}
			}
		}
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckMissingReturnStatementStructFunction(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
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

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.MissingReturnStatementError{}))
}
