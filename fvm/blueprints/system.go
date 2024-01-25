package blueprints

import (
	_ "embed"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const SystemChunkTransactionGasLimit = 100_000_000

// systemChunkTransactionTemplate looks for the epoch and version beacon heartbeat resources
// and calls them.
//
//go:embed scripts/systemChunkTransactionTemplate.cdc
var systemChunkTransactionTemplate string

// SystemChunkTransaction creates and returns the transaction corresponding to the
// system chunk for the given chain.
func SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	contracts := systemcontracts.SystemContractsForChain(chain.ChainID())

	tx := flow.NewTransactionBody().
		SetScript(
			[]byte(templates.ReplaceAddresses(
				systemChunkTransactionTemplate,
				contracts.AsTemplateEnv(),
			)),
		).
		// The heartbeat resources needed by the system tx have are on the service account,
		// therefore, the service account is the only authorizer needed.
		AddAuthorizer(chain.ServiceAddress()).
		SetComputeLimit(SystemChunkTransactionGasLimit)

	return tx, nil
}
