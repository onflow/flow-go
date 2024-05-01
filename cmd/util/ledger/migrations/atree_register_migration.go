package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	runtime2 "runtime"
	"time"

	"github.com/onflow/atree"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	util2 "github.com/onflow/flow-go/module/util"
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

	sampler := util2.NewTimedSampler(30 * time.Second)

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
	_ []*ledger.Payload,
	nWorkers int,
) error {
	m.log = log.With().Str("migration", "atree-register-migration").Logger()
	m.nWorkers = nWorkers

	return nil
}

func (m *AtreeRegisterMigrator) MigrateAccount(
	_ context.Context,
	address common.Address,
	oldPayloads []*ledger.Payload,
) ([]*ledger.Payload, error) {
	// create all the runtime components we need for the migration
	mr, err := NewAtreeRegisterMigratorRuntime(address, oldPayloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
	}

	// Check storage health before migration, if enabled.
	if m.checkStorageHealthBeforeMigration {
		err = checkStorageHealth(address, mr.Storage, oldPayloads)
		if err != nil {
			m.log.Warn().
				Err(err).
				Str("account", address.Hex()).
				Msg("storage health check before migration failed")
		}
	}

	// keep track of all storage maps that were accessed
	// if they are empty they won't be changed, but we still need to copy them over
	storageMapIds := make(map[string]struct{})

	// Do the storage conversion
	changes, err := m.migrateAccountStorage(mr, storageMapIds)
	if err != nil {
		if errors.Is(err, skippableAccountError) {
			return oldPayloads, nil
		}
		return nil, fmt.Errorf("failed to convert storage for address %s: %w", address.Hex(), err)
	}

	originalLen := len(oldPayloads)

	newPayloads, err := m.validateChangesAndCreateNewRegisters(mr, changes, storageMapIds)
	if err != nil {
		if errors.Is(err, skippableAccountError) {
			return oldPayloads, nil
		}
		return nil, err
	}

	if m.validateMigratedValues {
		err = validateCadenceValues(address, oldPayloads, newPayloads, m.log, m.logVerboseValidationError)
		if err != nil {
			if !m.continueMigrationOnValidationError {
				return nil, err
			}

			m.log.Error().
				Err(err).
				Hex("address", address[:]).
				Msg("failed not validate atree migration")
		}
	}

	newLen := len(newPayloads)

	if newLen > originalLen {
		// this is possible, its not something to be worried about.
		m.rw.Write(migrationProblem{
			Address: address.Hex(),
			Key:     "",
			Size:    len(mr.Snapshot.Payloads),
			Kind:    "more_registers_after_migration",
			Msg:     fmt.Sprintf("original: %d, new: %d", originalLen, newLen),
		})
	}

	// Check storage health after migration, if enabled.
	if m.checkStorageHealthAfterMigration {
		mr, err := NewAtreeRegisterMigratorRuntime(address, newPayloads)
		if err != nil {
			return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
		}

		err = checkStorageHealth(address, mr.Storage, newPayloads)
		if err != nil {
			m.log.Warn().
				Err(err).
				Str("account", address.Hex()).
				Msg("storage health check after migration failed")
		}
	}

	return newPayloads, nil
}

func (m *AtreeRegisterMigrator) migrateAccountStorage(
	mr *AtreeRegisterMigratorRuntime,
	storageMapIds map[string]struct{},
) (map[flow.RegisterID]flow.RegisterValue, error) {

	// iterate through all domains and migrate them
	for _, domain := range allStorageMapDomains {
		err := m.convertStorageDomain(mr, storageMapIds, domain)
		if err != nil {
			return nil, fmt.Errorf("failed to convert storage domain %s : %w", domain, err)
		}
	}

	err := mr.Storage.Commit(mr.Interpreter, false)
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
	storageMapIds map[string]struct{},
	domain string,
) error {

	storageMap := mr.Storage.GetStorageMap(mr.Address, domain, false)
	if storageMap == nil {
		// no storage for this domain
		return nil
	}
	storageMapIds[string(atree.SlabIndexToLedgerKey(storageMap.StorageID().Index))] = struct{}{}

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
				Size:    len(mr.Snapshot.Payloads),
				Key:     fmt.Sprintf("%v (%T)", key, key),
				Kind:    "migration_failure",
				Msg:     err.Error(),
			})
			return skippableAccountError
		}
	}

	return nil
}

