package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckIfStatementTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          if true {}
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidIfStatementTest(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          if 1 {}
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidIfStatementElse(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test() {
          if true {} else {
              x
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckIfStatementTestWithDeclaration(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int?): Int {
          if var y = x {
              return y
		  }
		  
		  return 0
      }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidIfStatementTestWithDeclarationReferenceInElse(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int?) {
          if var y = x {
              // ...
          } else {
              y
          }
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.NotDeclaredError{}))
}

func TestCheckIfStatementTestWithDeclarationNestedOptionals(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     fun test(x: Int??): Int? {
         if var y = x {
             return y
		 }
		 
		 return nil
     }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckIfStatementTestWithDeclarationNestedOptionalsExplicitAnnotation(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     fun test(x: Int??): Int? {
         if var y: Int? = x {
             return y
		 }
		 
		 return nil
     }
	`)

	Expect(err).
		To(Not(HaveOccurred()))
}

func TestCheckInvalidIfStatementTestWithDeclarationNonOptional(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
     fun test(x: Int) {
         if var y = x {
             // ...
		 }
		 
		 return
     }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}

func TestCheckInvalidIfStatementTestWithDeclarationSameType(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      fun test(x: Int?): Int? {
          if var y: Int? = x {
             return y
		  }
		  
		  return nil
      }
	`)

	errs := ExpectCheckerErrors(err, 1)

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.TypeMismatchError{}))
}
