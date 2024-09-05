package migrations

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence/migrations"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	cadenceErrors "github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

type PublicEntitlementsMigrationReporter interface {
	MigratedCapability(key interpreter.StorageKey, value *interpreter.IDCapabilityValue)
	MigratedCapabilityController(key interpreter.StorageKey, value *interpreter.StorageCapabilityControllerValue)
}

type PublicEntitlementsMigration struct {
	Reporter PublicEntitlementsMigrationReporter
}

var _ migrations.ValueMigration = &PublicEntitlementsMigration{}

func (*PublicEntitlementsMigration) Name() string {
	return "PublicEntitlementsMigration"
}

func (*PublicEntitlementsMigration) Domains() map[string]struct{} {
	return nil
}

func (m *PublicEntitlementsMigration) Migrate(
	storageKey interpreter.StorageKey,
	_ interpreter.StorageMapKey,
	value interpreter.Value,
	_ *interpreter.Interpreter,
	_ migrations.ValueMigrationPosition,
) (
	interpreter.Value,
	error,
) {
	switch value := value.(type) {
	case *interpreter.IDCapabilityValue:
		// TODO:
		m.Reporter.MigratedCapability(storageKey, value)

	case *interpreter.StorageCapabilityControllerValue:
		// TODO:
		m.Reporter.MigratedCapabilityController(storageKey, value)
	}

	return nil, nil
}

func (*PublicEntitlementsMigration) CanSkip(valueType interpreter.StaticType) bool {
	return CanSkipPublicEntitlementsMigration(valueType)
}

func CanSkipPublicEntitlementsMigration(valueType interpreter.StaticType) bool {
	switch valueType := valueType.(type) {
	case *interpreter.DictionaryStaticType:
		return CanSkipPublicEntitlementsMigration(valueType.KeyType) &&
			CanSkipPublicEntitlementsMigration(valueType.ValueType)

	case interpreter.ArrayStaticType:
		return CanSkipPublicEntitlementsMigration(valueType.ElementType())

	case *interpreter.OptionalStaticType:
		return CanSkipPublicEntitlementsMigration(valueType.Type)

	case *interpreter.CapabilityStaticType:
		return false

	case interpreter.PrimitiveStaticType:

		switch valueType {
		case interpreter.PrimitiveStaticTypeCapability,
			interpreter.PrimitiveStaticTypeStorageCapabilityController:
			return false

		case interpreter.PrimitiveStaticTypeBool,
			interpreter.PrimitiveStaticTypeVoid,
			interpreter.PrimitiveStaticTypeAddress,
			interpreter.PrimitiveStaticTypeMetaType,
			interpreter.PrimitiveStaticTypeBlock,
			interpreter.PrimitiveStaticTypeString,
			interpreter.PrimitiveStaticTypeCharacter:

			return true
		}

		if !valueType.IsDeprecated() { //nolint:staticcheck
			semaType := valueType.SemaType()

			if sema.IsSubType(semaType, sema.NumberType) ||
				sema.IsSubType(semaType, sema.PathType) {

				return true
			}
		}
	}

	return false
}

type PublicEntitlementsMigrationOptions struct {
	ChainID                           flow.ChainID
	NWorker                           int
	VerboseErrorOutput                bool
	LogVerboseDiff                    bool
	DiffMigrations                    bool
	CheckStorageHealthBeforeMigration bool
}

func NewPublicEntitlementsValueMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	programs map[runtime.Location]*interpreter.Program,
	opts PublicEntitlementsMigrationOptions,
) *CadenceBaseMigration {
	var diffReporter reporters.ReportWriter
	if opts.DiffMigrations {
		diffReporter = rwf.ReportWriter("public-entitlements-migration-diff")
	}

	reporter := rwf.ReportWriter("public-entitlements-migration")

	return &CadenceBaseMigration{
		name:                              "public_entitlements_migration",
		reporter:                          reporter,
		diffReporter:                      diffReporter,
		logVerboseDiff:                    opts.LogVerboseDiff,
		verboseErrorOutput:                opts.VerboseErrorOutput,
		checkStorageHealthBeforeMigration: opts.CheckStorageHealthBeforeMigration,
		valueMigrations: func(
			_ *interpreter.Interpreter,
			_ environment.Accounts,
			_ *cadenceValueMigrationReporter,
		) []migrations.ValueMigration {

			return []migrations.ValueMigration{
				&PublicEntitlementsMigration{
					Reporter: &publicEntitlementsMigrationReporter{
						reportWriter:        reporter,
						errorMessageHandler: errorMessageHandler,
						verboseErrorOutput:  opts.VerboseErrorOutput,
					},
				},
			}
		},
		errorMessageHandler: errorMessageHandler,
		programs:            programs,
		chainID:             opts.ChainID,
	}
}

type publicEntitlementsMigrationReporter struct {
	reportWriter        reporters.ReportWriter
	errorMessageHandler *errorMessageHandler
	verboseErrorOutput  bool
}

var _ PublicEntitlementsMigrationReporter = &publicEntitlementsMigrationReporter{}
var _ migrations.Reporter = &publicEntitlementsMigrationReporter{}

func (r *publicEntitlementsMigrationReporter) Migrated(
	storageKey interpreter.StorageKey,
	storageMapKey interpreter.StorageMapKey,
	migration string,
) {
	r.reportWriter.Write(cadenceValueMigrationEntry{
		StorageKey:    storageKey,
		StorageMapKey: storageMapKey,
		Migration:     migration,
	})
}

func (r *publicEntitlementsMigrationReporter) Error(err error) {

	var migrationErr migrations.StorageMigrationError

	if !errors.As(err, &migrationErr) {
		panic(cadenceErrors.NewUnreachableError())
	}

	message, showStack := r.errorMessageHandler.FormatError(migrationErr.Err)

	storageKey := migrationErr.StorageKey
	storageMapKey := migrationErr.StorageMapKey
	migration := migrationErr.Migration

	if showStack && len(migrationErr.Stack) > 0 {
		message = fmt.Sprintf("%s\n%s", message, migrationErr.Stack)
	}

	if r.verboseErrorOutput {
		r.reportWriter.Write(cadenceValueMigrationFailureEntry{
			StorageKey:    storageKey,
			StorageMapKey: storageMapKey,
			Migration:     migration,
			Message:       message,
		})
	}
}

func (r *publicEntitlementsMigrationReporter) DictionaryKeyConflict(accountAddressPath interpreter.AddressPath) {
	r.reportWriter.Write(dictionaryKeyConflictEntry{
		AddressPath: accountAddressPath,
	})
}

func (r *publicEntitlementsMigrationReporter) MigratedCapability(
	key interpreter.StorageKey,
	value *interpreter.IDCapabilityValue,
) {
	// TODO:
}

func (r *publicEntitlementsMigrationReporter) MigratedCapabilityController(
	key interpreter.StorageKey,
	value *interpreter.StorageCapabilityControllerValue,
) {
	// TODO:
}

func NewPublicEntitlementsMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	opts PublicEntitlementsMigrationOptions,
) []NamedMigration {

	errorMessageHandler := &errorMessageHandler{}

	// The value migrations are run as account-based migrations,
	// i.e. the migrations are only given the payloads for the account to be migrated.
	// However, the migrations need to be able to get the code for contracts of any account.
	//
	// To achieve this, the contracts are extracted from the payloads once,
	// before the value migrations are run.

	programs := make(map[common.Location]*interpreter.Program, 1000)

	return []NamedMigration{
		{
			Name: "check-contracts",
			Migrate: NewContractCheckingMigration(
				log,
				rwf,
				opts.ChainID,
				opts.VerboseErrorOutput,
				// TODO: what are the important locations?
				map[common.AddressLocation]struct{}{},
				programs,
			),
		},
		{
			Name: "fix-public-entitlements",
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]AccountBasedMigration{
					NewPublicEntitlementsValueMigration(
						rwf,
						errorMessageHandler,
						programs,
						opts,
					),
				},
			),
		},
	}
}
