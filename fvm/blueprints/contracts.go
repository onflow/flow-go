package blueprints

import (
	_ "embed"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/model/flow"
)

var ContractDeploymentAuthorizedAddressesPath = cadence.Path{
	Domain:     common.PathDomainStorage,
	Identifier: "authorizedAddressesToDeployContracts",
}
var ContractRemovalAuthorizedAddressesPath = cadence.Path{
	Domain:     common.PathDomainStorage,
	Identifier: "authorizedAddressesToRemoveContracts",
}
var IsContractDeploymentRestrictedPath = cadence.Path{
	Domain:     common.PathDomainStorage,
	Identifier: "isContractDeploymentRestricted",
}

//go:embed scripts/setContractOperationAuthorizersTransactionTemplate.cdc
var setContractOperationAuthorizersTransactionTemplate string

//go:embed scripts/setIsContractDeploymentRestrictedTransactionTemplate.cdc
var setIsContractDeploymentRestrictedTransactionTemplate string

//go:embed scripts/deployContractTransactionTemplate.cdc
var DeployContractTransactionTemplate []byte

// SetContractDeploymentAuthorizersTransaction returns a transaction for updating list of authorized accounts allowed to deploy/update contracts
func SetContractDeploymentAuthorizersTransaction(serviceAccount flow.Address, authorized []flow.Address) (*flow.TransactionBodyBuilder, error) {
	return setContractAuthorizersTransaction(ContractDeploymentAuthorizedAddressesPath, serviceAccount, authorized)
}

// SetContractRemovalAuthorizersTransaction returns a transaction for updating list of authorized accounts allowed to remove contracts
func SetContractRemovalAuthorizersTransaction(serviceAccount flow.Address, authorized []flow.Address) (*flow.TransactionBodyBuilder, error) {
	return setContractAuthorizersTransaction(ContractRemovalAuthorizedAddressesPath, serviceAccount, authorized)
}

func setContractAuthorizersTransaction(
	path cadence.Path,
	serviceAccount flow.Address,
	authorized []flow.Address,
) (*flow.TransactionBodyBuilder, error) {
	addressValues := make([]cadence.Value, 0, len(authorized))
	for _, address := range authorized {
		addressValues = append(
			addressValues,
			cadence.Address(address))
	}

	addressesArg, err := jsoncdc.Encode(cadence.NewArray(addressValues))
	if err != nil {
		return nil, err
	}

	pathArg, err := jsoncdc.Encode(path)
	if err != nil {
		return nil, err
	}

	return flow.NewTransactionBodyBuilder().
		SetScript([]byte(setContractOperationAuthorizersTransactionTemplate)).
		SetPayer(serviceAccount).
		AddAuthorizer(serviceAccount).
		AddArgument(addressesArg).
		AddArgument(pathArg), nil
}

// SetIsContractDeploymentRestrictedTransaction sets the restricted flag for contract deployment
func SetIsContractDeploymentRestrictedTransaction(serviceAccount flow.Address, restricted bool) (*flow.TransactionBody, error) {
	argRestricted, err := jsoncdc.Encode(cadence.Bool(restricted))
	if err != nil {
		return nil, err
	}

	argPath, err := jsoncdc.Encode(IsContractDeploymentRestrictedPath)
	if err != nil {
		return nil, err
	}

	return flow.NewTransactionBodyBuilder().
		SetScript([]byte(setIsContractDeploymentRestrictedTransactionTemplate)).
		SetPayer(serviceAccount).
		AddAuthorizer(serviceAccount).
		AddArgument(argRestricted).
		AddArgument(argPath).
		Build()
}

// TODO (ramtin) get rid of authorizers
func DeployContractTransaction(address flow.Address, contract []byte, contractName string) *flow.TransactionBodyBuilder {
	return flow.NewTransactionBodyBuilder().
		SetScript(DeployContractTransactionTemplate).
		AddArgument(jsoncdc.MustEncode(cadence.String(contractName))).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract))).
		SetPayer(address).
		AddAuthorizer(address)
}
