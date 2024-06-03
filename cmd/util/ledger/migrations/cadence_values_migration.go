package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"errors"

	"github.com/onflow/cadence/migrations"
	"github.com/onflow/cadence/migrations/capcons"
	"github.com/onflow/cadence/migrations/entitlements"
	"github.com/onflow/cadence/migrations/statictypes"
	"github.com/onflow/cadence/migrations/string_normalization"
	"github.com/onflow/cadence/migrations/type_keys"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	cadenceErrors "github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type CadenceBaseMigration struct {
	name                              string
	log                               zerolog.Logger
	reporter                          reporters.ReportWriter
	diffReporter                      reporters.ReportWriter
	logVerboseDiff                    bool
	verboseErrorOutput                bool
	checkStorageHealthBeforeMigration bool
	valueMigrations                   func(
		inter *interpreter.Interpreter,
		accounts environment.Accounts,
		reporter *cadenceValueMigrationReporter,
	) []migrations.ValueMigration
	interpreterMigrationRuntimeConfig InterpreterMigrationRuntimeConfig
	errorMessageHandler               *errorMessageHandler
	programs                          map[runtime.Location]*interpreter.Program
	chainID                           flow.ChainID
	nWorkers                          int
}

var _ AccountBasedMigration = (*CadenceBaseMigration)(nil)
var _ io.Closer = (*CadenceBaseMigration)(nil)

func (m *CadenceBaseMigration) Close() error {
	// Close the report writer so it flushes to file.
	m.reporter.Close()

	if m.diffReporter != nil {
		m.diffReporter.Close()
	}

	return nil
}

func (m *CadenceBaseMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	nWorkers int,
) error {
	m.log = log.With().Str("migration", m.name).Logger()
	m.nWorkers = nWorkers

	// During the migration, we only provide already checked programs,
	// no parsing/checking of contracts is expected.

	m.interpreterMigrationRuntimeConfig = InterpreterMigrationRuntimeConfig{
		GetOrLoadProgram: func(
			location runtime.Location,
			_ func() (*interpreter.Program, error),
		) (*interpreter.Program, error) {
			program, ok := m.programs[location]
			if !ok {
				return nil, fmt.Errorf("program not found: %s", location)
			}
			return program, nil
		},
		GetCode: func(_ common.AddressLocation) ([]byte, error) {
			return nil, fmt.Errorf("unexpected call to GetCode")
		},
		GetContractNames: func(address flow.Address) ([]string, error) {
			return nil, fmt.Errorf("unexpected call to GetContractNames")
		},
	}

	return nil
}

func (m *CadenceBaseMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {
	var oldPayloadsForDiff []*ledger.Payload
	if m.diffReporter != nil {
		oldPayloadsForDiff = accountRegisters.Payloads()
	}

	// Create all the runtime components we need for the migration
	migrationRuntime, err := NewInterpreterMigrationRuntime(
		accountRegisters,
		m.chainID,
		m.interpreterMigrationRuntimeConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to create interpreter migration runtime: %w", err)
	}

	storage := migrationRuntime.Storage

	// Check storage health before migration, if enabled.
	var storageHealthErrorBefore error
	if m.checkStorageHealthBeforeMigration {

		storageHealthErrorBefore = checkStorageHealth(address, storage, accountRegisters, m.nWorkers)
		if storageHealthErrorBefore != nil {
			m.log.Warn().
				Err(storageHealthErrorBefore).
				Str("account", address.HexWithPrefix()).
				Msg("storage health check before migration failed")
		}
	}

	migration, err := migrations.NewStorageMigration(
		migrationRuntime.Interpreter,
		storage,
		m.name,
		address,
	)
	if err != nil {
		return fmt.Errorf("failed to create storage migration: %w", err)
	}

	reporter := newValueMigrationReporter(
		m.reporter,
		m.log,
		m.errorMessageHandler,
		m.verboseErrorOutput,
	)

	valueMigrations := m.valueMigrations(
		migrationRuntime.Interpreter,
		migrationRuntime.Accounts,
		reporter,
	)

	migration.Migrate(
		migration.NewValueMigrationsPathMigrator(
			reporter,
			valueMigrations...,
		),
	)

	err = migration.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// Check storage health after migration.
	// If the storage health check failed before the migration, we don't need to check it again.
	if storageHealthErrorBefore == nil {
		storageHealthErrorAfter := storage.CheckHealth()
		if storageHealthErrorAfter != nil {
			m.log.Err(storageHealthErrorAfter).
				Str("account", address.HexWithPrefix()).
				Msg("storage health check after migration failed")
		}
	}

	// finalize the transaction
	result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	// Merge the changes into the registers
	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(address): {},
	}

	err = registers.ApplyChanges(
		accountRegisters,
		result.WriteSet,
		expectedAddresses,
		m.log,
	)
	if err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	if m.diffReporter != nil {
		newPayloadsForDiff := accountRegisters.Payloads()

		accountDiffReporter := NewCadenceValueDiffReporter(
			address,
			m.chainID,
			m.diffReporter,
			m.logVerboseDiff,
			m.nWorkers,
		)

		owner := flow.AddressToRegisterOwner(flow.Address(address))

		oldRegistersForDiff, err := registers.NewAccountRegistersFromPayloads(owner, oldPayloadsForDiff)
		if err != nil {
			return fmt.Errorf("failed to create registers from old payloads: %w", err)
		}

		newRegistersForDiff, err := registers.NewAccountRegistersFromPayloads(owner, newPayloadsForDiff)
		if err != nil {
			return fmt.Errorf("failed to create registers from new payloads: %w", err)
		}

		accountDiffReporter.DiffStates(
			oldRegistersForDiff,
			newRegistersForDiff,
			AllStorageMapDomains,
		)
	}

	return nil
}

