package migrations

import (
	"fmt"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func NewPreviewnetEVMContractMigration(
	log zerolog.Logger,
	chainID flow.ChainID,
	nWorkers int,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
		if chainID != flow.Previewnet {
			return nil, fmt.Errorf("migration is only supported on the previewnet chain")
		}

		migrationRuntime, err := NewMigratorRuntime(
			log,
			payloads,
			chainID,
			MigratorRuntimeConfig{},
			snapshot.SmallChangeSetSnapshot,
			nWorkers,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
		}

		address, err := evm.ContractAccountAddress(chainID)
		if err != nil {
			return nil, fmt.Errorf("failed to get EVM contract account address: %w", err)
		}

		exists, err := migrationRuntime.Accounts.ContractExists("EVM", address)
		if err != nil {
			return nil, fmt.Errorf("failed to check if EVM contract exists: %w", err)
		}

		if !exists {
			return nil, fmt.Errorf("EVM contract does not exist on %s", address)
		}

		sc := systemcontracts.SystemContractsForChain(chainID)

		contract := stdlib.ContractCode(
			sc.NonFungibleToken.Address,
			sc.FungibleToken.Address,
			sc.FlowToken.Address,
		)

		err = migrationRuntime.Accounts.SetContract("EVM", address, contract)
		if err != nil {
			return nil, fmt.Errorf("failed to set EVM contract: %w", err)
		}

		// We don't have to commit the contract updates,
		// because we are changing the code directly in storage.
		const commitContractUpdates = false
		err = migrationRuntime.Storage.Commit(migrationRuntime.Interpreter, commitContractUpdates)
		if err != nil {
			return nil, fmt.Errorf("failed to commit changes: %w", err)
		}

		err = migrationRuntime.Storage.CheckHealth()
		if err != nil {
			log.Err(err).Msg("storage health check failed")
		}

		// finalize the transaction
		result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
		if err != nil {
			return nil, fmt.Errorf("failed to finalize main transaction: %w", err)
		}

		// Merge the changes to the original payloads.
		expectedAddresses := map[flow.Address]struct{}{
			address: {},
		}

		newPayloads, err := migrationRuntime.Snapshot.ApplyChangesAndGetNewPayloads(
			result.WriteSet,
			expectedAddresses,
			log,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to merge register changes: %w", err)
		}

		return newPayloads, nil
	}
}
