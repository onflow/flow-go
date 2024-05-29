package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

func NewAccountStorageMigration(
	address common.Address,
	log zerolog.Logger,
	chainID flow.ChainID,
	migrate func(*runtime.Storage, *interpreter.Interpreter) error,
) RegistersMigration {

	return func(registersByAccount *registers.ByAccount) error {

		// Create an interpreter migration runtime
		migrationRuntime, err := NewInterpreterMigrationRuntime(
			registersByAccount,
			chainID,
			InterpreterMigrationRuntimeConfig{},
		)
		if err != nil {
			return fmt.Errorf("failed to create interpreter migration runtime: %w", err)
		}

		// Run the migration
		storage := migrationRuntime.Storage
		inter := migrationRuntime.Interpreter

		err = migrate(storage, inter)
		if err != nil {
			return fmt.Errorf("failed to migrate storage: %w", err)
		}

		// Commit the changes
		err = storage.NondeterministicCommit(inter, false)
		if err != nil {
			return fmt.Errorf("failed to commit changes: %w", err)
		}

		// Check the health of the storage
		err = storage.CheckHealth()
		if err != nil {
			log.Err(err).Msg("storage health check failed")
		}

		// Finalize the transaction
		result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
		if err != nil {
			return fmt.Errorf("failed to finalize main transaction: %w", err)
		}

		// Merge the changes into the registers
		expectedAddresses := map[flow.Address]struct{}{
			flow.Address(address): {},
		}

		err = registers.ApplyChanges(
			registersByAccount,
			result.WriteSet,
			expectedAddresses,
			log,
		)
		if err != nil {
			return fmt.Errorf("failed to apply register changes: %w", err)
		}

		return nil
	}
}