const cadenceValueMigrationReporterName = "cadence-value-migration"

// NewCadence1ValueMigration creates a new CadenceBaseMigration
// which runs some of the Cadence value migrations (static types, entitlements, strings)
func NewCadence1ValueMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	programs map[runtime.Location]*interpreter.Program,
	compositeTypeConverter statictypes.CompositeTypeConverterFunc,
	interfaceTypeConverter statictypes.InterfaceTypeConverterFunc,
	opts Options,
) *CadenceBaseMigration {

	var diffReporter reporters.ReportWriter
	if opts.DiffMigrations {
		diffReporter = rwf.ReportWriter("cadence-value-migration-diff")
	}

	return &CadenceBaseMigration{
		name:                              "cadence_value_migration",
		reporter:                          rwf.ReportWriter(cadenceValueMigrationReporterName),
		diffReporter:                      diffReporter,
		logVerboseDiff:                    opts.LogVerboseDiff,
		verboseErrorOutput:                opts.VerboseErrorOutput,
		checkStorageHealthBeforeMigration: opts.CheckStorageHealthBeforeMigration,
		valueMigrations: func(
			inter *interpreter.Interpreter,
			_ environment.Accounts,
			reporter *cadenceValueMigrationReporter,
		) []migrations.ValueMigration {
			return []migrations.ValueMigration{
				statictypes.NewStaticTypeMigration().
					WithCompositeTypeConverter(compositeTypeConverter).
					WithInterfaceTypeConverter(interfaceTypeConverter),
				entitlements.NewEntitlementsMigration(inter),
				type_keys.NewTypeKeyMigration(),
				string_normalization.NewStringNormalizingMigration(),
			}
		},
		errorMessageHandler: errorMessageHandler,
		programs:            programs,
		chainID:             opts.ChainID,
	}
}

