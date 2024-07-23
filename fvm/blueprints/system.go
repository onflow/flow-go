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

// TODO: when the EVM contract is moved to the flow-core-contracts, we can
// just directly use the replace address functionality of the templates package.

var placeholderEVMAddress = "\"EVM\""

func prepareSystemContractCode(chainID flow.ChainID) string {
	sc := systemcontracts.SystemContractsForChain(chainID)
	code := templates.ReplaceAddresses(
		systemChunkTransactionTemplate,
		sc.AsTemplateEnv(),
	)
	code = strings.ReplaceAll(
		code,
		placeholderEVMAddress,
		sc.EVMContract.Address.HexWithPrefix(),
	)
	return code
}

// SystemChunkTransaction creates and returns the transaction corresponding to the
// system chunk for the given chain.
func SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	tx := flow.NewTransactionBody().
		SetScript(
			[]byte(prepareSystemContractCode(chain.ChainID())),
		).
		// The heartbeat resources needed by the system tx have are on the service account,
		// therefore, the service account is the only authorizer needed.
		AddAuthorizer(chain.ServiceAddress()).
		SetComputeLimit(SystemChunkTransactionGasLimit)

	return tx, nil
}
