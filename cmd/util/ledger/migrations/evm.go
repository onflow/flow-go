package migrations

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func NewEVMDeploymentMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
	full bool,
) RegistersMigration {

	systemContracts := systemcontracts.SystemContractsForChain(chainID)
	address := systemContracts.EVMContract.Address

	var code []byte
	if full {
		code = stdlib.ContractCode(
			systemContracts.NonFungibleToken.Address,
			systemContracts.FungibleToken.Address,
			systemContracts.FlowToken.Address,
		)
	} else {
		code = []byte(stdlib.ContractMinimalCode)
	}

	return NewDeploymentMigration(
		chainID,
		Contract{
			Name: systemContracts.EVMContract.Name,
			Code: code,
		},
		address,
		map[flow.Address]struct{}{
			address: {},
		},
		logger,
	)
}

// NewEVMSetupMigration returns a migration that sets up the EVM contract account.
// It performs the same operations as the EVM contract's initializer, calling the function EVM.setupHeartbeat,
// which in turn creates an EVM.Heartbeat resource, and writes it to the account's storage.
func NewEVMSetupMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
) RegistersMigration {

	systemContracts := systemcontracts.SystemContractsForChain(chainID)
	evmContract := systemContracts.EVMContract

	tx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(
			`
              import EVM from %s

              transaction {
                  prepare() {
                      EVM.setupHeartbeat()
                  }
              }
            `,
			evmContract.Address.HexWithPrefix(),
		)))

	return NewTransactionBasedMigration(
		tx,
		chainID,
		logger,
		map[flow.Address]struct{}{
			// The function call writes to the EVM contract account
			evmContract.Address: {},
			// The function call writes to the global account,
			// as the function creates a resource, which gets initialized with a UUID
			flow.Address{}: {},
		},
	)
}
