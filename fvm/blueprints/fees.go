package blueprints

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/model/flow"
)

const TransactionFeesExecutionEffortWeightsPathDomain = "storage"
const TransactionFeesExecutionEffortWeightsPathIdentifier = "executionEffortWeights"

const deployTxFeesTransactionTemplate = `
transaction {
  prepare(flowFeesAccount: AuthAccount, serviceAccount: AuthAccount) {
    let adminAccount = serviceAccount
    flowFeesAccount.contracts.add(name: "FlowFees", code: "%s".decodeHex(), adminAccount: adminAccount)
  }
}
`

func DeployTxFeesContractTransaction(service, fungibleToken, flowToken, flowFees flow.Address) *flow.TransactionBody {
	contract := contracts.FlowFees(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
	)

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployTxFeesTransactionTemplate, hex.EncodeToString(contract)))).
		AddAuthorizer(flowFees).
		AddAuthorizer(service)
}

const deployStorageFeesTransactionTemplate = `
transaction {
  prepare(serviceAccount: AuthAccount) {
    serviceAccount.contracts.add(name: "FlowStorageFees", code: "%s".decodeHex())
  }
}
`

func DeployStorageFeesContractTransaction(service flow.Address, contract []byte) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployStorageFeesTransactionTemplate, hex.EncodeToString(contract)))).
		AddAuthorizer(service)
}

const setupParametersTransactionTemplate = `
import FlowStorageFees, FlowServiceAccount from 0x%s

transaction(accountCreationFee: UFix64, minimumStorageReservation: UFix64, storageMegaBytesPerReservedFLOW: UFix64, restrictedAccountCreationEnabled: Bool) {
    prepare(service: AuthAccount) {
        let serviceAdmin = service.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
            ?? panic("Could not borrow reference to the flow service admin!");

        let storageAdmin = service.borrow<&FlowStorageFees.Administrator>(from: /storage/storageFeesAdmin)
            ?? panic("Could not borrow reference to the flow storage fees admin!");

        serviceAdmin.setAccountCreationFee(accountCreationFee)
        serviceAdmin.setIsAccountCreationRestricted(restrictedAccountCreationEnabled)
        storageAdmin.setMinimumStorageReservation(minimumStorageReservation)
        storageAdmin.setStorageMegaBytesPerReservedFLOW(storageMegaBytesPerReservedFLOW)
    }
}
`

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
		SetScript([]byte(fmt.Sprintf(setupParametersTransactionTemplate, service))).
		AddArgument(addressCreationFeeArg).
		AddArgument(minimumStorageReservationArg).
		AddArgument(storagePerFlowArg).
		AddArgument(restrictedAccountCreationEnabledArg).
		AddAuthorizer(service)
}

const setupStorageForServiceAccountsTemplate = `
import FlowServiceAccount from 0x%s
import FlowStorageFees from 0x%s
import FungibleToken from 0x%s
import FlowToken from 0x%s

// This transaction sets up storage on any auth accounts that were created before the storage fees.
// This is used during bootstrapping a local environment 
transaction() {
    prepare(service: AuthAccount, fungibleToken: AuthAccount, flowToken: AuthAccount, feeContract: AuthAccount) {
        let authAccounts = [service, fungibleToken, flowToken, feeContract]

        // take all the funds from the service account
        let tokenVault = service.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
            ?? panic("Unable to borrow reference to the default token vault")
        
        for account in authAccounts {
            let storageReservation <- tokenVault.withdraw(amount: FlowStorageFees.minimumStorageReservation) as! @FlowToken.Vault
            let hasReceiver = account.getCapability(/public/flowTokenReceiver)!.check<&{FungibleToken.Receiver}>()
            if !hasReceiver {
                FlowServiceAccount.initDefaultToken(account)
            }
            let receiver = account.getCapability(/public/flowTokenReceiver)!.borrow<&{FungibleToken.Receiver}>()
                ?? panic("Could not borrow receiver reference to the recipient's Vault")

            receiver.deposit(from: <-storageReservation)
        }
    }
}
`

func SetupStorageForServiceAccountsTransaction(
	service, fungibleToken, flowToken, feeContract flow.Address,
) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(setupStorageForServiceAccountsTemplate, service, service, fungibleToken, flowToken))).
		AddAuthorizer(service).
		AddAuthorizer(fungibleToken).
		AddAuthorizer(flowToken).
		AddAuthorizer(feeContract)
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
		SetScript([]byte(fmt.Sprintf(setupFeesTransactionTemplate, flowFees))).
		AddArgument(surgeFactorArg).
		AddArgument(inclusionEffortCostArg).
		AddArgument(executionEffortCostArg).
		AddAuthorizer(service)
}

const setupFeesTransactionTemplate = `
import FlowFees from 0x%s

transaction(surgeFactor: UFix64, inclusionEffortCost: UFix64, executionEffortCost: UFix64) {
    prepare(service: AuthAccount) {

        let flowFeesAdmin = service.borrow<&FlowFees.Administrator>(from: /storage/flowFeesAdmin)
            ?? panic("Could not borrow reference to the flow fees admin!");

        flowFeesAdmin.setFeeParameters(surgeFactor: surgeFactor, inclusionEffortCost: inclusionEffortCost, executionEffortCost: executionEffortCost)
    }
}
`

// SetExecutionEffortWeightsTransaction creates a transaction that sets up weights for the weighted Meter.
func SetExecutionEffortWeightsTransaction(
	service flow.Address,
	weights map[uint]uint64,
) (*flow.TransactionBody, error) {
	return setExecutionWeightsTransaction(
		service,
		weights,
		TransactionFeesExecutionEffortWeightsPathDomain,
		TransactionFeesExecutionEffortWeightsPathIdentifier,
	)
}

func setExecutionWeightsTransaction(
	service flow.Address,
	weights map[uint]uint64,
	domain string,
	identifier string,
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

	storagePath, err := jsoncdc.Encode(cadence.Path{
		Domain:     domain,
		Identifier: identifier,
	})
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

const setExecutionWeightsScript = `
	transaction(newWeights: {UInt64: UInt64}, path: StoragePath) {
		prepare(signer: AuthAccount) {
			signer.load<{UInt64: UInt64}>(from: path)
			signer.save(newWeights, to: path)
		}
	}
`
