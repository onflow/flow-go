package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const deductTransactionFeeTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction {
  prepare(account: AuthAccount) {
 	FlowServiceAccount.deductTransactionFee(account)
  }
}
`

func deductTransactionFeeTransaction(accountAddress, serviceAddress flow.Address) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deductTransactionFeeTransactionTemplate, serviceAddress))).
			AddAuthorizer(accountAddress),
		0,
	)
}
