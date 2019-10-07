package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckIfStatementTest(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          if true {}
      }
	`)

	assert.Nil(t, err)
}

func TestCheckIfStatementScoping(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          if true {
              let x = 1
          }
          x
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidIfStatementTest(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          if 1 {}
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidIfStatementElse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          if true {} else {
              x
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckIfStatementTestWithDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int?): Int {
          if var y = x {
              return y
		  }
		  
		  return 0
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidIfStatementTestWithDeclarationReferenceInElse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int?) {
          if var y = x {
              // ...
          } else {
              y
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckIfStatementTestWithDeclarationNestedOptionals(t *testing.T) {

	_, err := ParseAndCheck(t, `
     fun test(x: Int??): Int? {
         if var y = x {
             return y
		 }
		 
		 return nil
     }
	`)

	assert.Nil(t, err)
}

func TestCheckIfStatementTestWithDeclarationNestedOptionalsExplicitAnnotation(t *testing.T) {

	_, err := ParseAndCheck(t, `
     fun test(x: Int??): Int? {
         if var y: Int? = x {
             return y
		 }
		 
		 return nil
     }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidIfStatementTestWithDeclarationNonOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
     fun test(x: Int) {
         if var y = x {
             // ...
		 }
		 
		 return
     }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidIfStatementTestWithDeclarationSameType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int?): Int? {
          if var y: Int? = x {
             return y
		  }
		  
		  return nil
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}
