package migrations

import (
	"context"
	"fmt"
	"io"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence/migrations"
	"github.com/onflow/cadence/migrations/account_type"
	"github.com/onflow/cadence/migrations/capcons"
	"github.com/onflow/cadence/migrations/string_normalization"
	"github.com/onflow/cadence/migrations/type_value"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

type CadenceValueMigrator struct {
	log      zerolog.Logger
	reporter reporters.ReportWriter
}

var _ AccountBasedMigration = (*CadenceValueMigrator)(nil)
var _ io.Closer = (*CadenceValueMigrator)(nil)

func NewCadenceValueMigrator(
	rwf reporters.ReportWriterFactory,
) *CadenceValueMigrator {
	return &CadenceValueMigrator{
		reporter: rwf.ReportWriter("cadence-value-migrator"),
	}
}

func (m *CadenceValueMigrator) Close() error {
	// Close the report writer so it flushes to file.
	m.reporter.Close()
	return nil
}

func (m *CadenceValueMigrator) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	m.log = log.With().Str("migration", "cadence-value-migration").Logger()
	return nil
}

func (m *CadenceValueMigrator) MigrateAccount(
	_ context.Context,
	address common.Address,
	oldPayloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	// Create all the runtime components we need for the migration
	migrationRuntime, err := newMigratorRuntime(address, oldPayloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
	}

	migration := migrations.NewStorageMigration(
		migrationRuntime.Interpreter,
		migrationRuntime.Storage,
	)

	capabilityIDs := map[interpreter.AddressPath]interpreter.UInt64Value{}

	reporter := newValueMigrationReporter(m.reporter)

	m.log.Info().Msg("Migrating cadence values")

	migration.Migrate(
		&migrations.AddressSliceIterator{
			Addresses: []common.Address{
				address,
			},
		},
		migration.NewValueMigrationsPathMigrator(
			reporter,
			&capcons.CapabilityValueMigration{
				CapabilityIDs: capabilityIDs,
				Reporter:      reporter,
			},
			&capcons.LinkValueMigration{
				CapabilityIDs:      capabilityIDs,
				AccountIDGenerator: &AccountIDGenerator{},
				Reporter:           reporter,
			},
			string_normalization.NewStringNormalizingMigration(),
			account_type.NewAccountTypeMigration(),
			type_value.NewTypeValueMigration(),
		),
	)

	m.log.Info().Msg("Committing changes")
	migration.Commit()

	// finalize the transaction
	result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	// Merge the changes to the original payloads.
	return m.mergeRegisterChanges(migrationRuntime, result.WriteSet)
}

func (m *CadenceValueMigrator) mergeRegisterChanges(
	mr *migratorRuntime,
	changes map[flow.RegisterID]flow.RegisterValue,
) ([]*ledger.Payload, error) {

	originalPayloads := mr.Snapshot.Payloads
	newPayloads := make([]*ledger.Payload, 0, len(originalPayloads))

	for id, value := range originalPayloads {
		if len(value.Value()) == 0 {
			// This is strange, but we don't want to add empty values. Log it.
			m.log.Warn().Msgf("empty value for key %s", id)
			continue
		}

		// If the payload had changed, then use the updated payload.
		if updatedPayload, contains := changes[id]; contains {
			key := convert.RegisterIDToLedgerKey(id)
			newPayloads = append(newPayloads, ledger.NewPayload(key, updatedPayload))
		} else {
			newPayloads = append(newPayloads, value)
		}
	}

	return newPayloads, nil
}

// AccountIDGenerator for the link-value migration
type AccountIDGenerator struct {
	ids map[common.Address]uint64
}

func (g *AccountIDGenerator) GenerateAccountID(address common.Address) (uint64, error) {
	if g.ids == nil {
		g.ids = make(map[common.Address]uint64)
	}
	g.ids[address]++
	return g.ids[address], nil
}

// cadenceValueMigrationReporter is the reporter for cadence value migrations
type cadenceValueMigrationReporter struct {
	rw reporters.ReportWriter
}

var _ capcons.LinkMigrationReporter = &cadenceValueMigrationReporter{}
var _ capcons.CapabilityMigrationReporter = &cadenceValueMigrationReporter{}

func newValueMigrationReporter(rw reporters.ReportWriter) *cadenceValueMigrationReporter {
	return &cadenceValueMigrationReporter{
		rw: rw,
	}
}

func (t *cadenceValueMigrationReporter) Report(
	addressPath interpreter.AddressPath,
	migration string,
) {
	t.rw.Write(cadenceValueMigrationReportEntry{
		Address:   addressPath,
		Migration: migration,
	})
}

func (t *cadenceValueMigrationReporter) MigratedPathCapability(
	accountAddress common.Address,
	addressPath interpreter.AddressPath,
	borrowType *interpreter.ReferenceStaticType,
) {
	t.rw.Write(capConsPathCapabilityMigration{
		AccountAddress: accountAddress,
		AddressPath:    addressPath,
		BorrowType:     borrowType,
	})
}

func (t *cadenceValueMigrationReporter) MissingCapabilityID(
	accountAddress common.Address,
	addressPath interpreter.AddressPath,
) {
	t.rw.Write(capConsMissingCapabilityID{
		AccountAddress: accountAddress,
		AddressPath:    addressPath,
	})
}

func (t *cadenceValueMigrationReporter) MigratedLink(
	accountAddressPath interpreter.AddressPath,
	capabilityID interpreter.UInt64Value,
) {
	t.rw.Write(capConsLinkMigration{
		AccountAddressPath: accountAddressPath,
		CapabilityID:       capabilityID,
	})
}

func (t *cadenceValueMigrationReporter) CyclicLink(err capcons.CyclicLinkError) {
	t.rw.Write(err)
}

func (t *cadenceValueMigrationReporter) MissingTarget(accountAddressPath interpreter.AddressPath) {
	t.rw.Write(capConsMissingTarget{
		AddressPath: accountAddressPath,
	})
}

type cadenceValueMigrationReportEntry struct {
	Address   interpreter.AddressPath `json:"address"`
	Migration string                  `json:"migration"`
}

type capConsLinkMigration struct {
	AccountAddressPath interpreter.AddressPath `json:"address"`
	CapabilityID       interpreter.UInt64Value `json:"capabilityID"`
}

type capConsPathCapabilityMigration struct {
	AccountAddress common.Address                   `json:"address"`
	AddressPath    interpreter.AddressPath          `json:"addressPath"`
	BorrowType     *interpreter.ReferenceStaticType `json:"borrowType"`
}

type capConsMissingCapabilityID struct {
	AccountAddress common.Address          `json:"address"`
	AddressPath    interpreter.AddressPath `json:"addressPath"`
}

type capConsMissingTarget struct {
	AddressPath interpreter.AddressPath `json:"addressPath"`
}