func (m *AtreeRegisterMigrator) validateChangesAndCreateNewRegisters(
	mr *AtreeRegisterMigratorRuntime,
	changes map[flow.RegisterID]flow.RegisterValue,
	storageMapIds map[string]struct{},
) ([]*ledger.Payload, error) {
	originalPayloadsSnapshot := mr.Snapshot
	originalPayloads := originalPayloadsSnapshot.Payloads
	newPayloads := make([]*ledger.Payload, 0, len(originalPayloads))

	// store state payload so that it can be updated
	var statePayload *ledger.Payload
	progressLog := func(int) {}

	for id, value := range changes {
		progressLog(1)
		// delete all values that were changed from the original payloads so that we can
		// check what remains
		delete(originalPayloads, id)

		if len(value) == 0 {
			// value was deleted
			continue
		}

		ownerAddress, err := common.BytesToAddress([]byte(id.Owner))
		if err != nil {
			return nil, fmt.Errorf("failed to convert owner address: %w", err)
		}

		if ownerAddress != mr.Address {
			// something was changed that does not belong to this account. Log it.
			m.log.Error().
				Str("key", id.String()).
				Str("owner_address", ownerAddress.Hex()).
				Str("account", mr.Address.Hex()).
				Msg("key is part of the change set, but is for a different account")
		}

		key := convert.RegisterIDToLedgerKey(id)

		if statePayload == nil && isAccountKey(key) {
			statePayload = ledger.NewPayload(key, value)
			// we will append this later
			continue
		}

		newPayloads = append(newPayloads, ledger.NewPayload(key, value))
	}

	removedSize := uint64(0)

	// add all values that were not changed
	if len(originalPayloads) > 0 {

		for id, value := range originalPayloads {
			progressLog(1)

			if len(value.Value()) == 0 {
				// this is strange, but we don't want to add empty values. Log it.
				m.log.Warn().Msgf("empty value for key %s", id)
				continue
			}

			key := convert.RegisterIDToLedgerKey(id)
			if statePayload == nil && isAccountKey(key) {
				statePayload = value
				// we will append this later
				continue
			}

			if id.IsInternalState() {
				// this is expected. Move it to the new payloads
				newPayloads = append(newPayloads, value)
				continue
			}

			if _, isADomainKey := allStorageMapDomainsSet[id.Key]; isADomainKey {
				// this is expected. Move it to the new payloads
				newPayloads = append(newPayloads, value)
				continue
			}

			if _, ok := storageMapIds[id.Key]; ok {
				// This is needed because storage map can be empty.
				// Empty storage map only exists in old payloads because there isn't any element to migrate.
				newPayloads = append(newPayloads, value)
				continue
			}

			m.rw.Write(migrationProblem{
				Address: mr.Address.Hex(),
				Key:     id.String(),
				Size:    len(mr.Snapshot.Payloads),
				Kind:    "not_migrated",
				Msg:     fmt.Sprintf("%x", value.Value()),
			})

			size, err := payloadSize(key, value)
			if err != nil {
				return nil, fmt.Errorf("failed to get payload size: %w", err)
			}

			removedSize += size

			// this is ok
			// return nil, skippableAccountError
		}
	}

	if statePayload == nil {
		return nil, fmt.Errorf("state payload was not found")
	}

	// since some registers were removed, we need to update the storage used
	if removedSize > 0 {
		status, err := environment.AccountStatusFromBytes(statePayload.Value())
		if err != nil {
			return nil, fmt.Errorf("could not parse account status: %w", err)
		}

		status.SetStorageUsed(status.StorageUsed() - removedSize)

		newPayload, err := newPayloadWithValue(statePayload, status.ToBytes())
		if err != nil {
			return nil, fmt.Errorf("cannot create new payload with value: %w", err)
		}

		statePayload = newPayload
	}

	newPayloads = append(newPayloads, statePayload)

	return newPayloads, nil
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
