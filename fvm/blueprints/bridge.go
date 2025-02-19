package blueprints

import (
	_ "embed"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	bridge "github.com/onflow/flow-evm-bridge"

	"github.com/onflow/flow-go/model/flow"
)

var BridgeContracts = []string{
	"cadence/contracts/utils/ArrayUtils.cdc",
	"cadence/contracts/utils/StringUtils.cdc",
	"cadence/contracts/utils/ScopedFTProviders.cdc",
	"cadence/contracts/utils/Serialize.cdc",
	"cadence/contracts/utils/SerializeMetadata.cdc",
	"cadence/contracts/interfaces/FlowEVMBridgeHandlerInterfaces.cdc",
	"cadence/contracts/interfaces/IBridgePermissions.cdc",
	"cadence/contracts/interfaces/ICrossVM.cdc",
	"cadence/contracts/interfaces/ICrossVMAsset.cdc",
	"cadence/contracts/interfaces/CrossVMNFT.cdc",
	"cadence/contracts/interfaces/CrossVMToken.cdc",
	"cadence/contracts/interfaces/IEVMBridgeNFTMinter.cdc",
	"cadence/contracts/interfaces/IEVMBridgeTokenMinter.cdc",
	"cadence/contracts/FlowEVMBridgeConfig.cdc",
	"cadence/contracts/interfaces/IFlowEVMNFTBridge.cdc",
	"cadence/contracts/interfaces/IFlowEVMTokenBridge.cdc",
	"cadence/contracts/FlowEVMBridgeUtils.cdc",
	"cadence/contracts/FlowEVMBridgeResolver.cdc",
	"cadence/contracts/FlowEVMBridgeHandlers.cdc",
	"cadence/contracts/FlowEVMBridgeNFTEscrow.cdc",
	"cadence/contracts/FlowEVMBridgeTokenEscrow.cdc",
	"cadence/contracts/FlowEVMBridgeTemplates.cdc",
	"cadence/contracts/FlowEVMBridge.cdc",
	"cadence/contracts/FlowEVMBridgeAccessor.cdc",
}

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

// DeployFlowEVMBridgeUtilsContractTransaction returns the transaction body for the deploy FlowEVMBridgeUtils contract transaction
func DeployFlowEVMBridgeUtilsContractTransaction(
	service flow.Address,
	bridgeEnv *bridge.Environment,
	env templates.Environment,
	contract []byte,
	contractName string,
	factoryAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/admin/deploy_bridge_utils.cdc", *bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String(contractName))).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract))).
		AddArgument(jsoncdc.MustEncode(cadence.String(factoryAddress))).
		AddAuthorizer(service)
}

// PauseBridgeTransaction returns the transaction body for the transaction to pause or unpause the VM bridge
func PauseBridgeTransaction(service flow.Address, bridgeEnv bridge.Environment, env templates.Environment, pause bool) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/pause/update_bridge_pause_status", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.Bool(pause))).
		AddAuthorizer(service)
}
