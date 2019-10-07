package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckFunctionConditions(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int) {
          pre {
              x != 0
          }
          post {
              x == 0
          }
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionPreConditionReference(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int) {
          pre {
              y == 0
          }
          post {
              z == 0
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.Equal(t, errs[0].(*sema.NotDeclaredError).Name, "y")

	assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	assert.Equal(t, errs[1].(*sema.NotDeclaredError).Name, "z")
}

func TestCheckInvalidFunctionNonBoolCondition(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int) {
          pre {
              1
          }
          post {
              2
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])

	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckFunctionPostConditionWithBefore(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int) {
          post {
              before(x) != 0
          }
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionPostConditionWithBeforeAndNoArgument(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int) {
          post {
              before() != 0
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.ArgumentCountError{}, errs[0])

	assert.IsType(t, &sema.InvalidBinaryOperandsError{}, errs[1])

}

func TestCheckInvalidFunctionPreConditionWithBefore(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int) {
          pre {
              before(x) != 0
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.Equal(t, errs[0].(*sema.NotDeclaredError).Name, "before")
}

func TestCheckInvalidFunctionWithBeforeVariableAndPostConditionWithBefore(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int) {
          post {
              before(x) == 0
          }
          let before = 0
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

func TestCheckFunctionWithBeforeVariable(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int) {
          let before = 0
      }
	`)

	assert.Nil(t, err)
}

func TestCheckFunctionPostCondition(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: Int): Int {
          post {
              y == 0
          }
          let y = x
          return y
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionPreConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          pre {
              result == 0
          }
          return 0
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.Equal(t, errs[0].(*sema.NotDeclaredError).Name, "result")
}

func TestCheckInvalidFunctionPostConditionWithResultWrongType(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          post {
              result == true
          }
          return 0
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidBinaryOperandsError{}, errs[0])
}

func TestCheckFunctionPostConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          post {
              result == 0
          }
          return 0
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionPostConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          post {
              result == 0
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	assert.Equal(t, errs[0].(*sema.NotDeclaredError).Name, "result")
}

func TestCheckFunctionWithoutReturnTypeAndLocalResultAndPostConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          post {
              result == 0
          }
          let result = 0
      }
	`)

	assert.Nil(t, err)
}

func TestCheckFunctionWithoutReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(result: Int) {
          post {
              result == 0
          }
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionWithReturnTypeAndLocalResultAndPostConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): Int {
          post {
              result == 2
          }
          let result = 1
          return result * 2
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
}

// TODO: should this be invalid?
func TestCheckFunctionWithReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(result: Int): Int {
          post {
              result == 2
          }
          return result * 2
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionPostConditionWithFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          post {
              (fun (): Int { return 2 })() == 2
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.FunctionExpressionInConditionError{}, errs[0])
}

func TestCheckFunctionPostConditionWithMessageUsingStringLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          post {
             1 == 2: "nope"
          }
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidFunctionPostConditionWithMessageUsingBooleanLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          post {
             1 == 2: true
          }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckFunctionPostConditionWithMessageUsingResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(): String {
          post {
             1 == 2: result
          }
          return ""
      }
	`)

	assert.Nil(t, err)
}

func TestCheckFunctionPostConditionWithMessageUsingBefore(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: String) {
          post {
             1 == 2: before(x)
          }
      }
	`)

	assert.Nil(t, err)
}

func TestCheckFunctionPostConditionWithMessageUsingParameter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: String) {
          post {
             1 == 2: x
          }
      }
	`)

	assert.Nil(t, err)
}
