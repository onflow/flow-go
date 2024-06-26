package migrations

import (
	"context"
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

// AccountUsageMigration iterates through each payload, and calculate the storage usage
// and update the accounts status with the updated storage usage
type AccountUsageMigration struct {
	log zerolog.Logger
}

var _ AccountBasedMigration = &AccountUsageMigration{}

func (m *AccountUsageMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	_ int,
) error {
	m.log = log.With().Str("component", "AccountUsageMigration").Logger()
	return nil
}

const oldAccountStatusSize = 25

func (m *AccountUsageMigration) Close() error {
	return nil
}

func (m *AccountUsageMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	var status *environment.AccountStatus
	var statusValue []byte
	actualSize := uint64(0)

	// Find the account status register,
	// and calculate the storage usage
	err := accountRegisters.ForEach(func(owner, key string, value []byte) error {

		if key == flow.AccountStatusKey {
			statusValue = value

			var err error
			status, err = environment.AccountStatusFromBytes(value)
			if err != nil {
				return fmt.Errorf("could not parse account status: %w", err)
			}
		}

		actualSize += uint64(environment.RegisterSize(
			flow.RegisterID{
				Owner: owner,
				Key:   key,
			},
			value,
		))

		return nil
	})
	if err != nil {
		return fmt.Errorf(
			"could not iterate through registers of account %s: %w",
			address.HexWithPrefix(),
			err,
		)
	}

	if status == nil {
		log.Error().
			Str("account", address.HexWithPrefix()).
			Msgf("could not find account status register")
		return nil
	}

	isOldVersionOfStatusRegister := len(statusValue) == oldAccountStatusSize

	same := m.compareUsage(isOldVersionOfStatusRegister, status, actualSize, address)
	if same {
		// there is no problem with the usage, return
		return nil
	}

	if isOldVersionOfStatusRegister {
		// size will grow by 8 bytes because of the on-the-fly
		// migration of account status in AccountStatusFromBytes
		actualSize += 8
	}

	// update storage used
	status.SetStorageUsed(actualSize)

	err = accountRegisters.Set(
		string(address[:]),
		flow.AccountStatusKey,
		status.ToBytes(),
	)
	if err != nil {
		return fmt.Errorf("could not update account status register: %w", err)
	}

	return nil
}

func (m *AccountUsageMigration) compareUsage(
	isOldVersionOfStatusRegister bool,
	status *environment.AccountStatus,
	actualSize uint64,
	address common.Address,
) bool {
	oldSize := status.StorageUsed()
	if isOldVersionOfStatusRegister {
		// size will be reported as 8 bytes larger than the actual size due to on-the-fly
		// migration of account status in AccountStatusFromBytes
		oldSize -= 8
	}

	if oldSize != actualSize {
		m.log.Warn().
			Uint64("old_size", oldSize).
			Uint64("new_size", actualSize).
			Str("account", address.HexWithPrefix()).
			Msg("account storage used usage mismatch")
		return false
	}
	return true
}
