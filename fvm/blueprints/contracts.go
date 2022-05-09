package blueprints

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

const ContractDeploymentAuthorizedAddressesPathDomain = "storage"
const ContractDeploymentAuthorizedAddressesPathIdentifier = "authorizedAddressesToDeployContracts"

const ContractRemovalAuthorizedAddressesPathDomain = "storage"
const ContractRemovalAuthorizedAddressesPathIdentifier = "authorizedAddressesToRemoveContracts"

const IsContractDeploymentRestrictedPathDomain = "storage"
const IsContractDeploymentRestrictedPathIdentifier = "isContractDeploymentRestricted"

const setContractDeploymentAuthorizersTransactionTemplate = `
transaction(addresses: [Address], path: StoragePath) {
	prepare(signer: AuthAccount) {
		signer.load<[Address]>(from: path)
		signer.save(addresses, to: path)
	}
}
`

// SetContractDeploymentAuthorizersTransaction returns a transaction for updating list of authorized accounts allowed to deploy/update contracts
func SetContractDeploymentAuthorizersTransaction(serviceAccount flow.Address, authorized []flow.Address) (*flow.TransactionBody, error) {
	arg1, err := jsoncdc.Encode(utils.AddressSliceToCadenceValue(utils.FlowAddressSliceToCadenceAddressSlice(authorized)))
	if err != nil {
		return nil, err
	}

	arg2, err := jsoncdc.Encode(cadence.Path{
		Domain:     ContractDeploymentAuthorizedAddressesPathDomain,
		Identifier: ContractDeploymentAuthorizedAddressesPathIdentifier,
	})
	if err != nil {
		return nil, err
	}

	return flow.NewTransactionBody().
		SetScript([]byte(setContractDeploymentAuthorizersTransactionTemplate)).
		AddAuthorizer(serviceAccount).
		AddArgument(arg1).
		AddArgument(arg2), nil
}

// SetContractRemovalAuthorizersTransaction returns a transaction for updating list of authorized accounts allowed to remove contracts
func SetContractRemovalAuthorizersTransaction(serviceAccount flow.Address, authorized []flow.Address) (*flow.TransactionBody, error) {
	arg1, err := jsoncdc.Encode(utils.AddressSliceToCadenceValue(utils.FlowAddressSliceToCadenceAddressSlice(authorized)))
	if err != nil {
		return nil, err
	}

	arg2, err := jsoncdc.Encode(cadence.Path{
		Domain:     ContractRemovalAuthorizedAddressesPathDomain,
		Identifier: ContractRemovalAuthorizedAddressesPathIdentifier,
	})
	if err != nil {
		return nil, err
	}

	return flow.NewTransactionBody().
		SetScript([]byte(setContractDeploymentAuthorizersTransactionTemplate)).
		AddAuthorizer(serviceAccount).
		AddArgument(arg1).
		AddArgument(arg2), nil
}

const setIsContractDeploymentRestrictedTransactionTemplate = `
transaction(restricted: Bool, path: StoragePath) {
	prepare(signer: AuthAccount) {
		signer.load<Bool>(from: path)
		signer.save(restricted, to: path)
	}
}
`

// SetIsContractDeploymentRestrictedTransaction sets the restricted flag for contract deployment
func SetIsContractDeploymentRestrictedTransaction(serviceAccount flow.Address, restricted bool) (*flow.TransactionBody, error) {
	argRestricted, err := jsoncdc.Encode(cadence.Bool(restricted))
	if err != nil {
		return nil, err
	}

	argPath, err := jsoncdc.Encode(cadence.Path{
		Domain:     IsContractDeploymentRestrictedPathDomain,
		Identifier: IsContractDeploymentRestrictedPathIdentifier,
	})
	if err != nil {
		return nil, err
	}

	return flow.NewTransactionBody().
		SetScript([]byte(setIsContractDeploymentRestrictedTransactionTemplate)).
		AddAuthorizer(serviceAccount).
		AddArgument(argRestricted).
		AddArgument(argPath), nil
}

const deployContractTransactionTemplate = `
transaction {
  prepare(signer: AuthAccount) {
    signer.contracts.add(name: "%s", code: "%s".decodeHex())
  }
}
`

// TODO (ramtin) get rid of authorizers
func DeployContractTransaction(address flow.Address, contract []byte, contractName string) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployContractTransactionTemplate, contractName, hex.EncodeToString(contract)))).
		AddAuthorizer(address)
}
