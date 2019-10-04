package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckInvalidWhileTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          while 1 {}
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckWhileTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          while true {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidWhileBlock(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          while true { x }
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckBreakStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       fun test() {
           while true {
               break
           }
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidBreakStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       fun test() {
           while true {
               fun () {
                   break
               }
           }
       }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ControlStatementError{}))
}

func TestCheckContinueStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       fun test() {
           while true {
               continue
           }
       }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidContinueStatement(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
       fun test() {
           while true {
               fun () {
                   continue
               }
           }
       }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.ControlStatementError{}))
}
