package migrations

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

// AccountUsageMigration iterates through each payload, and calculate the storage usage
// and update the accounts status with the updated storage usage. It also upgrades the
// account status registers to the latest version.
type AccountUsageMigration struct {
	log zerolog.Logger
	rw  reporters.ReportWriter
}

var _ AccountBasedMigration = &AccountUsageMigration{}

func NewAccountUsageMigration(rw reporters.ReportWriterFactory) *AccountUsageMigration {
	return &AccountUsageMigration{
		rw: rw.ReportWriter("account-usage-migration"),
	}
}

func (m *AccountUsageMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	_ int,
) error {
	m.log = log.With().Str("component", "AccountUsageMigration").Logger()
	return nil
}

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
	actualUsed := uint64(0)

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

		actualUsed += uint64(environment.RegisterSize(
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
		return fmt.Errorf("could not find account status register")
	}

	// reading the status will upgrade the status to the latest version, so it might
	// have a different size than the one in the register
	newStatusValue := status.ToBytes()
	statusSizeDiff := len(newStatusValue) - len(statusValue)

	// the status size diff should be added to the actual size
	if statusSizeDiff < 0 {
		if uint64(-statusSizeDiff) > actualUsed {
			log.Error().
				Str("account", address.HexWithPrefix()).
				Msgf("account storage used would be negative")
			return nil
		}

		actualUsed = actualUsed - uint64(-statusSizeDiff)
	} else if statusSizeDiff > 0 {
		actualUsed = actualUsed + uint64(statusSizeDiff)
	}

	currentUsed := status.StorageUsed()

	// update storage used if the actual size is different from the size in the status register
	// or if the status size is different.
	if actualUsed != currentUsed || statusSizeDiff != 0 {
		// update storage used
		status.SetStorageUsed(actualUsed)

		err = accountRegisters.Set(
			string(address[:]),
			flow.AccountStatusKey,
			status.ToBytes(),
		)
		if err != nil {
			return fmt.Errorf("could not update account status register: %w", err)
		}

		m.rw.Write(accountUsageMigrationReportData{
			AccountAddress: address,
			OldStorageUsed: currentUsed,
			NewStorageUsed: actualUsed,
		})
	}

	return nil
}

type accountUsageMigrationReportData struct {
	AccountAddress common.Address
	OldStorageUsed uint64
	NewStorageUsed uint64
}
