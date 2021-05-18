package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const systemChunkTransactionTemplate = `
import FlowServiceAccount from 0x%s
transaction {
  prepare(serviceAccount: AuthAccount) { 

  }

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
		SetScript([]byte(fmt.Sprintf(systemChunkTransactionTemplate, serviceAddress))).
		AddAuthorizer(serviceAddress)
}
