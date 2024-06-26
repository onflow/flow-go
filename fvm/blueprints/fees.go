package blueprints

import (
	_ "embed"
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/model/flow"
)

var TransactionFeesExecutionEffortWeightsPath = cadence.Path{
	Domain:     common.PathDomainStorage,
	Identifier: "executionEffortWeights",
}
var TransactionFeesExecutionMemoryWeightsPath = cadence.Path{
	Domain:     common.PathDomainStorage,
	Identifier: "executionMemoryWeights",
}
var TransactionFeesExecutionMemoryLimitPath = cadence.Path{
	Domain:     common.PathDomainStorage,
	Identifier: "executionMemoryLimit",
}

//go:embed scripts/deployTxFeesTransactionTemplate.cdc
var deployTxFeesTransactionTemplate string

//go:embed scripts/setupParametersTransactionTemplate.cdc
var setupParametersTransactionTemplate string

//go:embed scripts/setupStorageForServiceAccountsTemplate.cdc
var setupStorageForServiceAccountsTemplate string

//go:embed scripts/setupStorageForAccount.cdc
var setupStorageForAccountTemplate string

//go:embed scripts/setupFeesTransactionTemplate.cdc
var setupFeesTransactionTemplate string

//go:embed scripts/setExecutionMemoryLimit.cdc
var setExecutionMemoryLimit string

func DeployTxFeesContractTransaction(flowFees, service flow.Address, contract []byte) *flow.TransactionBody {

	return flow.NewTransactionBody().
		SetScript([]byte(deployTxFeesTransactionTemplate)).
		AddArgument(jsoncdc.MustEncode(cadence.String(hex.EncodeToString(contract)))).
		AddAuthorizer(flowFees).
		AddAuthorizer(service)
}

func DeployStorageFeesContractTransaction(service flow.Address, contract []byte) *flow.TransactionBody {
	return DeployContractTransaction(service, contract, "FlowStorageFees")
}

func SetupParametersTransaction(
	service flow.Address,
	addressCreationFee,
	minimumStorageReservation,
	storagePerFlow cadence.UFix64,
	restrictedAccountCreationEnabled cadence.Bool,
) *flow.TransactionBody {
	addressCreationFeeArg, err := jsoncdc.Encode(addressCreationFee)
	if err != nil {
		panic(fmt.Sprintf("failed to encode address creation fee: %s", err.Error()))
	}
	minimumStorageReservationArg, err := jsoncdc.Encode(minimumStorageReservation)
	if err != nil {
		panic(fmt.Sprintf("failed to encode minimum storage reservation: %s", err.Error()))
	}
	storagePerFlowArg, err := jsoncdc.Encode(storagePerFlow)
	if err != nil {
		panic(fmt.Sprintf("failed to encode storage ratio: %s", err.Error()))
	}
	restrictedAccountCreationEnabledArg, err := jsoncdc.Encode(restrictedAccountCreationEnabled)
	if err != nil {
		panic(fmt.Sprintf("failed to encode restrictedAccountCreationEnabled: %s", err.Error()))
	}

	return flow.NewTransactionBody().
		SetScript([]byte(templates.ReplaceAddresses(setupParametersTransactionTemplate,
			templates.Environment{
				StorageFeesAddress:    service.Hex(),
				ServiceAccountAddress: service.Hex(),
			})),
		).
		AddArgument(addressCreationFeeArg).
		AddArgument(minimumStorageReservationArg).
		AddArgument(storagePerFlowArg).
		AddArgument(restrictedAccountCreationEnabledArg).
		AddAuthorizer(service)
}

func SetupStorageForServiceAccountsTransaction(
	service, fungibleToken, flowToken, feeContract flow.Address,
) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(templates.ReplaceAddresses(setupStorageForServiceAccountsTemplate,
			templates.Environment{
				ServiceAccountAddress: service.Hex(),
				StorageFeesAddress:    service.Hex(),
				FungibleTokenAddress:  fungibleToken.Hex(),
				FlowTokenAddress:      flowToken.Hex(),
			})),
		).
		AddAuthorizer(service).
		AddAuthorizer(fungibleToken).
		AddAuthorizer(flowToken).
		AddAuthorizer(feeContract)
}

