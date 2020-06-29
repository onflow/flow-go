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

func deductAccountCreationFeeTransaction(address flow.Address, restrictedAccountCreationEnabled bool) InvokableTransaction {
	var script string

	if restrictedAccountCreationEnabled {
		script = deductAccountCreationFeeWithAllowlistTransactionTemplate
	} else {
		script = deductAccountCreationFeeTransactionTemplate
	}

	// TODO: remove hard-coded chain
	chain := flow.Mainnet.Chain()

	tx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(script, chain.ServiceAddress()))).
		AddAuthorizer(address)

	return Transaction(tx)
}

func deductTransactionFeeTransaction(address flow.Address) InvokableTransaction {
	// TODO: remove hard-coded chain
	chain := flow.Mainnet.Chain()

	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deductTransactionFeeTransactionTemplate, chain.ServiceAddress()))).
			AddAuthorizer(address),
	)
}
