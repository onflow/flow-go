package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

// TODO: add support for nested composite declarations

func TestCheckNestedCompositeDeclarations(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract TestContract {
          resource TestResource {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	// TODO: add support for contracts

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckNestedCompositeInterfaceDeclarations(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract interface TestContract {
          resource TestResource {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	// TODO: add support for contracts

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
}

func TestCheckInvalidNestedCompositeDeclarationInComposite(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract interface TestContract {
          contract NestedContract {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	// TODO: add support for contracts

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	assert.IsType(t, &sema.InvalidNestedDeclarationError{}, errs[1])
}

func TestCheckInvalidNestedCompositeDeclarations(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource TestContract {
          resource TestResource {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidNestedDeclarationError{}, errs[0])
}

func TestCheckInvalidNestedInterfaceDeclarations(t *testing.T) {

	_, err := ParseAndCheck(t, `
      resource interface TestContract {
          resource TestResource {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidNestedDeclarationError{}, errs[0])
}
