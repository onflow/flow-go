package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
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

	require.NoError(t, err)
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
	assert.Equal(t,
		"y",
		errs[0].(*sema.NotDeclaredError).Name,
	)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	assert.Equal(t,
		"z",
		errs[1].(*sema.NotDeclaredError).Name,
	)
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

	checker, err := ParseAndCheck(t, `
      fun test(x: Int) {
          post {
              before(x) != 0
          }
      }
    `)

	require.NoError(t, err)

	assert.Len(t, checker.Elaboration.VariableDeclarationValueTypes, 1)
	assert.Len(t, checker.Elaboration.VariableDeclarationTargetTypes, 1)
}

func TestCheckFunctionPostConditionWithBeforeNotDeclaredUse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          post {
              before(x) != 0
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
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
	assert.Equal(t,
		"before",
		errs[0].(*sema.NotDeclaredError).Name,
	)
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

	require.NoError(t, err)
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

	require.NoError(t, err)
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
	assert.Equal(t,
		"result",
		errs[0].(*sema.NotDeclaredError).Name,
	)
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

	require.NoError(t, err)
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
	assert.Equal(t,
		"result",
		errs[0].(*sema.NotDeclaredError).Name,
	)
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

	require.NoError(t, err)
}

func TestCheckFunctionWithoutReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(result: Int) {
          post {
              result == 0
          }
      }
    `)

	require.NoError(t, err)
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

func TestCheckInvalidFunctionWithReturnTypeAndResultParameterAndPostConditionWithResult(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(result: Int): Int {
          post {
              result == 2
          }
          return result * 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.RedeclarationError{}, errs[0])
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

	require.NoError(t, err)
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

	require.NoError(t, err)
}

func TestCheckFunctionPostConditionWithMessageUsingBefore(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: String) {
          post {
             1 == 2: before(x)
          }
      }
    `)

	require.NoError(t, err)
}

func TestCheckFunctionPostConditionWithMessageUsingParameter(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test(x: String) {
          post {
             1 == 2: x
          }
      }
    `)

	require.NoError(t, err)
}
