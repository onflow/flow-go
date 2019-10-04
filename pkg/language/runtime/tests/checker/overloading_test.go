package checker

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/pkg/language/runtime/tests/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckInvalidCompositeInitializerOverloading(t *testing.T) {

	for _, kind := range common.CompositeKinds {
		t.Run(kind.Keyword(), func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
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

			errs := ExpectCheckerErrors(t, err, expectedErrorCount)

			assert.IsType(t, &sema.UnsupportedOverloadingError{}, errs[0])

			if kind != common.CompositeKindStructure {
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}
