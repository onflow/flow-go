package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	runtime2 "runtime"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/util"
)

// AtreeRegisterMigrator is a migrator that converts the storage of an account from the
// old atree format to the new atree format.
// Account "storage used" should be correctly updated after the migration.
type AtreeRegisterMigrator struct {
	log zerolog.Logger

	sampler zerolog.Sampler
	rw      reporters.ReportWriter

	nWorkers int

	validateMigratedValues             bool
	logVerboseValidationError          bool
	continueMigrationOnValidationError bool
	checkStorageHealthBeforeMigration  bool
	checkStorageHealthAfterMigration   bool
}

var _ AccountBasedMigration = (*AtreeRegisterMigrator)(nil)
var _ io.Closer = (*AtreeRegisterMigrator)(nil)

func NewAtreeRegisterMigrator(
	rwf reporters.ReportWriterFactory,
	validateMigratedValues bool,
	logVerboseValidationError bool,
	continueMigrationOnValidationError bool,
	checkStorageHealthBeforeMigration bool,
	checkStorageHealthAfterMigration bool,
) *AtreeRegisterMigrator {

	sampler := util.NewTimedSampler(30 * time.Second)

	migrator := &AtreeRegisterMigrator{
		sampler:                            sampler,
		rw:                                 rwf.ReportWriter("atree-register-migrator"),
		validateMigratedValues:             validateMigratedValues,
		logVerboseValidationError:          logVerboseValidationError,
		continueMigrationOnValidationError: continueMigrationOnValidationError,
		checkStorageHealthBeforeMigration:  checkStorageHealthBeforeMigration,
		checkStorageHealthAfterMigration:   checkStorageHealthAfterMigration,
	}

	return migrator
}

func (m *AtreeRegisterMigrator) Close() error {
	// close the report writer so it flushes to file
	m.rw.Close()

	return nil
}

func (m *AtreeRegisterMigrator) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	nWorkers int,
) error {
	m.log = log.With().Str("migration", "atree-register-migration").Logger()
	m.nWorkers = nWorkers

	return nil
}

func (m *AtreeRegisterMigrator) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {
	// create all the runtime components we need for the migration
	mr, err := NewAtreeRegisterMigratorRuntime(address, accountRegisters)
	if err != nil {
		return fmt.Errorf("failed to create migrator runtime: %w", err)
	}

	var oldPayloads []*ledger.Payload
	if m.validateMigratedValues {
		// Only need to capture old payloads when validation is on.
		oldPayloads = accountRegisters.Payloads()
	}

	// Preload account payloads
	err = loadAtreeSlabsInRegisterStorage(mr.Storage, accountRegisters, m.nWorkers)
	if err != nil {
		return err
	}

	// Check storage health before migration, if enabled.
	if m.checkStorageHealthBeforeMigration {
		// Atree slabs are preloaded already.
		err = checkStorageHealth(address, mr.Storage, accountRegisters, m.nWorkers, false)
		if err != nil {
			m.log.Warn().
				Err(err).
				Str("account", address.Hex()).
				Msg("storage health check before migration failed")
		}
	}

	originalLen := accountRegisters.Count()

	// Do the storage conversion
	changes, err := m.migrateAccountStorage(mr)
	if err != nil {
		if errors.Is(err, skippableAccountError) {
			return nil
		}
		return fmt.Errorf("failed to convert storage for address %s: %w", address.Hex(), err)
	}

	// Merge the changes to the original payloads.
	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(address): {},
	}

	err = registers.ApplyChanges(
		accountRegisters,
		changes,
		expectedAddresses,
		m.log,
	)
	if err != nil {
		return fmt.Errorf("failed to apply changes to account registers: %w", err)
	}

	newLen := accountRegisters.Count()

	if newLen > originalLen {
		// this is possible, its not something to be worried about.
		m.rw.Write(migrationProblem{
			Address: address.Hex(),
			Key:     "",
			Size:    newLen,
			Kind:    "more_registers_after_migration",
			Msg:     fmt.Sprintf("original: %d, new: %d", originalLen, newLen),
		})
	}

	if m.validateMigratedValues {
		newPayloads := accountRegisters.Payloads()

		err = validateCadenceValues(
			address,
			oldPayloads,
			newPayloads,
			m.log,
			m.logVerboseValidationError,
			m.nWorkers,
		)
		if err != nil {
			if !m.continueMigrationOnValidationError {
				return err
			}

			m.log.Error().
				Err(err).
				Hex("address", address[:]).
				Msg("failed not validate atree migration")
		}
	}

	// Check storage health after migration, if enabled.
	if m.checkStorageHealthAfterMigration {
		mr, err := NewAtreeRegisterMigratorRuntime(address, accountRegisters)
		if err != nil {
			return fmt.Errorf("failed to create migrator runtime: %w", err)
		}

		err = checkStorageHealth(address, mr.Storage, accountRegisters, m.nWorkers, true)
		if err != nil {
			m.log.Warn().
				Err(err).
				Str("account", address.Hex()).
				Msg("storage health check after migration failed")
		}
	}

	return nil
}

