package fvm

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

const systemChunkTransactionTemplate = `
import FlowServiceAccount from 0x%s
transaction {
  execute() {
 	FlowServiceAccount.pulse()
  }
}
`

// SystemChunkTransaction creates and returns the transaction corresponding to the system chunk
// at the specified service address.
func SystemChunkTransaction(serviceAddress flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(systemChunkTransactionTemplate, serviceAddress)))
}
