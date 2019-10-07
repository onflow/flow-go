package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TODO: add support for nested composite declarations

func TestCheckInvalidNestedCompositeDeclarations(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract TestContract {
          resource TestResource {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	// TODO: add support for contracts

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	// TODO: add support for nested composite declarations

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])

}

func TestCheckInvalidNestedInterfaceDeclarations(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract interface TestContract {
          resource TestResource {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 2)

	// TODO: add support for contracts

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])

	// TODO: add support for nested composite declarations

	assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
}
