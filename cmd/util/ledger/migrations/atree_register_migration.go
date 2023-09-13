package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	util2 "github.com/onflow/flow-go/module/util"
)

func MigrateAtreeRegisters(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	nWorker int,
) func([]*ledger.Payload) ([]*ledger.Payload, error) {
	return CreateAccountBasedMigration(
		log,
		func(_ []*ledger.Payload, nWorker int) (AccountMigrator, error) {
			return NewMigrator(
				log.With().Str("component", "atree-register-migrator").Logger(),
				rwf,
			)
		},
		nWorker,
	)
}

// AtreeRegisterMigrator is a migrator that converts the storage of an account from the
// old atree format to the new atree format.
// Account "storage used" should be correctly updated after the migration.
type AtreeRegisterMigrator struct {
	log zerolog.Logger

	sampler zerolog.Sampler
	rw      reporters.ReportWriter
	rwf     reporters.ReportWriterFactory
}

var _ AccountMigrator = (*AtreeRegisterMigrator)(nil)
var _ io.Closer = (*AtreeRegisterMigrator)(nil)

func NewMigrator(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
) (*AtreeRegisterMigrator, error) {

	log.Info().Msgf("created snapshot")
	sampler := util2.NewTimedSampler(30 * time.Second)

	migrator := &AtreeRegisterMigrator{
		log:     log,
		sampler: sampler,

		rwf: rwf,
		rw:  rwf.ReportWriter("atree-register-migrator"),
	}

	return migrator, nil
}

func (m *AtreeRegisterMigrator) Close() error {
	m.rw.Close()
	return nil
}

var skippableAccountError = errors.New("account is skippable")

func (m *AtreeRegisterMigrator) MigratePayloads(
	_ context.Context,
	address common.Address,
	oldPayloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	if address == common.ZeroAddress {
		return oldPayloads, nil
	}

	if reason, ok := knownProblematicAccounts[address]; ok {
		m.log.Info().
			Str("account", address.Hex()).
			Str("reason", reason).
			Msg("Account is known to have issues. Skipping it.")
		return oldPayloads, nil
	}

	// create all the runtime components we need for the migration
	mr, err := m.newMigratorRuntime(address, oldPayloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
	}

	//// check the storage health
	//healthOk, err := m.checkStorageHealth(mr)
	//if err != nil {
	//	return nil, fmt.Errorf("storage health issues for address %s: %w", address.Hex(), err)
	//}

	// Do the storage conversion
	changes, err := m.migrateAccountStorage(mr)
	if err != nil {
		if errors.Is(err, skippableAccountError) {
			return oldPayloads, nil
		}
		return nil, fmt.Errorf("failed to convert storage for address %s: %w", address.Hex(), err)
	}

	originalLen := len(oldPayloads)

	newPayloads, err := m.validateChangesAndCreateNewRegisters(mr, changes)
	if err != nil {
		if errors.Is(err, skippableAccountError) {
			return oldPayloads, nil
		}
		return nil, err
	}

	newLen := len(newPayloads)

	if newLen > originalLen {
		m.rw.Write(migrationProblem{
			Address: address.Hex(),
			Key:     "",
			Kind:    "More registers after migration",
			Msg:     fmt.Sprintf("original: %d, new: %d", originalLen, newLen),
		})
	}

	if _, ok := accountsToLog[address]; ok {
		m.dumpAccount(mr.Address, oldPayloads, newPayloads)
	}

	return newPayloads, nil
}

