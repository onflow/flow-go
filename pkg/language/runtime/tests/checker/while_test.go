package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckInvalidWhileTest(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          while 1 {}
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckWhileTest(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          while true {}
      }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidWhileBlock(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun test() {
          while true { x }
      }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
}

func TestCheckBreakStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
       fun test() {
           while true {
               break
           }
       }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidBreakStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
       fun test() {
           while true {
               fun () {
                   break
               }
           }
       }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ControlStatementError{}, errs[0])
}

func TestCheckContinueStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
       fun test() {
           while true {
               continue
           }
       }
	`)

	assert.Nil(t, err)
}

func TestCheckInvalidContinueStatement(t *testing.T) {

	_, err := ParseAndCheck(t, `
       fun test() {
           while true {
               fun () {
                   continue
               }
           }
       }
	`)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.ControlStatementError{}, errs[0])
}
