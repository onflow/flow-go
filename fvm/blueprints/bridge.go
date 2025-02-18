package blueprints

import (
	_ "embed"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	bridge "github.com/onflow/flow-evm-bridge"

	"github.com/onflow/flow-go/model/flow"
)

// CreateCOATransaction returns the transaction body for the create COA transaction
func CreateCOATransaction(service flow.Address, bridgeEnv bridge.Environment, env templates.Environment) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/evm/create_account.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(0.0))).
		AddAuthorizer(service)
}

// DeployEVMContractTransaction returns the transaction body for the deploy EVM contract transaction
func DeployEVMContractTransaction(service flow.Address, bytecode string, gasLimit int, deploymentValue float64, bridgeEnv bridge.Environment, env templates.Environment) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/evm/deploy.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String(bytecode))).
		AddArgument(jsoncdc.MustEncode(cadence.UInt64(gasLimit))).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(deploymentValue))).
		AddAuthorizer(service)
}
