package blueprints

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const SystemChunkTransactionGasLimit = 100_000_000

// TODO (Ramtin) after changes to this method are merged into master move them here.

const systemChunkTransactionTemplate = `
import FlowEpoch from 0x%s

transaction {
  prepare(serviceAccount: AuthAccount) { 
	let heartbeat = serviceAccount.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
      ?? panic("Could not borrow heartbeat from storage path")
    heartbeat.advanceBlock()
  }
} 
`

// SystemChunkTransaction creates and returns the transaction corresponding to the system chunk
// for the given chain.
func SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {

	contracts, err := systemcontracts.SystemContractsForChain(chain.ChainID())
	if err != nil {
		return nil, fmt.Errorf("could not get system contracts for chain: %w", err)
	}

	tx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(systemChunkTransactionTemplate, contracts.Epoch.Address))).
		AddAuthorizer(contracts.Epoch.Address).
		SetGasLimit(SystemChunkTransactionGasLimit)

	return tx, nil
}