func (m *AtreeRegisterMigrator) migrateAccountStorage(
	mr *AtreeRegisterMigratorRuntime,
) (map[flow.RegisterID]flow.RegisterValue, error) {

	// iterate through all domains and migrate them
	for _, domain := range allStorageMapDomains {
		err := m.convertStorageDomain(mr, domain)
		if err != nil {
			return nil, fmt.Errorf("failed to convert storage domain %s : %w", domain, err)
		}
	}

	err := mr.Storage.PersistentSlabStorage.NondeterministicFastCommit(m.nWorkers)
	if err != nil {
		return nil, fmt.Errorf("failed to commit storage: %w", err)
	}

	// finalize the transaction
	result, err := mr.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	return result.WriteSet, nil
}

func (m *AtreeRegisterMigrator) convertStorageDomain(
	mr *AtreeRegisterMigratorRuntime,
	domain string,
) error {

	storageMap := mr.Storage.GetStorageMap(mr.Address, domain, false)
	if storageMap == nil {
		// no storage for this domain
		return nil
	}

	iterator := storageMap.Iterator(nil)
	keys := make([]interpreter.StorageMapKey, 0, storageMap.Count())
	// to be safe avoid modifying the map while iterating
	for {
		key := iterator.NextKey()
		if key == nil {
			break
		}

		switch key := key.(type) {
		case interpreter.StringAtreeValue:
			keys = append(keys, interpreter.StringStorageMapKey(key))

		case interpreter.Uint64AtreeValue:
			keys = append(keys, interpreter.Uint64StorageMapKey(key))

		default:
			return fmt.Errorf("invalid key type %T, expected interpreter.StringAtreeValue or interpreter.Uint64AtreeValue", key)
		}
	}

	for _, key := range keys {
		err := func() error {
			var value interpreter.Value

			err := capturePanic(func() {
				value = storageMap.ReadValue(nil, key)
			})
			if err != nil {
				return fmt.Errorf("failed to read value for key %s: %w", key, err)
			}

			value, err = m.cloneValue(mr, value)

			if err != nil {
				return fmt.Errorf("failed to clone value for key %s: %w", key, err)
			}

			err = capturePanic(func() {
				// set value will first purge the old value
				storageMap.SetValue(mr.Interpreter, key, value)
			})

			if err != nil {
				return fmt.Errorf("failed to set value for key %s: %w", key, err)
			}

			return nil
		}()
		if err != nil {
			m.rw.Write(migrationProblem{
				Address: mr.Address.Hex(),
				Key:     fmt.Sprintf("%v (%T)", key, key),
				Kind:    "migration_failure",
				Msg:     err.Error(),
			})
			return skippableAccountError
		}
	}

	return nil
}

func (m *AtreeRegisterMigrator) cloneValue(
	mr *AtreeRegisterMigratorRuntime,
	value interpreter.Value,
) (interpreter.Value, error) {

	err := capturePanic(func() {
		// force the value to be read entirely
		value = value.Clone(mr.Interpreter)
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// capturePanic captures panics and converts them to errors
// this is needed for some cadence functions that panic on error
func capturePanic(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var stack [100000]byte
			n := runtime2.Stack(stack[:], false)
			fmt.Printf("%s", stack[:n])

			switch x := r.(type) {
			case runtime.Error:
				err = fmt.Errorf("runtime error @%s: %w", x.Location, x)
			case error:
				err = x
			default:
				err = fmt.Errorf("panic: %v", r)
			}
		}
	}()
	f()

	return
}

// migrationProblem is a struct for reporting errors
type migrationProblem struct {
	Address string
	// Size is the account size in register count
	Size int
	Key  string
	Kind string
	Msg  string
}

var skippableAccountError = errors.New("account can be skipped")
