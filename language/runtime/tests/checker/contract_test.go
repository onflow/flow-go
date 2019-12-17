package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckInvalidContractAccountField(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {
          let account: Account

          init(account: Account) {
              self.account = account
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckInvalidContractAccountFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {
          fun account() {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckContractAccountFieldUse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {

          init() {
              self.account.address
          }
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidContractAccountFieldInitialization(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {

          init(account: Account) {
              self.account = account
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
}

func TestCheckInvalidContractAccountFieldAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {}

      let test = Test.account
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
}

func TestCheckContractAccountFieldUseInitialized(t *testing.T) {

	code := `
      contract Test {
          let address: Address

          init() {
              // field 'account' can be used, as it is considered initialized
              self.address = self.account.address
          }

          fun test(): Address {
              return self.account.address
          }
      }

      let address1 = Test.address
      let address2 = Test.test()
    `
	_, err := ParseAndCheck(t, code)

	require.NoError(t, err)
}

func TestCheckInvalidContractMoveToFunction(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              use(%s)
                          }
                      }

                      fun use(_ c: C) {}
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveInVariableDeclaration(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let x = %s
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveReturnFromFunction(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test(): C {
                              return %s
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveIntoArrayLiteral(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let txs = [%s]
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveIntoDictionaryLiteral(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let txs = {"C": %s}
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}
