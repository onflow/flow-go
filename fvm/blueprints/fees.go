package blueprints

import (
	"encoding/hex"
	"fmt"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/model/flow"
)

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

const setupFeesTransactionTemplate = `
import FlowStorageFees, FlowServiceAccount from 0x%s

transaction(transactionFee: UFix64, accountCreationFee: UFix64, minimumStorageReservation: UFix64, storageMegaBytesPerReservedFLOW: UFix64, restrictedAccountCreationEnabled: Bool) {
    prepare(service: AuthAccount) {
        let serviceAdmin = service.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
            ?? panic("Could not borrow reference to the flow service admin!");

        let storageAdmin = service.borrow<&FlowStorageFees.Administrator>(from: /storage/storageFeesAdmin)
            ?? panic("Could not borrow reference to the flow storage fees admin!");

        serviceAdmin.setTransactionFee(transactionFee)
        serviceAdmin.setAccountCreationFee(accountCreationFee)
        serviceAdmin.setIsAccountCreationRestricted(restrictedAccountCreationEnabled)
        storageAdmin.setMinimumStorageReservation(minimumStorageReservation)
        storageAdmin.setStorageMegaBytesPerReservedFLOW(storageMegaBytesPerReservedFLOW)
    }
}
`

func SetupFeesTransaction(
	service flow.Address,
	transactionFee,
	addressCreationFee,
	minimumStorageReservation,
	storagePerFlow cadence.UFix64,
	restrictedAccountCreationEnabled cadence.Bool,
) *flow.TransactionBody {
	transactionFeeArg, err := jsoncdc.Encode(transactionFee)
	if err != nil {
		panic(fmt.Sprintf("failed to encode transaction fee: %s", err.Error()))
	}
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
		SetScript([]byte(fmt.Sprintf(setupFeesTransactionTemplate, service))).
		AddArgument(transactionFeeArg).
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
