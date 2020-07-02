package fvm

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

const deductAccountCreationFeeTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction {
  prepare(account: AuthAccount) {
    FlowServiceAccount.deductAccountCreationFee(account)
  }
}
`

const deductAccountCreationFeeWithAllowlistTransactionTemplate = `
import FlowServiceAccount from 0x%s
	
transaction {
  prepare(account: AuthAccount) {
    if !FlowServiceAccount.isAccountCreator(account.address) {
	  panic("Account not authorized to create accounts")
    }

    FlowServiceAccount.deductAccountCreationFee(account)
  }
}
`

const deductTransactionFeeTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction {
  prepare(account: AuthAccount) {
 	FlowServiceAccount.deductTransactionFee(account)
  }
}
`

func deductAccountCreationFeeTransaction(
	accountAddress, serviceAddress flow.Address,
	restrictedAccountCreationEnabled bool,
) InvokableTransaction {
	var script string

	if restrictedAccountCreationEnabled {
		script = deductAccountCreationFeeWithAllowlistTransactionTemplate
	} else {
		script = deductAccountCreationFeeTransactionTemplate
	}

	tx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(script, serviceAddress))).
		AddAuthorizer(accountAddress)

	return Transaction(tx)
}

func deductTransactionFeeTransaction(accountAddress, serviceAddress flow.Address) InvokableTransaction {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deductTransactionFeeTransactionTemplate, serviceAddress))).
			AddAuthorizer(accountAddress),
	)
}
