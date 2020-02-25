package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckOptionalChainingNonOptionalFieldRead(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      struct Test {
          let x: Int

          init(x: Int) {
              self.x = x
          }
      }

      let test: Test? = Test(x: 1)
      let x = test?.x
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.IntType{}},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckOptionalChainingOptionalFieldRead(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      struct Test {
          let x: Int?

          init(x: Int?) {
              self.x = x
          }
      }

      let test: Test? = Test(x: 1)
      let x = test?.x
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.OptionalType{Type: &sema.IntType{}},
		checker.GlobalValues["x"].Type,
	)
}

func TestCheckOptionalChainingFunctionRead(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      struct Test {
          fun x(): Int {
              return 42
          }
      }

      let test: Test? = Test()
      let x = test?.x
    `)

	require.NoError(t, err)

	assert.True(t,
		checker.GlobalValues["x"].Type.Equal(
			&sema.OptionalType{
				Type: &sema.FunctionType{
					ReturnTypeAnnotation: &sema.TypeAnnotation{
						Type: &sema.IntType{},
					},
				},
			},
		),
	)
}

func TestCheckOptionalChainingFunctionCall(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      struct Test {
          fun x(): Int {
              return 42
          }
      }

      let test: Test? = Test()
      let x = test?.x()
    `)

	require.NoError(t, err)

	assert.True(t,
		checker.GlobalValues["x"].Type.Equal(
			&sema.OptionalType{Type: &sema.IntType{}},
		),
	)
}

func TestCheckInvalidOptionalChainingNonOptional(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct Test {
          let x: Int

          init(x: Int) {
              self.x = x
          }
      }

      let test = Test(x: 1)
      let x = test?.x
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidOptionalChainingError{}, errs[0])
}

func TestCheckInvalidOptionalChainingFieldAssignment(t *testing.T) {

	_, err := ParseAndCheck(t, `
      struct Test {
          var x: Int
          init(x: Int) {
              self.x = x
          }
      }

      fun test() {
          let test: Test? = Test(x: 1)
          test?.x = 2
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.UnsupportedOptionalChainingAssignmentError{}, errs[0])
}
