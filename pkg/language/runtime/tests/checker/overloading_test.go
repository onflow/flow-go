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

			// TODO: add support for non-structure / non-resource declarations

			switch kind {
			case common.CompositeKindStructure, common.CompositeKindResource:
				errs := ExpectCheckerErrors(t, err, 1)

				assert.IsType(t, &sema.UnsupportedOverloadingError{}, errs[0])

			default:
				errs := ExpectCheckerErrors(t, err, 2)

				assert.IsType(t, &sema.UnsupportedOverloadingError{}, errs[0])
				assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[1])
			}
		})
	}
}
