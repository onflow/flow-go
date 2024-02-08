package migrations

import (
	"context"
	"fmt"
	"io"

	"github.com/onflow/cadence/migrations/statictypes"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/migrations"
	"github.com/onflow/cadence/migrations/capcons"
	"github.com/onflow/cadence/migrations/entitlements"
	"github.com/onflow/cadence/migrations/string_normalization"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type CadenceBaseMigrator struct {
	name            string
	log             zerolog.Logger
	reporter        reporters.ReportWriter
	valueMigrations func(
		inter *interpreter.Interpreter,
		accounts environment.Accounts,
		reporter *cadenceValueMigrationReporter,
	) []migrations.ValueMigration
	runtimeInterfaceConfig util.RuntimeInterfaceConfig
}

var _ AccountBasedMigration = (*CadenceBaseMigrator)(nil)
var _ io.Closer = (*CadenceBaseMigrator)(nil)

func (m *CadenceBaseMigrator) Close() error {
	// Close the report writer so it flushes to file.
	m.reporter.Close()
	return nil
}

func (m *CadenceBaseMigrator) InitMigration(
	log zerolog.Logger,
	allPayloads []*ledger.Payload,
	_ int,
) error {
	m.log = log.With().Str("migration", m.name).Logger()

	// The MigrateAccount function is only given the payloads for the account to be migrated.
	// However, the migration needs to be able to get the code for contracts of any account.

	fullPayloadSnapshot, err := util.NewPayloadSnapshot(allPayloads)
	if err != nil {
		return err
	}

	m.runtimeInterfaceConfig = util.RuntimeInterfaceConfig{

		GetContractCodeFunc: func(location runtime.Location) ([]byte, error) {
			addressLocation, ok := location.(common.AddressLocation)
			if !ok {
				return nil, nil
			}
			contractRegisterID := flow.ContractRegisterID(
				flow.Address(addressLocation.Address),
				addressLocation.Name,
			)
			contract, err := fullPayloadSnapshot.Get(contractRegisterID)
			if err != nil {
				return nil, fmt.Errorf("failed to get contract code: %w", err)
			}
			return contract, nil
		},
	}

	return nil
}

func (m *CadenceBaseMigrator) MigrateAccount(
	_ context.Context,
	address common.Address,
	oldPayloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	// Create all the runtime components we need for the migration

	migrationRuntime, err := newMigratorRuntime(
		address,
		oldPayloads,
		m.runtimeInterfaceConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
	}

	migration := migrations.NewStorageMigration(
		migrationRuntime.Interpreter,
		migrationRuntime.Storage,
	)

	reporter := newValueMigrationReporter(m.reporter, m.log)

	m.log.Info().Msg("Migrating cadence values")

	migration.Migrate(
		&migrations.AddressSliceIterator{
			Addresses: []common.Address{
				address,
			},
		},
		migration.NewValueMigrationsPathMigrator(
			reporter,
			m.valueMigrations(migrationRuntime.Interpreter, migrationRuntime.Accounts, reporter)...,
		),
	)

	m.log.Info().Msg("Committing changes")
	err = migration.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit changes: %w", err)
	}

	// finalize the transaction
	result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	// Merge the changes to the original payloads.
	return MergeRegisterChanges(
		migrationRuntime.Snapshot.Payloads,
		result.WriteSet,
		m.log,
	)
}

// NewCadence1ValueMigrator creates a new CadenceBaseMigrator
// which runs some of the Cadence value migrations (static types, entitlements, strings)
func NewCadence1ValueMigrator(
	rwf reporters.ReportWriterFactory,
	compositeTypeConverter statictypes.CompositeTypeConverterFunc,
	interfaceTypeConverter statictypes.InterfaceTypeConverterFunc,
) *CadenceBaseMigrator {
	return &CadenceBaseMigrator{
		name:     "cadence-value-migration",
		reporter: rwf.ReportWriter("cadence-value-migrator"),
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
				string_normalization.NewStringNormalizingMigration(),
			}
		},
	}
}

// NewCadence1LinkValueMigrator creates a new CadenceBaseMigrator
// which migrates links to capability controllers.
// It populates the given map with the IDs of the capability controller it issues.
func NewCadence1LinkValueMigrator(
	rwf reporters.ReportWriterFactory,
	capabilityIDs map[interpreter.AddressPath]interpreter.UInt64Value,
) *CadenceBaseMigrator {
	return &CadenceBaseMigrator{
		name:     "cadence-link-value-migration",
		reporter: rwf.ReportWriter("cadence-link-value-migrator"),
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
					CapabilityIDs:      capabilityIDs,
					AccountIDGenerator: idGenerator,
					Reporter:           reporter,
				},
			}
		},
	}
}

// NewCadence1CapabilityValueMigrator creates a new CadenceBaseMigrator
// which migrates path capability values to ID capability values.
// It requires a map the IDs of the capability controllers,
// generated by the link value migration.
func NewCadence1CapabilityValueMigrator(
	rwf reporters.ReportWriterFactory,
	capabilityIDs map[interpreter.AddressPath]interpreter.UInt64Value,
) *CadenceBaseMigrator {
	return &CadenceBaseMigrator{
		name:     "cadence-capability-value-migration",
		reporter: rwf.ReportWriter("cadence-capability-value-migrator"),
		valueMigrations: func(
			_ *interpreter.Interpreter,
			_ environment.Accounts,
			reporter *cadenceValueMigrationReporter,
		) []migrations.ValueMigration {
			return []migrations.ValueMigration{
				&capcons.CapabilityValueMigration{
					CapabilityIDs: capabilityIDs,
					Reporter:      reporter,
				},
			}
		},
	}
}

