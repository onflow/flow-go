package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const systemChunkTransactionTemplate = `
import FlowEpoch from 0x%s

transaction {
  prepare(serviceAccount: AuthAccount) { 
	heartbeat = serviceAccount.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
      ?? panic("Could not borrow heartbeat from storage path")
    heartbeat.advanceBlock()
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
