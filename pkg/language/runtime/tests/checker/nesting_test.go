package checker

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

// TODO: add support for nested composite declarations

func TestCheckInvalidNestedCompositeDeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      contract TestContract {
          resource TestResource {}
      }
    `)

	errs := ExpectCheckerErrors(err, 2)

	// TODO: add support for contracts

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	// TODO: add support for nested composite declarations

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

}

func TestCheckInvalidNestedInterfaceDeclarations(t *testing.T) {
	RegisterTestingT(t)

	_, err := ParseAndCheck(`
      contract interface TestContract {
          resource TestResource {}
      }
    `)

	errs := ExpectCheckerErrors(err, 2)

	// TODO: add support for contracts

	Expect(errs[0]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))

	// TODO: add support for nested composite declarations

	Expect(errs[1]).
		To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
}
