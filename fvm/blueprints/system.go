package blueprints

import (
	_ "embed"
	"fmt"
	"strings"

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

const placeholderMigrationAddress = "\"Migration\""

func prepareSystemContractCode(chainID flow.ChainID) string {
	sc := systemcontracts.SystemContractsForChain(chainID)
	code := templates.ReplaceAddresses(
		systemChunkTransactionTemplate,
		sc.AsTemplateEnv(),
	)
	code = strings.ReplaceAll(
		code,
		placeholderMigrationAddress,
		sc.Migration.Address.HexWithPrefix(),
	)
	return code
}

// SystemChunkTransaction creates and returns the transaction corresponding to the
// system chunk for the given chain.
func SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	// The heartbeat resources needed by the system tx have are on the service account,
	// therefore, the service account is the only authorizer needed.
	systemTxBody, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(prepareSystemContractCode(chain.ChainID()))).
		SetComputeLimit(SystemChunkTransactionGasLimit).
		AddAuthorizer(chain.ServiceAddress()).
		Build()
	if err != nil {
		return nil, fmt.Errorf("could not build system chunk transaction: %w", err)
	}

	return systemTxBody, nil
}

// SystemCollection returns the re-created system collection after it has been already executed
// using the events from the process callback transaction.
func SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	process, err := ProcessCallbacksTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct process callbacks transaction: %w", err)
	}

	executes, err := ExecuteCallbacksTransactions(chain, processEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to construct execute callbacks transactions: %w", err)
	}

	systemTx, err := SystemChunkTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
	}

	transactions := make([]*flow.TransactionBody, 0, len(executes)+2) // +2 process and system tx
	transactions = append(transactions, process)
	transactions = append(transactions, executes...)
	transactions = append(transactions, systemTx)

	collection, err := flow.NewCollection(flow.UntrustedCollection{
		Transactions: transactions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to construct system collection: %w", err)
	}

	return collection, nil
}
