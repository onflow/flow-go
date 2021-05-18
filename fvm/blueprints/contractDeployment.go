package blueprints

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

const ContractDeploymentAuthorizedAddressesPathDomain = "storage"
const ContractDeploymentAuthorizedAddressesPathIdentifier = "authorizedAddressesToDeployContracts"

const setContractDeploymentAuthorizersTransactionTemplate = `
transaction(addresses: [Address], path: StoragePath) {
	prepare(signer: AuthAccount) {
		signer.load<[Address]>(from: path)
		signer.save(addresses, to: path)
	}
}
`

// SetContractDeploymentAuthorizersTransaction returns a transaction for updating list of authroized accounts allowed to deploy/update contracts
func SetContractDeploymentAuthorizersTransaction(serviceAccount flow.Address, authorized []flow.Address) (*flow.TransactionBody, error) {
	arg1, err := jsoncdc.Encode(utils.AddressSliceToCadenceValue(utils.FlowAddressSliceToCadenceAddressSlice(authorized)))
	if err != nil {
		return nil, err
	}

	arg2, err := jsoncdc.Encode(cadence.Path{Domain: ContractDeploymentAuthorizedAddressesPathDomain,
		Identifier: ContractDeploymentAuthorizedAddressesPathIdentifier})
	if err != nil {
		return nil, err
	}

	return flow.NewTransactionBody().
		SetScript([]byte(setContractDeploymentAuthorizersTransactionTemplate)).
		AddAuthorizer(serviceAccount).
		AddArgument(arg1).
		AddArgument(arg2), nil
}
