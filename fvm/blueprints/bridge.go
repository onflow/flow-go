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
	"cadence/contracts/bridge/interfaces/FlowEVMBridgeHandlerInterfaces.cdc",
	"cadence/contracts/bridge/interfaces/IBridgePermissions.cdc",
	"cadence/contracts/bridge/interfaces/ICrossVM.cdc",
	"cadence/contracts/bridge/interfaces/ICrossVMAsset.cdc",
	"cadence/contracts/bridge/interfaces/CrossVMNFT.cdc",
	"cadence/contracts/bridge/interfaces/CrossVMToken.cdc",
	"cadence/contracts/bridge/interfaces/IEVMBridgeNFTMinter.cdc",
	"cadence/contracts/bridge/interfaces/IEVMBridgeTokenMinter.cdc",
	"cadence/contracts/bridge/FlowEVMBridgeConfig.cdc",
	"cadence/contracts/bridge/interfaces/IFlowEVMNFTBridge.cdc",
	"cadence/contracts/bridge/interfaces/IFlowEVMTokenBridge.cdc",
	"cadence/contracts/bridge/FlowEVMBridgeUtils.cdc",
	"cadence/contracts/bridge/FlowEVMBridgeResolver.cdc",
	"cadence/contracts/bridge/FlowEVMBridgeHandlers.cdc",
	"cadence/contracts/bridge/FlowEVMBridgeNFTEscrow.cdc",
	"cadence/contracts/bridge/FlowEVMBridgeTokenEscrow.cdc",
	"cadence/contracts/bridge/FlowEVMBridgeTemplates.cdc",
	"cadence/contracts/bridge/FlowEVMBridge.cdc",
}

// CreateCOATransaction returns the transaction body for the create COA transaction
func CreateCOATransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/evm/create_account.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(0.0))).
		AddAuthorizer(service)
}

// DeployEVMContractTransaction returns the transaction body for
// the deploy EVM contract transaction
func DeployEVMContractTransaction(
	service flow.Address,
	bytecode string,
	gasLimit int,
	deploymentValue float64,
	bridgeEnv bridge.Environment,
	env templates.Environment,
) *flow.TransactionBody {
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

// DeployFlowEVMBridgeUtilsContractTransaction returns the transaction body for
// the deploy FlowEVMBridgeUtils contract transaction
func DeployFlowEVMBridgeUtilsContractTransaction(
	service flow.Address,
	bridgeEnv *bridge.Environment,
	env templates.Environment,
	contract []byte,
	contractName string,
	factoryAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/deploy_bridge_utils.cdc", *bridgeEnv, env)
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

// PauseBridgeTransaction returns the transaction body for the transaction
// to pause or unpause the VM bridge
func PauseBridgeTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
	pause bool,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/pause/update_bridge_pause_status.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.Bool(pause))).
		AddAuthorizer(service)
}

// SetRegistrarTransaction returns the transaction body for the transaction to set the factory as registrar
func SetRegistrarTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
	registryAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm/set_registrar.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String(registryAddress))).
		AddAuthorizer(service)
}

// SetDeploymentRegistryTransaction returns the transaction body for the transaction
// to add the registry to the factory
func SetDeploymentRegistryTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
	registryAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm/set_deployment_registry.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String(registryAddress))).
		AddAuthorizer(service)
}

// SetDelegatedDeployerTransaction returns the transaction body for the transaction
// to set a delegated deployer for a particular token type
func SetDelegatedDeployerTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
	deployerAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm/set_delegated_deployer.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String(deployerAddress))).
		AddAuthorizer(service)
}

// AddDeployerTransaction returns the transaction body for the transaction
// to add a deployer for a particular token type
func AddDeployerTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
	deployerTag,
	deployerAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm/add_deployer.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String(deployerTag))).
		AddArgument(jsoncdc.MustEncode(cadence.String(deployerAddress))).
		AddAuthorizer(service)
}

// DeployFlowEVMBridgeAccessorContractTransaction returns the transaction body for the deploy FlowEVMBridgeAccessor contract transaction
func DeployFlowEVMBridgeAccessorContractTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
) *flow.TransactionBody {
	contract, _ := bridge.GetCadenceContractCode("cadence/contracts/bridge/FlowEVMBridgeAccessor.cdc", bridgeEnv, env)
	contractName := "FlowEVMBridgeAccessor"
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/deploy_bridge_accessor.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String(contractName))).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract))).
		AddArgument(jsoncdc.MustEncode(cadence.Address(service))).
		AddAuthorizer(service)
}

// IntegrateEVMWithBridgeAccessorTransaction returns the transaction body for the transaction
// that claims the bridge accessor capability and saves the bridge router
func IntegrateEVMWithBridgeAccessorTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm-integration/claim_accessor_capability_and_save_router.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String("FlowEVMBridgeAccessor"))).
		AddArgument(jsoncdc.MustEncode(cadence.Address(service))).
		AddAuthorizer(service)
}

// UpdateOnboardFeeTransaction returns the transaction body for the transaction
// that updates the onboarding fees for the bridge
func UpdateOnboardFeeTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
	fee float64,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/fee/update_onboard_fee.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(fee))).
		AddAuthorizer(service)
}

// UpdateBaseFeeTransaction returns the transaction body for the transaction
// that updates the base fees for the bridge
func UpdateBaseFeeTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
	fee float64,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/fee/update_base_fee.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(fee))).
		AddAuthorizer(service)
}

// UpsertContractCodeChunksTransaction returns the transaction body for the transaction
// that adds the code chunks for the FT or NFT templates to the bridge
func UpsertContractCodeChunksTransaction(
	service flow.Address,
	bridgeEnv bridge.Environment,
	env templates.Environment,
	forTemplate string,
	newChunks []string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/templates/upsert_contract_code_chunks.cdc", bridgeEnv, env)

	chunks := make([]cadence.Value, len(newChunks))
	for i, chunk := range newChunks {
		chunks[i] = cadence.String(chunk)
	}

	return flow.NewTransactionBody().
		SetScript([]byte(
			txScript,
		),
		).
		AddArgument(jsoncdc.MustEncode(cadence.String(forTemplate))).
		AddArgument(jsoncdc.MustEncode(cadence.NewArray(chunks))).
		AddAuthorizer(service)
}
