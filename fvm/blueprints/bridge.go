package blueprints

import (
	_ "embed"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	bridge "github.com/onflow/flow-evm-bridge"

	"github.com/onflow/flow-go/model/flow"
)

// All the Cadence contracts that make up the core functionality
// of the Flow VM bridge. They are all needed for the
// bridge to function properly.
// Solidity contracts are handled elsewhere in the bootstrapping process
// See more info in the VM Bridge Repo
// https://github.com/onflow/flow-evm-bridge
// or FLIP
// https://github.com/onflow/flips/blob/main/application/20231222-evm-vm-bridge.md
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
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/evm/create_account.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(0.0))).
		AddAuthorizer(service)
}

// DeployEVMContractTransaction returns the transaction body for
// the deploy EVM contract transaction
func DeployEVMContractTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	bytecode string,
	gasLimit int,
	deploymentValue float64,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/evm/deploy.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(bytecode))).
		AddArgument(jsoncdc.MustEncode(cadence.UInt64(gasLimit))).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(deploymentValue))).
		AddAuthorizer(service)
}

// DeployFlowEVMBridgeUtilsContractTransaction returns the transaction body for
// the deploy FlowEVMBridgeUtils contract transaction
func DeployFlowEVMBridgeUtilsContractTransaction(
	env templates.Environment,
	bridgeEnv *bridge.Environment,
	service flow.Address,
	contract []byte,
	contractName string,
	factoryAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/deploy_bridge_utils.cdc", *bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(contractName))).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract))).
		AddArgument(jsoncdc.MustEncode(cadence.String(factoryAddress))).
		AddAuthorizer(service)
}

// PauseBridgeTransaction returns the transaction body for the transaction
// to pause or unpause the VM bridge
func PauseBridgeTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	pause bool,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/pause/update_bridge_pause_status.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.Bool(pause))).
		AddAuthorizer(service)
}

// SetRegistrarTransaction returns the transaction body for the transaction to set the factory as registrar
func SetRegistrarTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	registryAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm/set_registrar.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(registryAddress))).
		AddAuthorizer(service)
}

// SetDeploymentRegistryTransaction returns the transaction body for the transaction
// to add the registry to the factory
func SetDeploymentRegistryTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	registryAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm/set_deployment_registry.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(registryAddress))).
		AddAuthorizer(service)
}

// SetDelegatedDeployerTransaction returns the transaction body for the transaction
// to set a delegated deployer for a particular token type
func SetDelegatedDeployerTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	deployerAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm/set_delegated_deployer.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(deployerAddress))).
		AddAuthorizer(service)
}

// AddDeployerTransaction returns the transaction body for the transaction
// to add a deployer for a particular token type
func AddDeployerTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	deployerTag, deployerAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm/add_deployer.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(deployerTag))).
		AddArgument(jsoncdc.MustEncode(cadence.String(deployerAddress))).
		AddAuthorizer(service)
}

// DeployFlowEVMBridgeAccessorContractTransaction returns the transaction body for the deploy FlowEVMBridgeAccessor contract transaction
func DeployFlowEVMBridgeAccessorContractTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
) *flow.TransactionBody {
	contract, _ := bridge.GetCadenceContractCode("cadence/contracts/bridge/FlowEVMBridgeAccessor.cdc", bridgeEnv, env)
	contractName := "FlowEVMBridgeAccessor"
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/deploy_bridge_accessor.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(contractName))).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract))).
		AddArgument(jsoncdc.MustEncode(cadence.Address(service))).
		AddAuthorizer(service)
}

// IntegrateEVMWithBridgeAccessorTransaction returns the transaction body for the transaction
// that claims the bridge accessor capability and saves the bridge router
func IntegrateEVMWithBridgeAccessorTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/evm-integration/claim_accessor_capability_and_save_router.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String("FlowEVMBridgeAccessor"))).
		AddArgument(jsoncdc.MustEncode(cadence.Address(service))).
		AddAuthorizer(service)
}

// UpdateOnboardFeeTransaction returns the transaction body for the transaction
// that updates the onboarding fees for the bridge
func UpdateOnboardFeeTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	fee float64,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/fee/update_onboard_fee.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(fee))).
		AddAuthorizer(service)
}

// UpdateBaseFeeTransaction returns the transaction body for the transaction
// that updates the base fees for the bridge
func UpdateBaseFeeTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	fee float64,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/fee/update_base_fee.cdc", bridgeEnv, env)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(fee))).
		AddAuthorizer(service)
}

