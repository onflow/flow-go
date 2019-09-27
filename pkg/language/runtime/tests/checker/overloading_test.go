package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCheckInvalidCompositeInitializerOverloading(t *testing.T) {
	RegisterTestingT(t)

	for _, kind := range common.CompositeKinds {
		_, err := ParseAndCheck(fmt.Sprintf(`
          %s X {
              init() {}
              init(y: Int) {}
          }
	    `, kind.Keyword()))

		// TODO: add support for non-structure declarations

		expectedErrorCount := 1
		if kind != common.CompositeKindStructure {
			expectedErrorCount += 1
		}

		errs := ExpectCheckerErrors(err, expectedErrorCount)

		Expect(errs[0]).
			To(BeAssignableToTypeOf(&sema.UnsupportedOverloadingError{}))

		if kind != common.CompositeKindStructure {
			Expect(errs[1]).
				To(BeAssignableToTypeOf(&sema.UnsupportedDeclarationError{}))
		}
	}
}
