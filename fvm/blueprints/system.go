package blueprints

import (
	_ "embed"
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

func prepareSystemContractCode(sc *systemcontracts.SystemContracts) []byte {
	code := templates.ReplaceAddresses(systemChunkTransactionTemplate, sc.AsTemplateEnv())
	code = strings.ReplaceAll(
		code,
		placeholderMigrationAddress,
		sc.Migration.Address.HexWithPrefix(),
	)
	return []byte(code)
}

// SystemChunkTransaction creates and returns the transaction corresponding to the
// system chunk for the given chain.
//
// No error returns are expected during normal operation.
func SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	// The heartbeat resources needed by the system tx have are on the service account,
	// therefore, the service account is the only authorizer needed.
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	script := prepareSystemContractCode(sc)
	return flow.NewTransactionBodyBuilder().
		SetScript(script).
		SetComputeLimit(SystemChunkTransactionGasLimit).
		AddAuthorizer(chain.ServiceAddress()).
		Build()
}
