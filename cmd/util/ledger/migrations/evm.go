package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
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
	evmContractAddress := common.Address(evmContract.Address)

	return NewAccountStorageMigration(
		evmContractAddress,
		logger,
		chainID,
		func(storage *runtime.Storage, inter *interpreter.Interpreter) error {

			// Get the storage map for the EVM contract account

			storageMap := storage.GetStorageMap(
				evmContractAddress,
				common.PathDomainStorage.Identifier(),
				false,
			)
			if storageMap == nil {
				return fmt.Errorf("failed to get storage map for EVM contract account")
			}

			// Check if the EVM.Heartbeat resource already exists

			key := interpreter.StringStorageMapKey("EVMHeartbeat")

			if storageMap.ValueExists(key) {
				return nil
			}

			// Create the EVM.Heartbeat resource and write it to storage

			heartbeatResource := interpreter.NewCompositeValue(
				inter,
				interpreter.EmptyLocationRange,
				common.AddressLocation{
					Address: evmContractAddress,
					Name:    stdlib.ContractName,
				},
				"EVM.Heartbeat",
				common.CompositeKindResource,
				nil,
				evmContractAddress,
			)

			storageMap.WriteValue(
				inter,
				key,
				heartbeatResource,
			)

			return nil
		},
	)
}