// cadenceValueMigrationReporter is the reporter for cadence value migrations
type cadenceValueMigrationReporter struct {
	rw  reporters.ReportWriter
	log zerolog.Logger
}

var _ capcons.LinkMigrationReporter = &cadenceValueMigrationReporter{}
var _ capcons.CapabilityMigrationReporter = &cadenceValueMigrationReporter{}
var _ migrations.Reporter = &cadenceValueMigrationReporter{}

func newValueMigrationReporter(rw reporters.ReportWriter, log zerolog.Logger) *cadenceValueMigrationReporter {
	return &cadenceValueMigrationReporter{
		rw:  rw,
		log: log,
	}
}

func (t *cadenceValueMigrationReporter) Migrated(
	storageKey interpreter.StorageKey,
	storageMapKey interpreter.StorageMapKey,
	migration string,
) {
	t.rw.Write(cadenceValueMigrationReportEntry{
		StorageKey:    storageKey,
		StorageMapKey: storageMapKey,
		Migration:     migration,
	})
}

func (t *cadenceValueMigrationReporter) Error(
	storageKey interpreter.StorageKey,
	storageMapKey interpreter.StorageMapKey,
	migration string,
	err error,
) {
	t.log.Error().Msgf(
		"failed to run %s in account %s, domain %s, key %s: %s",
		migration,
		storageKey.Address,
		storageKey.Key,
		storageMapKey,
		err,
	)
}

func (t *cadenceValueMigrationReporter) MigratedPathCapability(
	accountAddress common.Address,
	addressPath interpreter.AddressPath,
	borrowType *interpreter.ReferenceStaticType,
) {
	t.rw.Write(capConsPathCapabilityMigrationEntry{
		AccountAddress: accountAddress,
		AddressPath:    addressPath,
		BorrowType:     borrowType,
	})
}

func (t *cadenceValueMigrationReporter) MissingCapabilityID(
	accountAddress common.Address,
	addressPath interpreter.AddressPath,
) {
	t.rw.Write(capConsMissingCapabilityIDEntry{
		AccountAddress: accountAddress,
		AddressPath:    addressPath,
	})
}

func (t *cadenceValueMigrationReporter) MigratedLink(
	accountAddressPath interpreter.AddressPath,
	capabilityID interpreter.UInt64Value,
) {
	t.rw.Write(capConsLinkMigrationEntry{
		AccountAddressPath: accountAddressPath,
		CapabilityID:       capabilityID,
	})
}

func (t *cadenceValueMigrationReporter) CyclicLink(err capcons.CyclicLinkError) {
	t.rw.Write(err)
}

func (t *cadenceValueMigrationReporter) MissingTarget(accountAddressPath interpreter.AddressPath) {
	t.rw.Write(capConsMissingTargetEntry{
		AddressPath: accountAddressPath,
	})
}

type reportEntry interface {
	accountAddress() common.Address
}

type cadenceValueMigrationReportEntry struct {
	StorageKey    interpreter.StorageKey    `json:"storageKey"`
	StorageMapKey interpreter.StorageMapKey `json:"storageMapKey"`
	Migration     string                    `json:"migration"`
}

var _ reportEntry = cadenceValueMigrationReportEntry{}

func (e cadenceValueMigrationReportEntry) accountAddress() common.Address {
	return e.StorageKey.Address
}

type capConsLinkMigrationEntry struct {
	AccountAddressPath interpreter.AddressPath `json:"address"`
	CapabilityID       interpreter.UInt64Value `json:"capabilityID"`
}

var _ reportEntry = capConsLinkMigrationEntry{}

func (e capConsLinkMigrationEntry) accountAddress() common.Address {
	return e.AccountAddressPath.Address
}

type capConsPathCapabilityMigrationEntry struct {
	AccountAddress common.Address                   `json:"address"`
	AddressPath    interpreter.AddressPath          `json:"addressPath"`
	BorrowType     *interpreter.ReferenceStaticType `json:"borrowType"`
}

var _ reportEntry = capConsPathCapabilityMigrationEntry{}

func (e capConsPathCapabilityMigrationEntry) accountAddress() common.Address {
	return e.AccountAddress
}

type capConsMissingCapabilityIDEntry struct {
	AccountAddress common.Address          `json:"address"`
	AddressPath    interpreter.AddressPath `json:"addressPath"`
}

var _ reportEntry = capConsMissingCapabilityIDEntry{}

type capConsMissingTargetEntry struct {
	AddressPath interpreter.AddressPath `json:"addressPath"`
}

func (e capConsMissingTargetEntry) accountAddress() common.Address {
	return e.AddressPath.Address
}

var _ reportEntry = capConsMissingTargetEntry{}

func (e capConsMissingCapabilityIDEntry) accountAddress() common.Address {
	return e.AccountAddress
}
