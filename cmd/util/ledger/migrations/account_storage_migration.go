package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func NewAccountStorageMigration(
	address common.Address,
	log zerolog.Logger,
	migrate func(*runtime.Storage, *interpreter.Interpreter) error,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {

		migrationRuntime, err := NewMigratorRuntime(
			address,
			payloads,
			util.RuntimeInterfaceConfig{},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
		}

		storage := migrationRuntime.Storage
		inter := migrationRuntime.Interpreter

		err = migrate(storage, inter)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate storage: %w", err)
		}

		err = storage.Commit(inter, false)
		if err != nil {
			return nil, fmt.Errorf("failed to commit changes: %w", err)
		}

		err = storage.CheckHealth()
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
			flow.Address(address): {},
		}

		newPayloads, err := MergeRegisterChanges(
			migrationRuntime.Snapshot.Payloads,
			result.WriteSet,
			expectedAddresses,
			nil,
			log,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to merge register changes: %w", err)
		}

		return newPayloads, nil
	}
}
