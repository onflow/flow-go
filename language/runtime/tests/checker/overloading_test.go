package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckInvalidCompositeInitializerOverloading(t *testing.T) {

	interfacePossibilities := []bool{true, false}

	for _, kind := range common.CompositeKinds {
		for _, isInterface := range interfacePossibilities {

			interfaceKeyword := ""
			body := ""
			if isInterface {
				interfaceKeyword = "interface"
			} else {
				body = "{}"
			}

			testName := fmt.Sprintf("%s_%s",
				kind.Keyword(),
				interfaceKeyword,
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheck(t, fmt.Sprintf(`
                      %[1]s %[2]s X {
                          init() %[3]s
                          init(y: Int) %[3]s
                      }
                    `,
					kind.Keyword(),
					interfaceKeyword,
					body,
				))

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
}

func TestCheckInvalidResourceDestructorOverloading(t *testing.T) {

	interfacePossibilities := []bool{true, false}

	for _, isInterface := range interfacePossibilities {

		interfaceKeyword := ""
		body := ""
		if isInterface {
			interfaceKeyword = "interface"
		} else {
			body = "{}"
		}

		t.Run(interfaceKeyword, func(t *testing.T) {

			_, err := ParseAndCheck(t, fmt.Sprintf(`
                  resource %[1]s X {
                      destroy() %[2]s
                      destroy(y: Int) %[2]s
                  }
                `,
				interfaceKeyword,
				body,
			))

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.UnsupportedOverloadingError{}, errs[0])
		})
	}
}