func SetupStorageForAccountTransaction(
	account, service, fungibleToken, flowToken flow.Address,
) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(templates.ReplaceAddresses(setupStorageForAccountTemplate,
			templates.Environment{
				ServiceAccountAddress: service.Hex(),
				StorageFeesAddress:    service.Hex(),
				FungibleTokenAddress:  fungibleToken.Hex(),
				FlowTokenAddress:      flowToken.Hex(),
			})),
		).
		AddAuthorizer(account).
		AddAuthorizer(service)
}

func SetupFeesTransaction(
	service flow.Address,
	flowFees flow.Address,
	surgeFactor,
	inclusionEffortCost,
	executionEffortCost cadence.UFix64,
) *flow.TransactionBody {
	surgeFactorArg, err := jsoncdc.Encode(surgeFactor)
	if err != nil {
		panic(fmt.Sprintf("failed to encode surge factor: %s", err.Error()))
	}
	inclusionEffortCostArg, err := jsoncdc.Encode(inclusionEffortCost)
	if err != nil {
		panic(fmt.Sprintf("failed to encode inclusion effort cost: %s", err.Error()))
	}
	executionEffortCostArg, err := jsoncdc.Encode(executionEffortCost)
	if err != nil {
		panic(fmt.Sprintf("failed to encode execution effort cost: %s", err.Error()))
	}

	return flow.NewTransactionBody().
		SetScript([]byte(templates.ReplaceAddresses(setupFeesTransactionTemplate,
			templates.Environment{
				FlowFeesAddress: flowFees.Hex(),
			})),
		).
		AddArgument(surgeFactorArg).
		AddArgument(inclusionEffortCostArg).
		AddArgument(executionEffortCostArg).
		AddAuthorizer(service)
}

// SetExecutionEffortWeightsTransaction creates a transaction that sets up weights for the weighted Meter.
func SetExecutionEffortWeightsTransaction(
	service flow.Address,
	weights map[uint]uint64,
) (*flow.TransactionBody, error) {
	return setExecutionWeightsTransaction(
		service,
		weights,
		TransactionFeesExecutionEffortWeightsPath,
	)
}

// SetExecutionMemoryWeightsTransaction creates a transaction that sets up weights for the weighted Meter.
func SetExecutionMemoryWeightsTransaction(
	service flow.Address,
	weights map[uint]uint64,
) (*flow.TransactionBody, error) {
	return setExecutionWeightsTransaction(
		service,
		weights,
		TransactionFeesExecutionMemoryWeightsPath,
	)
}

func setExecutionWeightsTransaction(
	service flow.Address,
	weights map[uint]uint64,
	path cadence.Path,
) (*flow.TransactionBody, error) {
	newWeightsKeyValuePairs := make([]cadence.KeyValuePair, len(weights))
	i := 0
	for k, w := range weights {
		newWeightsKeyValuePairs[i] = cadence.KeyValuePair{
			Key:   cadence.UInt64(k),
			Value: cadence.UInt64(w),
		}
		i += 1
	}
	newWeights, err := jsoncdc.Encode(cadence.NewDictionary(newWeightsKeyValuePairs))
	if err != nil {
		return nil, err
	}

	storagePath, err := jsoncdc.Encode(path)
	if err != nil {
		return nil, err
	}

	tx := flow.NewTransactionBody().
		SetScript([]byte(setExecutionWeightsScript)).
		AddArgument(newWeights).
		AddArgument(storagePath).
		AddAuthorizer(service)

	return tx, nil
}

//go:embed scripts/setExecutionWeightsScript.cdc
var setExecutionWeightsScript string

func SetExecutionMemoryLimitTransaction(
	service flow.Address,
	limit uint64,
) (*flow.TransactionBody, error) {
	newLimit, err := jsoncdc.Encode(cadence.UInt64(limit))
	if err != nil {
		return nil, err
	}

	storagePath, err := jsoncdc.Encode(TransactionFeesExecutionMemoryLimitPath)
	if err != nil {
		return nil, err
	}

	tx := flow.NewTransactionBody().
		SetScript([]byte(setExecutionMemoryLimit)).
		AddArgument(newLimit).
		AddArgument(storagePath).
		AddAuthorizer(service)

	return tx, nil
}