// NewCadence1LinkValueMigration creates a new CadenceBaseMigration
// which migrates links to capability controllers.
// It populates the given map with the IDs of the capability controller it issues.
func NewCadence1LinkValueMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	programs map[runtime.Location]*interpreter.Program,
	capabilityMapping *capcons.CapabilityMapping,
	opts Options,
) *CadenceBaseMigration {
	var diffReporter reporters.ReportWriter
	if opts.DiffMigrations {
		diffReporter = rwf.ReportWriter("cadence-link-value-migration-diff")
	}

	return &CadenceBaseMigration{
		name:                              "cadence_link_value_migration",
		reporter:                          rwf.ReportWriter("cadence-link-value-migration"),
		diffReporter:                      diffReporter,
		logVerboseDiff:                    opts.LogVerboseDiff,
		verboseErrorOutput:                opts.VerboseErrorOutput,
		checkStorageHealthBeforeMigration: opts.CheckStorageHealthBeforeMigration,
		valueMigrations: func(
			_ *interpreter.Interpreter,
			accounts environment.Accounts,
			reporter *cadenceValueMigrationReporter,
		) []migrations.ValueMigration {
			idGenerator := environment.NewAccountLocalIDGenerator(
				tracing.NewMockTracerSpan(),
				util.NopMeter{},
				accounts,
			)
			return []migrations.ValueMigration{
				&capcons.LinkValueMigration{
					CapabilityMapping:  capabilityMapping,
					AccountIDGenerator: idGenerator,
					Reporter:           reporter,
				},
			}
		},
		errorMessageHandler: errorMessageHandler,
		programs:            programs,
		chainID:             opts.ChainID,
	}
}

// NewCadence1CapabilityValueMigration creates a new CadenceBaseMigration
// which migrates path capability values to ID capability values.
// It requires a map the IDs of the capability controllers,
// generated by the link value migration.
func NewCadence1CapabilityValueMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	programs map[runtime.Location]*interpreter.Program,
	capabilityMapping *capcons.CapabilityMapping,
	opts Options,
) *CadenceBaseMigration {
	var diffReporter reporters.ReportWriter
	if opts.DiffMigrations {
		diffReporter = rwf.ReportWriter("cadence-capability-value-migration-diff")
	}

	return &CadenceBaseMigration{
		name:                              "cadence_capability_value_migration",
		reporter:                          rwf.ReportWriter("cadence-capability-value-migration"),
		diffReporter:                      diffReporter,
		logVerboseDiff:                    opts.LogVerboseDiff,
		verboseErrorOutput:                opts.VerboseErrorOutput,
		checkStorageHealthBeforeMigration: opts.CheckStorageHealthBeforeMigration,
		valueMigrations: func(
			_ *interpreter.Interpreter,
			_ environment.Accounts,
			reporter *cadenceValueMigrationReporter,
		) []migrations.ValueMigration {
			return []migrations.ValueMigration{
				&capcons.CapabilityValueMigration{
					CapabilityMapping: capabilityMapping,
					Reporter:          reporter,
				},
			}
		},
		errorMessageHandler: errorMessageHandler,
		programs:            programs,
		chainID:             opts.ChainID,
	}
}

// errorMessageHandler formats error messages from errors.
// It only reports program loading errors once.
type errorMessageHandler struct {
	// common.Location -> struct{}
	reportedProgramLoadingErrors sync.Map
}

func (t *errorMessageHandler) FormatError(err error) (message string, showStack bool) {

	// Only report program loading errors once,
	// omit full error message for subsequent occurrences

	var programLoadingError environment.ProgramLoadingError
	if errors.As(err, &programLoadingError) {
		location := programLoadingError.Location
		_, ok := t.reportedProgramLoadingErrors.LoadOrStore(location, struct{}{})
		if ok {
			return "error getting program", false
		}

		return err.Error(), false
	}

	return err.Error(), true
}

// cadenceValueMigrationReporter is the reporter for cadence value migrations
type cadenceValueMigrationReporter struct {
	reportWriter        reporters.ReportWriter
	log                 zerolog.Logger
	errorMessageHandler *errorMessageHandler
	verboseErrorOutput  bool
}

var _ capcons.LinkMigrationReporter = &cadenceValueMigrationReporter{}
var _ capcons.CapabilityMigrationReporter = &cadenceValueMigrationReporter{}
var _ migrations.Reporter = &cadenceValueMigrationReporter{}

func newValueMigrationReporter(
	reportWriter reporters.ReportWriter,
	log zerolog.Logger,
	errorMessageHandler *errorMessageHandler,
	verboseErrorOutput bool,
) *cadenceValueMigrationReporter {
	return &cadenceValueMigrationReporter{
		reportWriter:        reportWriter,
		log:                 log,
		errorMessageHandler: errorMessageHandler,
		verboseErrorOutput:  verboseErrorOutput,
	}
}