// UpsertContractCodeChunksTransaction returns the transaction body for the transaction
// that adds the code chunks for the FT or NFT templates to the bridge
func UpsertContractCodeChunksTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	forTemplate string,
	newChunks []string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/templates/upsert_contract_code_chunks.cdc", bridgeEnv, env)

	chunks := make([]cadence.Value, len(newChunks))
	for i, chunk := range newChunks {
		chunks[i] = cadence.String(chunk)
	}

	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(forTemplate))).
		AddArgument(jsoncdc.MustEncode(cadence.NewArray(chunks))).
		AddAuthorizer(service)
}

// CreateWFLOWTokenHandlerTransaction returns the transaction body for the transaction
// that creates a token handler for the WFLOW Solidity contract
func CreateWFLOWTokenHandlerTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	wflowEVMAddress string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/token-handler/create_wflow_token_handler.cdc", bridgeEnv, env)

	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(wflowEVMAddress))).
		SetProposalKey(service, 0, 0).
		SetPayer(service).
		AddAuthorizer(service)
}

// EnableWFLOWTokenHandlerTransaction returns the transaction body for the transaction
// that enables the token handler for the WFLOW Solidity contract
func EnableWFLOWTokenHandlerTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	flowTokenType string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/admin/token-handler/enable_token_handler.cdc", bridgeEnv, env)

	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(flowTokenType))).
		SetProposalKey(service, 0, 0).
		SetPayer(service).
		AddAuthorizer(service)
}

// OnboardToBridgeByTypeIDTransaction returns the transaction body for the transaction
// that onboards a FT or NFT type to the bridge
func OnboardToBridgeByTypeIDTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	forType string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/onboarding/onboard_by_type_identifier.cdc", bridgeEnv, env)

	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(forType))).
		SetProposalKey(service, 0, 0).
		SetPayer(service).
		AddAuthorizer(service)
}

// BridgeFTToEVMTransaction returns the transaction body for the transaction
// that bridges a fungible token from Cadence to EVM
func BridgeFTToEVMTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	forType string,
	amount string,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/tokens/bridge_tokens_to_evm.cdc", bridgeEnv, env)
	bridgeAmount, _ := cadence.NewUFix64(amount)
	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(forType))).
		AddArgument(jsoncdc.MustEncode(bridgeAmount)).
		SetProposalKey(service, 0, 0).
		SetPayer(service).
		AddAuthorizer(service)
}

// BridgeFTFromEVMTransaction returns the transaction body for the transaction
// that bridges a fungible token from EVM to Cadence
func BridgeFTFromEVMTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	forType string,
	amount uint,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/tokens/bridge_tokens_from_evm.cdc", bridgeEnv, env)

	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(forType))).
		AddArgument(jsoncdc.MustEncode(cadence.NewUInt256(amount))).
		SetProposalKey(service, 0, 0).
		SetPayer(service).
		AddAuthorizer(service)
}

// GetEscrowedTokenBalanceScript returns the script body for the script
// that gets the balance of an escrowed fungible token in the Cadence side of the VM bridge
func GetEscrowedTokenBalanceScript(
	env templates.Environment,
	bridgeEnv bridge.Environment,
) []byte {
	script, _ := bridge.GetCadenceTransactionCode("cadence/scripts/escrow/get_locked_token_balance.cdc", bridgeEnv, env)

	return script
}

// BridgeNFTToEVMTransaction returns the transaction body for the transaction
// that bridges a non-fungible token from Cadence to EVM
func BridgeNFTToEVMTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	forType string,
	id cadence.UInt64,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/nft/bridge_nft_to_evm.cdc", bridgeEnv, env)

	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(forType))).
		AddArgument(jsoncdc.MustEncode(id)).
		SetProposalKey(service, 0, 0).
		SetPayer(service).
		AddAuthorizer(service)
}

// BridgeNFTFromEVMTransaction returns the transaction body for the transaction
// that bridges a non-fungible token from EVM to Cadence
func BridgeNFTFromEVMTransaction(
	env templates.Environment,
	bridgeEnv bridge.Environment,
	service flow.Address,
	forType string,
	id cadence.UInt256,
) *flow.TransactionBody {
	txScript, _ := bridge.GetCadenceTransactionCode("cadence/transactions/bridge/nft/bridge_nft_from_evm.cdc", bridgeEnv, env)

	return flow.NewTransactionBody().
		SetScript(txScript).
		AddArgument(jsoncdc.MustEncode(cadence.String(forType))).
		AddArgument(jsoncdc.MustEncode(id)).
		SetProposalKey(service, 0, 0).
		SetPayer(service).
		AddAuthorizer(service)
}

// GetIsNFTInEscrowScript returns the script body for the script
// that gets if an NFT is escrowed in the Cadence side of the VM bridge
func GetIsNFTInEscrowScript(
	env templates.Environment,
	bridgeEnv bridge.Environment,
) []byte {
	script, _ := bridge.GetCadenceTransactionCode("cadence/scripts/escrow/is_nft_locked.cdc", bridgeEnv, env)

	return script
}
