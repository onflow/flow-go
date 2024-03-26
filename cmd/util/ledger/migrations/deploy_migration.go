package migrations

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func NewDeploymentMigration(
	chainID flow.ChainID,
	contract Contract,
	authorizer flow.Address,
	expectedWriteAddresses map[flow.Address]struct{},
	nWorker int,
	logger zerolog.Logger,
) ledger.Migration {

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
		nWorker,
		expectedWriteAddresses,
	)
}

func NewBurnerDeploymentMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
	nWorker int,
) ledger.Migration {
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
		nWorker,
		logger,
	)
}