func (t *cadenceValueMigrationReporter) Migrated(
	storageKey interpreter.StorageKey,
	storageMapKey interpreter.StorageMapKey,
	migration string,
) {
	t.reportWriter.Write(cadenceValueMigrationEntry{
		StorageKey:    storageKey,
		StorageMapKey: storageMapKey,
		Migration:     migration,
	})
}

func (t *cadenceValueMigrationReporter) Error(err error) {

	var migrationErr migrations.StorageMigrationError

	if !errors.As(err, &migrationErr) {
		panic(cadenceErrors.NewUnreachableError())
	}

	message, showStack := t.errorMessageHandler.FormatError(migrationErr.Err)

	storageKey := migrationErr.StorageKey
	storageMapKey := migrationErr.StorageMapKey
	migration := migrationErr.Migration

	if showStack && len(migrationErr.Stack) > 0 {
		message = fmt.Sprintf("%s\n%s", message, migrationErr.Stack)
	}

	if t.verboseErrorOutput {
		t.reportWriter.Write(cadenceValueMigrationFailureEntry{
			StorageKey:    storageKey,
			StorageMapKey: storageMapKey,
			Migration:     migration,
			Message:       message,
		})
	}
}

func (t *cadenceValueMigrationReporter) MigratedPathCapability(
	accountAddress common.Address,
	addressPath interpreter.AddressPath,
	borrowType *interpreter.ReferenceStaticType,
) {
	t.reportWriter.Write(capabilityMigrationEntry{
		AccountAddress: accountAddress,
		AddressPath:    addressPath,
		BorrowType:     borrowType,
	})
}

func (t *cadenceValueMigrationReporter) MissingCapabilityID(
	accountAddress common.Address,
	addressPath interpreter.AddressPath,
) {
	t.reportWriter.Write(capabilityMissingCapabilityIDEntry{
		AccountAddress: accountAddress,
		AddressPath:    addressPath,
	})
}

func (t *cadenceValueMigrationReporter) MigratedLink(
	accountAddressPath interpreter.AddressPath,
	capabilityID interpreter.UInt64Value,
) {
	t.reportWriter.Write(linkMigrationEntry{
		AccountAddressPath: accountAddressPath,
		CapabilityID:       uint64(capabilityID),
	})
}

func (t *cadenceValueMigrationReporter) CyclicLink(err capcons.CyclicLinkError) {
	t.reportWriter.Write(err)
}

func (t *cadenceValueMigrationReporter) MissingTarget(accountAddressPath interpreter.AddressPath) {
	t.reportWriter.Write(linkMissingTargetEntry{
		AddressPath: accountAddressPath,
	})
}

func (t *cadenceValueMigrationReporter) DictionaryKeyConflict(accountAddressPath interpreter.AddressPath) {
	t.reportWriter.Write(dictionaryKeyConflictEntry{
		AddressPath: accountAddressPath,
	})
}

type valueMigrationReportEntry interface {
	accountAddress() common.Address
}

// cadenceValueMigrationReportEntry

type cadenceValueMigrationEntry struct {
	StorageKey    interpreter.StorageKey
	StorageMapKey interpreter.StorageMapKey
	Migration     string
}

var _ valueMigrationReportEntry = cadenceValueMigrationEntry{}

func (e cadenceValueMigrationEntry) accountAddress() common.Address {
	return e.StorageKey.Address
}

var _ json.Marshaler = cadenceValueMigrationEntry{}

func (e cadenceValueMigrationEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		StorageDomain  string `json:"domain"`
		Key            string `json:"key"`
		Migration      string `json:"migration"`
	}{
		Kind:           "cadence-value-migration-success",
		AccountAddress: e.StorageKey.Address.HexWithPrefix(),
		StorageDomain:  e.StorageKey.Key,
		Key:            fmt.Sprintf("%s", e.StorageMapKey),
		Migration:      e.Migration,
	})
}

// cadenceValueMigrationFailureEntry

type cadenceValueMigrationFailureEntry struct {
	StorageKey    interpreter.StorageKey
	StorageMapKey interpreter.StorageMapKey
	Migration     string
	Message       string
}

var _ valueMigrationReportEntry = cadenceValueMigrationFailureEntry{}

func (e cadenceValueMigrationFailureEntry) accountAddress() common.Address {
	return e.StorageKey.Address
}

