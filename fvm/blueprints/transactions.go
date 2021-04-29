package blueprints

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/model/flow"
)

const systemChunkTransactionTemplate = `
import FlowServiceAccount from 0x%s
transaction {
  execute {
    // TODO: replace with call to service account heartbeat
 	log("pulse")
  }
} 
`

// SystemChunkTransaction creates and returns the transaction corresponding to the system chunk
// at the specified service address.
func SystemChunkTransaction(serviceAddress flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(systemChunkTransactionTemplate, serviceAddress)))
}

const initAccountTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction(restrictedAccountCreationEnabled: Bool) {
  prepare(newAccount: AuthAccount, payerAccount: AuthAccount) {
    if restrictedAccountCreationEnabled && !FlowServiceAccount.isAccountCreator(payerAccount.address) {
	  panic("Account not authorized to create accounts")
    }

    FlowServiceAccount.setupNewAccount(newAccount: newAccount, payer: payerAccount)
  }
}
`

func InitAccountTransaction(
	payerAddress flow.Address,
	accountAddress flow.Address,
	serviceAddress flow.Address,
	restrictedAccountCreationEnabled bool,
) *flow.TransactionBody {
	arg, err := jsoncdc.Encode(cadence.NewBool(restrictedAccountCreationEnabled))
	if err != nil {
		// this should not fail! It simply encodes a boolean
		panic(fmt.Errorf("cannot json encode cadence boolean argument: %w", err))
	}

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(initAccountTransactionTemplate, serviceAddress))).
		AddAuthorizer(accountAddress).
		AddAuthorizer(payerAddress).
		AddArgument(arg)
}

const deductTransactionFeeTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction {
  prepare(account: AuthAccount) {
 	FlowServiceAccount.deductTransactionFee(account)
  }
}
`

func DeductTransactionFeeTransaction(accountAddress, serviceAddress flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deductTransactionFeeTransactionTemplate, serviceAddress))).
		AddAuthorizer(accountAddress)
}
