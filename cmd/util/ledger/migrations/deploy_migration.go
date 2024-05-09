package migrations

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/rs/zerolog"

	evm "github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func NewDeploymentMigration(
	chainID flow.ChainID,
	contract Contract,
	authorizer flow.Address,
	expectedWriteAddresses map[flow.Address]struct{},
	logger zerolog.Logger,
) RegistersMigration {

	script := []byte(`
      transaction(name: String, code: String) {
          prepare(signer: auth(AddContract) &Account) {
              signer.contracts.add(name: name, code: code.utf8)
          }
      }
    `)

	tx := flow.NewTransactionBody().
		SetScript(script).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract.Name))).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract.Code))).
		AddAuthorizer(authorizer)

	return NewTransactionBasedMigration(
		tx,
		chainID,
		logger,
		expectedWriteAddresses,
	)
}

func NewBurnerDeploymentMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
) RegistersMigration {
	address := BurnerAddressForChain(chainID)
	return NewDeploymentMigration(
		chainID,
		Contract{
			Name: "Burner",
			Code: coreContracts.Burner(),
		},
		address,
		map[flow.Address]struct{}{
			address: {},
		},
		logger,
	)
}

func NewEVMDeploymentMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
) RegistersMigration {
	systemContracts := systemcontracts.SystemContractsForChain(chainID)
	address := systemContracts.EVMContract.Address
	return NewDeploymentMigration(
		chainID,
		Contract{
			Name: systemContracts.EVMContract.Name,
			Code: evm.ContractCode(
				systemContracts.NonFungibleToken.Address,
				systemContracts.FungibleToken.Address,
				systemContracts.FlowToken.Address,
			),
		},
		address,
		map[flow.Address]struct{}{
			address: {},
		},
		logger,
	)
}
