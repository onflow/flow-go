package checker

import (
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

          init(account: Account) {
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

      let test = Test().account
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

      let address1 = Test().address
      let address2 = Test().test()
    `
	_, err := ParseAndCheck(t, code)

	require.NoError(t, err)
}
