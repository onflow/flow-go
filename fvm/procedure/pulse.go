package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// TODO RAMTIN we might need to move this to exec node instead of here
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
