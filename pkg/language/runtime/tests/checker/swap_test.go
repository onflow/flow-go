package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
)

func TestCheckInvalidUnknownDeclarationSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 1
          x <-> y
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckInvalidLeftConstantSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          let x = 2
          var y = 1
          x <-> y
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
}

func TestCheckInvalidRightConstantSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 2
          let y = 1
          x <-> y
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantError{}, errs[0])
}

func TestCheckSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 2
          var y = 3
          x <-> y
      }
    `)

	assert.Nil(t, err)
}

func TestCheckInvalidTypesSwap(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 2
          var y = "1"
          x <-> y
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidTypesSwap2(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = "2"
          var y = 1
          x <-> y
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckInvalidSwapTargetExpressionLeft(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 1
          f() <-> x
      }

      fun f(): Int {
          return 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[0])
}

func TestCheckInvalidSwapTargetExpressionRight(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x = 1
          x <-> f()
      }

      fun f(): Int {
          return 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[0])
}

func TestCheckInvalidSwapTargetExpressions(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          f() <-> f()
      }

      fun f(): Int {
          return 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[0])
	assert.IsType(t, &sema.InvalidSwapExpressionError{}, errs[1])
}

func TestCheckSwapOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          var x: Int? = 2
          var y: Int? = nil
          x <-> y
      }
    `)

	assert.Nil(t, err)
}

func TestCheckSwapResourceArrayElementAndVariable(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs <- [<-create X()]
          var x <- create X()
          x <-> xs[0]
          destroy x
          destroy xs
      }
    `)

	assert.Nil(t, err)
}

func TestCheckSwapResourceArrayElements(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      fun test() {
          let xs <- [<-create X(), <-create X()]
          xs[0] <-> xs[1]
          destroy xs
      }
    `)

	assert.Nil(t, err)
}

func TestCheckSwapResourceFields(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource X {}

      resource Y {
          var x: <-X

          init(x: <-X) {
              self.x <- x
          }

          destroy() {
              destroy self.x
          }
      }

      fun test() {
          let y1 <- create Y(x: <-create X())
          let y2 <- create Y(x: <-create X())
          y1.x <-> y2.x
          destroy y1
          destroy y2
      }
    `)

	assert.Nil(t, err)
}

// TestCheckInvalidSwapConstantResourceFields tests that it is invalid
// to swap fields which are constant (`let`)
//
func TestCheckInvalidSwapConstantResourceFields(t *testing.T) {

	for i := 0; i < 2; i += 1 {

		first := "var"
		second := "let"

		if i == 1 {
			first = "let"
			second = "var"
		}

		testName := fmt.Sprintf("%s_%s", first, second)

		t.Run(testName, func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  resource X {}

                  resource Y {
                      %[1]s x: <-X

                      init(x: <-X) {
                          self.x <- x
                      }

                      destroy() {
                          destroy self.x
                      }
                  }

                  resource Z {
                      %[2]s x: <-X

                      init(x: <-X) {
                          self.x <- x
                      }

                      destroy() {
                          destroy self.x
                      }
                  }

                  fun test() {
                      let y <- create Y(x: <-create X())
                      let z <- create Z(x: <-create X())
                      y.x <-> z.x
                      destroy y
                      destroy z
                  }
                `,
				first,
				second,
			))

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
		})
	}
}
