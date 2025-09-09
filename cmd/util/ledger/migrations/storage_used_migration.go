package migrations

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"

	"github.com/onflow/cadence/common"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

// Sequence number registers are created on demand to reduce register count
// and they are in their own registers to avoid blocking concurrent execution.
// We also need to include sequence number register sizes in the storage used
// computation before the sequence number register is created (i.e.,
// we update storage used when account public key is created) to unblock
// some use cases of concurrent execution.
//
// In other words,
// - When account public key is appened, predefined sequence number register size
// is included in storage used.
// - When sequence number register is created, storage size isn't affected.
//
// To simplify computation and avoid blocking concurrent execution, the storage used
// computation always uses 1 to indicate the number of bytes used to store each
// sequence number's value.

func predefinedSequenceNumberPayloadSizes(owner string, startKeyIndex uint32, endKeyIndex uint32) uint64 {
	size := uint64(0)
	for i := startKeyIndex; i < endKeyIndex; i++ {
		size += environment.PredefinedSequenceNumberPayloadSize(flow.BytesToAddress([]byte(owner)), i)
	}
	return size
}

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

	csvReportHeader := []string{
		"address",
		"old_storage_used",
		"new_storage_used",
	}

	m.rw.Write(csvReportHeader)

	return nil
}

func (m *AccountUsageMigration) Close() error {
	m.rw.Close()
	return nil
}

func (m *AccountUsageMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	var status *environment.AccountStatus
	var statusValue []byte
	var accountPublicKeyCount uint32

	actualUsed := uint64(0)

	// Find the account status register,
	// and calculate the storage usage
	err := accountRegisters.ForEach(func(owner, key string, value []byte) error {

		if strings.HasPrefix(key, flow.SequenceNumberRegisterKeyPrefix) {
			// DO NOT include individual sequence number registers in storage used here.
			// Instead, we include storage used for all account public key at key index >= 1
			// later in this function.
			return nil
		}

		if key == flow.AccountStatusKey {
			statusValue = value

			var err error
			status, err = environment.AccountStatusFromBytes(value)
			if err != nil {
				return fmt.Errorf("could not parse account status: %w", err)
			}

			accountPublicKeyCount = status.AccountPublicKeyCount()
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
			return fmt.Errorf("account storage used would be negative")
		}

		actualUsed = actualUsed - uint64(-statusSizeDiff)
	} else if statusSizeDiff > 0 {
		actualUsed = actualUsed + uint64(statusSizeDiff)
	}

	if accountPublicKeyCount > 1 {
		// Include predefined sequence number payload size per key for all account public key at index >= 1.
		// NOTE: sequence number for the first account public key is included in the
		// first account public key register, so it doesn't need to be included here.
		actualUsed += predefinedSequenceNumberPayloadSizes(string(address[:]), 1, accountPublicKeyCount)
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

		m.rw.Write([]string{
			address.Hex(),
			strconv.FormatUint(currentUsed, 10),
			strconv.FormatUint(actualUsed, 10),
		})
	}

	return nil
}

// nolint:unused
type accountUsageMigrationReportData struct {
	AccountAddress string
	OldStorageUsed uint64
	NewStorageUsed uint64
}
