package blueprints

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

func DeductTransactionFeeTransaction(accountAddress, serviceAddress flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deductTransactionFeeTransactionTemplate, serviceAddress))).
		AddAuthorizer(accountAddress)
}
