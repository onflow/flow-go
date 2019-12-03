package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckCompositeDeclarationNesting(t *testing.T) {

	interfacePossibilities := []bool{true, false}

	for _, outerComposite := range common.CompositeKinds {
		for _, outerIsInterface := range interfacePossibilities {
			for _, innerComposite := range common.CompositeKinds {
				for _, innerIsInterface := range interfacePossibilities {

					outer := outerComposite.DeclarationKind(outerIsInterface)
					inner := innerComposite.DeclarationKind(innerIsInterface)

					testName := fmt.Sprintf("%s/%s", outer, inner)

					t.Run(testName, func(t *testing.T) {

						code := fmt.Sprintf(`
                              %s Outer {
                                  %s Inner {}
                              }
                            `,
							outer.Keywords(),
							inner.Keywords(),
						)
						_, err := ParseAndCheck(t, code)

						switch outerComposite {
						case common.CompositeKindContract:

							switch innerComposite {
							case common.CompositeKindContract:
								if outerIsInterface {
									errs := ExpectCheckerErrors(t, err, 2)

									assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
									assert.IsType(t, &sema.InvalidNestedDeclarationError{}, errs[1])

								} else if !innerIsInterface {
									errs := ExpectCheckerErrors(t, err, 1)

									assert.IsType(t, &sema.InvalidNestedDeclarationError{}, errs[0])
								}

							case common.CompositeKindResource, common.CompositeKindStructure:
								// TODO: add support for contract interfaces

								if outerIsInterface {
									errs := ExpectCheckerErrors(t, err, 1)

									assert.IsType(t, &sema.UnsupportedDeclarationError{}, errs[0])
								} else {
									require.NoError(t, err)
								}

							default:
								t.Errorf("unknown outer composite kind %s", outerComposite)
							}

						case common.CompositeKindResource, common.CompositeKindStructure:
							errs := ExpectCheckerErrors(t, err, 1)

							assert.IsType(t, &sema.InvalidNestedDeclarationError{}, errs[0])

						default:
							t.Errorf("unknown outer composite kind %s", outerComposite)
						}
					})
				}
			}
		}
	}
}