func (m *AtreeRegisterMigrator) migrateAccountStorage(
	mr *migratorRuntime,
) (map[flow.RegisterID]flow.RegisterValue, error) {

	// iterate through all domains and migrate them
	for _, domain := range domains {
		err := m.convertStorageDomain(mr.Address, mr.Storage, mr.Interpreter, domain)
		if err != nil {
			return nil, fmt.Errorf("failed to convert storage domain %s : %w", domain, err)
		}
	}

	// commit the storage changes
	err := mr.Storage.Commit(mr.Interpreter, true)
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
	address common.Address,
	storage *runtime.Storage,
	inter *interpreter.Interpreter,
	domain string,
) error {

	storageMap := storage.GetStorageMap(address, domain, false)
	if storageMap == nil {
		// no storage for this domain
		return nil
	}

	iterator := storageMap.Iterator(util.NopMemoryGauge{})
	keys := make([]interpreter.StringStorageMapKey, 0)
	// to be safe avoid modifying the map while iterating
	for {
		key := iterator.NextKey()
		if key == nil {
			break
		}

		stringKey, ok := key.(interpreter.StringAtreeValue)
		if !ok {
			return fmt.Errorf("invalid key type %T, expected interpreter.StringAtreeValue", key)
		}

		keys = append(keys, interpreter.StringStorageMapKey(stringKey))
	}

	for _, key := range keys {
		err := func() error {
			var value interpreter.Value

			err := capturePanic(func() {
				value = storageMap.ReadValue(util.NopMemoryGauge{}, key)
			})
			if err != nil {
				return fmt.Errorf("failed to read value for key %s: %w", key, err)
			}

			err = capturePanic(func() {
				// force the value to be read entirely
				value = value.Clone(inter)
			})
			if err != nil {
				return fmt.Errorf("failed to clone value for key %s: %w", key, err)
			}

			err = capturePanic(func() {
				// set value will first purge the old value
				storageMap.SetValue(inter, key, value)
			})
			if err != nil {
				return fmt.Errorf("failed to set value for key %s: %w", key, err)
			}

			return nil
		}()
		if err != nil {

			m.rw.Write(migrationProblem{
				Address: address.Hex(),
				Key:     string(key),
				Kind:    "migration_failure",
				Msg:     err.Error(),
			})
			return skippableAccountError
		}
	}

	return nil
}

func (m *AtreeRegisterMigrator) newMigratorRuntime(
	address common.Address,
	payloads []*ledger.Payload,
) (
	*migratorRuntime,
	error,
) {
	snapshot, err := util.NewPayloadSnapshot(payloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload snapshot: %w", err)
	}
	transactionState := state.NewTransactionState(snapshot, state.DefaultParameters())
	accounts := environment.NewAccounts(transactionState)

	accountsAtreeLedger := util.NewAccountsAtreeLedger(accounts)
	storage := runtime.NewStorage(accountsAtreeLedger, util.NopMemoryGauge{})

	ri := &util.MigrationRuntimeInterface{
		Accounts: accounts,
	}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{
		AccountLinkingEnabled: true,
		// Attachments are enabled everywhere except for Mainnet
		AttachmentsEnabled: true,
		// Capability Controllers are enabled everywhere except for Mainnet
		CapabilityControllersEnabled: true,
	})

	env.Configure(
		ri,
		runtime.NewCodesAndPrograms(),
		storage,
		runtime.NewCoverageReport(),
	)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		env.InterpreterConfig)
	if err != nil {
		return nil, err
	}

	return &migratorRuntime{
		Address:          address,
		Payloads:         payloads,
		Snapshot:         snapshot,
		TransactionState: transactionState,
		Interpreter:      inter,
		Storage:          storage,
	}, nil
}

type migratorRuntime struct {
	Snapshot         *util.PayloadSnapshot
	TransactionState state.NestedTransactionPreparer
	Interpreter      *interpreter.Interpreter
	Storage          *runtime.Storage
	Payloads         []*ledger.Payload
	Address          common.Address
}

func (m *AtreeRegisterMigrator) validateChangesAndCreateNewRegisters(
	mr *migratorRuntime,
	changes map[flow.RegisterID]flow.RegisterValue,
) (
	[]*ledger.Payload,
	error,
) {
	originalPayloadsSnapshot := mr.Snapshot
	originalPayloads := originalPayloadsSnapshot.Payloads
	newPayloads := make([]*ledger.Payload, 0, len(originalPayloads))

	for id, value := range changes {
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

		if ownerAddress.Hex() != mr.Address.Hex() {
			// something was changed that does not belong to this account. Log it.
			m.log.Error().
				Str("key", id.String()).
				Str("owner_address", ownerAddress.Hex()).
				Str("account", mr.Address.Hex()).
				Msg("key is part of the change set, but is for a different account")

			return nil, fmt.Errorf("register for a different account was produced during migration")
		}

		newPayloads = append(newPayloads, ledger.NewPayload(util.RegisterIDToKey(id), value))
	}

	//hasMissingKeys := false

	// add all values that were not changed
	for id, value := range originalPayloads {
		if len(value.Value()) == 0 {
			// this is strange, but we don't want to add empty values. Log it.
			m.log.Warn().Msgf("empty value for key %s", id)
			continue
		}
		if id.IsInternalState() {
			// this is expected. Move it to the new payload
			newPayloads = append(newPayloads, value)
			continue
		}

		isADomainKey := false
		for _, domain := range domains {
			if id.Key == domain {
				isADomainKey = true
				break
			}
		}
		if isADomainKey {
			// TODO: check if this is really expected
			// this is expected. Move it to the new payload
			newPayloads = append(newPayloads, value)
			continue
		}

		m.rw.Write(migrationProblem{
			Address: mr.Address.Hex(),
			Key:     id.String(),
			Kind:    "not_migrated",
			Msg:     fmt.Sprintf("%x", value),
		})

		return nil, skippableAccountError
	}

	//if hasMissingKeys && m.sampler.Sample(zerolog.InfoLevel) {
	//	m.dumpAccount(mr.Address, mr.Payloads, newPayloads)
	//}
	return newPayloads, nil
}