var _ json.Marshaler = cadenceValueMigrationFailureEntry{}

func (e cadenceValueMigrationFailureEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		StorageDomain  string `json:"domain"`
		Key            string `json:"key"`
		Migration      string `json:"migration"`
		Message        string `json:"message"`
	}{
		Kind:           "cadence-value-migration-failure",
		AccountAddress: e.StorageKey.Address.HexWithPrefix(),
		StorageDomain:  e.StorageKey.Key,
		Key:            fmt.Sprintf("%s", e.StorageMapKey),
		Migration:      e.Migration,
		Message:        e.Message,
	})
}

// linkMigrationEntry

type linkMigrationEntry struct {
	AccountAddressPath interpreter.AddressPath
	CapabilityID       uint64
}

var _ valueMigrationReportEntry = linkMigrationEntry{}

func (e linkMigrationEntry) accountAddress() common.Address {
	return e.AccountAddressPath.Address
}

var _ json.Marshaler = linkMigrationEntry{}

func (e linkMigrationEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		Path           string `json:"path"`
		CapabilityID   uint64 `json:"capability_id"`
	}{
		Kind:           "link-migration-success",
		AccountAddress: e.AccountAddressPath.Address.HexWithPrefix(),
		Path:           e.AccountAddressPath.Path.String(),
		CapabilityID:   e.CapabilityID,
	})
}

// capabilityMigrationEntry

type capabilityMigrationEntry struct {
	AccountAddress common.Address
	AddressPath    interpreter.AddressPath
	BorrowType     *interpreter.ReferenceStaticType
}

var _ valueMigrationReportEntry = capabilityMigrationEntry{}

func (e capabilityMigrationEntry) accountAddress() common.Address {
	return e.AccountAddress
}

var _ json.Marshaler = capabilityMigrationEntry{}

func (e capabilityMigrationEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		Address        string `json:"address"`
		Path           string `json:"path"`
		BorrowType     string `json:"borrow_type"`
	}{
		Kind:           "capability-migration-success",
		AccountAddress: e.AccountAddress.HexWithPrefix(),
		Address:        e.AddressPath.Address.HexWithPrefix(),
		Path:           e.AddressPath.Path.String(),
		BorrowType:     string(e.BorrowType.ID()),
	})
}

// capabilityMissingCapabilityIDEntry

type capabilityMissingCapabilityIDEntry struct {
	AccountAddress common.Address
	AddressPath    interpreter.AddressPath
}

var _ valueMigrationReportEntry = capabilityMissingCapabilityIDEntry{}

func (e capabilityMissingCapabilityIDEntry) accountAddress() common.Address {
	return e.AccountAddress
}

var _ json.Marshaler = capabilityMissingCapabilityIDEntry{}

func (e capabilityMissingCapabilityIDEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		Address        string `json:"address"`
		Path           string `json:"path"`
	}{
		Kind:           "capability-missing-capability-id",
		AccountAddress: e.AccountAddress.HexWithPrefix(),
		Address:        e.AddressPath.Address.HexWithPrefix(),
		Path:           e.AddressPath.Path.String(),
	})
}

// linkMissingTargetEntry

type linkMissingTargetEntry struct {
	AddressPath interpreter.AddressPath
}

var _ valueMigrationReportEntry = linkMissingTargetEntry{}

func (e linkMissingTargetEntry) accountAddress() common.Address {
	return e.AddressPath.Address
}

var _ json.Marshaler = linkMissingTargetEntry{}

func (e linkMissingTargetEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		Path           string `json:"path"`
	}{
		Kind:           "link-missing-target",
		AccountAddress: e.AddressPath.Address.HexWithPrefix(),
		Path:           e.AddressPath.Path.String(),
	})
}

// dictionaryKeyConflictEntry

type dictionaryKeyConflictEntry struct {
	AddressPath interpreter.AddressPath
}

var _ json.Marshaler = dictionaryKeyConflictEntry{}

func (e dictionaryKeyConflictEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		Path           string `json:"path"`
	}{
		Kind:           "dictionary-key-conflict",
		AccountAddress: e.AddressPath.Address.HexWithPrefix(),
		Path:           e.AddressPath.Path.String(),
	})
}
