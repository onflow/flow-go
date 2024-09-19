package migrations

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

func NewAccountCreationMigration(
	address flow.Address,
	logger zerolog.Logger,
) RegistersMigration {

	return func(registersByAccount *registers.ByAccount) error {

		migrationRuntime := NewBasicMigrationRuntime(registersByAccount)

		// Check if the account already exists
		exists, err := migrationRuntime.Accounts.Exists(address)
		if err != nil {
			return fmt.Errorf(
				"failed to check if account %s exists: %w",
				address,
				err,
			)
		}

		// If the account already exists, do nothing
		if exists {
			logger.Info().Msgf("account %s already exists", address)
			return nil
		}

		// Create the account
		err = migrationRuntime.Accounts.Create(nil, address)
		if err != nil {
			return fmt.Errorf(
				"failed to create account %s: %w",
				address,
				err,
			)
		}

		logger.Info().Msgf("created account %s", address)

		// Commit the changes to the migrated registers
		err = migrationRuntime.Commit(
			map[flow.Address]struct{}{
				address: {},
			},
			logger,
		)
		if err != nil {
			return fmt.Errorf("failed to commit account creation: %w", err)
		}

		return nil
	}
}
