package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/model/flow"
)

const deductTransactionFeeTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction(computationEffort: UInt64, 
		inclusionEffort: UInt64, 
		gasLimit: UFix64) {
  prepare(account: AuthAccount) {
 	FlowServiceAccount.deductTransactionFees(
			account: account,
            computationEffort: computationEffort,
            inclusionEffort: inclusionEffort,
            gasLimit: gasLimit)
  }
}
`

func deductTransactionFeeTransaction(accountAddress, serviceAddress flow.Address, gasLimit uint64) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deductTransactionFeeTransactionTemplate, serviceAddress))).
			AddAuthorizer(accountAddress).
			AddArgument(jsoncdc.MustEncode(cadence.UInt64(0))).
			AddArgument(jsoncdc.MustEncode(cadence.UInt64(0))).
			AddArgument(jsoncdc.MustEncode(cadence.UFix64(gasLimit))),
		0,
	)
}