func (m *AtreeRegisterMigrator) dumpAccount(address common.Address, before, after []*ledger.Payload) {
	beforeWriter := m.rwf.ReportWriter(fmt.Sprintf("account-before-%s", address.Hex()))
	for _, p := range before {
		beforeWriter.Write(p)
	}
	beforeWriter.Close()

	afterWriter := m.rwf.ReportWriter(fmt.Sprintf("account-after-%s", address.Hex()))
	for _, p := range after {
		afterWriter.Write(p)
	}
	afterWriter.Close()
}

//
//func (m *AtreeRegisterMigrator) checkStorageHealth(
//	mr *migratorRuntime,
//) (
//	err error,
//) {
//
//	storageIDs := make([]atree.StorageID, 0, len(mr.Payloads))
//
//	for _, payload := range mr.Payloads {
//		key, err := payload.Key()
//		if err != nil {
//			return fmt.Errorf("failed to get payload key: %w", err)
//		}
//
//		id, err := util.KeyToRegisterID(key)
//		if err != nil {
//			return fmt.Errorf("failed to convert key to register ID: %w", err)
//		}
//
//		if id.IsInternalState() {
//			continue
//		}
//
//		if !strings.HasPrefix(id.Key, atree.LedgerBaseStorageSlabPrefix) {
//			continue
//		}
//
//		storageIDs = append(storageIDs, atree.StorageID{
//			Address: atree.Address([]byte(id.Owner)),
//			Index:   atree.StorageIndex([]byte(id.Key)[1:]),
//		})
//	}
//
//	// load (but don't create) storage map in Cadence storage
//	for _, domain := range domains {
//		_ = mr.Storage.GetStorageMap(mr.Address, domain, false)
//
//	}
//
//	// load slabs in atree storage
//	for _, id := range storageIDs {
//		_, _, _ = mr.Storage.Retrieve(id)
//	}
//
//	err = mr.Storage.CheckHealth()
//	if err != nil {
//		m.rw.Write(migrationProblem{
//			Address: mr.Address.Hex(),
//			Key:     "",
//			Kind:    "storage_health_problem",
//			Msg:     err.Error(),
//		})
//
//		return skippableAccountError
//	}
//	return nil
//
//}

// capturePanic captures panics and converts them to errors
// this is needed for some cadence functions that panic on error
func capturePanic(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	f()

	return
}

// convert all domains
var domains = []string{
	common.PathDomainStorage.Identifier(),
	common.PathDomainPrivate.Identifier(),
	common.PathDomainPublic.Identifier(),
	runtime.StorageDomainContract,
}

// migrationProblem is a struct for reporting errors
type migrationProblem struct {
	Address string
	Key     string
	Kind    string
	Msg     string
}

var knownProblematicAccounts = map[common.Address]string{
	// Testnet accounts with broken contracts
	mustHexToAddress("434a1f199a7ae3ba"): "Broken contract FanTopPermission",
	mustHexToAddress("454c9991c2b8d947"): "Broken contract Test",
	mustHexToAddress("48602d8056ff9d93"): "Broken contract FanTopPermission",
	mustHexToAddress("5d63c34d7f05e5a4"): "Broken contract FanTopPermission",
	mustHexToAddress("5e3448b3cffb97f2"): "Broken contract FanTopPermission",
	mustHexToAddress("7d8c7e050c694eaa"): "Broken contract Test",
	mustHexToAddress("ba53f16ede01972d"): "Broken contract FanTopPermission",
	mustHexToAddress("c843c1f5a4805c3a"): "Broken contract FanTopPermission",
	mustHexToAddress("48d3be92e6e4a973"): "Broken contract FanTopPermission",
}

var accountsToLog = map[common.Address]string{
	// Testnet accounts
	mustHexToAddress("ff8bbaa2905b7bd6"): "Small account that increased in size",
}

func mustHexToAddress(hex string) common.Address {
	address, err := common.HexToAddress(hex)
	if err != nil {
		panic(err)
	}
	return address
}
