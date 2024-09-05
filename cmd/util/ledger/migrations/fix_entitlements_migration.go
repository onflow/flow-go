package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

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

// FixCapabilityControllerEntitlementsMigration

type FixCapabilityControllerEntitlementsMigrationReporter interface {
	MigratedCapabilityController(
		key interpreter.StorageKey,
		value *interpreter.StorageCapabilityControllerValue,
	)
}

type FixCapabilityControllerEntitlementsMigration struct {
	Reporter FixCapabilityControllerEntitlementsMigrationReporter
}

var _ migrations.ValueMigration = &FixCapabilityControllerEntitlementsMigration{}

func (*FixCapabilityControllerEntitlementsMigration) Name() string {
	return "FixCapabilityControllerEntitlementsMigration"
}

func (*FixCapabilityControllerEntitlementsMigration) Domains() map[string]struct{} {
	return nil
}

func (m *FixCapabilityControllerEntitlementsMigration) Migrate(
	storageKey interpreter.StorageKey,
	_ interpreter.StorageMapKey,
	value interpreter.Value,
	_ *interpreter.Interpreter,
	_ migrations.ValueMigrationPosition,
) (
	interpreter.Value,
	error,
) {
	if capability, ok := value.(*interpreter.StorageCapabilityControllerValue); ok {
		// TODO:
		m.Reporter.MigratedCapabilityController(storageKey, capability)
	}

	return nil, nil
}

func (*FixCapabilityControllerEntitlementsMigration) CanSkip(valueType interpreter.StaticType) bool {
	return CanSkipFixEntitlementsMigration(valueType)
}

// FixCapabilityEntitlementsMigration

type FixCapabilityEntitlementsMigrationReporter interface {
	MigratedCapability(
		key interpreter.StorageKey,
		value *interpreter.IDCapabilityValue,
	)
}

type FixCapabilityEntitlementsMigration struct {
	Reporter FixCapabilityEntitlementsMigrationReporter
}

var _ migrations.ValueMigration = &FixCapabilityEntitlementsMigration{}

func (*FixCapabilityEntitlementsMigration) Name() string {
	return "FixCapabilityEntitlementsMigration"
}

func (*FixCapabilityEntitlementsMigration) Domains() map[string]struct{} {
	return nil
}

func (m *FixCapabilityEntitlementsMigration) Migrate(
	storageKey interpreter.StorageKey,
	_ interpreter.StorageMapKey,
	value interpreter.Value,
	_ *interpreter.Interpreter,
	_ migrations.ValueMigrationPosition,
) (
	interpreter.Value,
	error,
) {
	if capability, ok := value.(*interpreter.IDCapabilityValue); ok {
		// TODO:
		m.Reporter.MigratedCapability(storageKey, capability)
	}

	return nil, nil
}

func (*FixCapabilityEntitlementsMigration) CanSkip(valueType interpreter.StaticType) bool {
	return CanSkipFixEntitlementsMigration(valueType)
}

func CanSkipFixEntitlementsMigration(valueType interpreter.StaticType) bool {
	switch valueType := valueType.(type) {
	case *interpreter.DictionaryStaticType:
		return CanSkipFixEntitlementsMigration(valueType.KeyType) &&
			CanSkipFixEntitlementsMigration(valueType.ValueType)

	case interpreter.ArrayStaticType:
		return CanSkipFixEntitlementsMigration(valueType.ElementType())

	case *interpreter.OptionalStaticType:
		return CanSkipFixEntitlementsMigration(valueType.Type)

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

type FixEntitlementsMigrationOptions struct {
	ChainID                           flow.ChainID
	NWorker                           int
	VerboseErrorOutput                bool
	LogVerboseDiff                    bool
	DiffMigrations                    bool
	CheckStorageHealthBeforeMigration bool
}

func NewFixCapabilityControllerEntitlementsMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	programs map[runtime.Location]*interpreter.Program,
	opts FixEntitlementsMigrationOptions,
) *CadenceBaseMigration {
	var diffReporter reporters.ReportWriter
	if opts.DiffMigrations {
		diffReporter = rwf.ReportWriter("fix-capability-controller-entitlements-migration-diff")
	}

	reporter := rwf.ReportWriter("fix-capability-controller-entitlements-migration")

	return &CadenceBaseMigration{
		name:                              "fix_capability_controller_entitlements_migration",
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
				&FixCapabilityControllerEntitlementsMigration{
					Reporter: &fixEntitlementsMigrationReporter{
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

func NewFixCapabilityEntitlementsMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	programs map[runtime.Location]*interpreter.Program,
	opts FixEntitlementsMigrationOptions,
) *CadenceBaseMigration {
	var diffReporter reporters.ReportWriter
	if opts.DiffMigrations {
		diffReporter = rwf.ReportWriter("fix-capability-entitlements-migration-diff")
	}

	reporter := rwf.ReportWriter("fix-capability-entitlements-migration")

	return &CadenceBaseMigration{
		name:                              "fix_capability_entitlements_migration",
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
				&FixCapabilityEntitlementsMigration{
					Reporter: &fixEntitlementsMigrationReporter{
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

type fixEntitlementsMigrationReporter struct {
	reportWriter        reporters.ReportWriter
	errorMessageHandler *errorMessageHandler
	verboseErrorOutput  bool
}

var _ FixCapabilityEntitlementsMigrationReporter = &fixEntitlementsMigrationReporter{}
var _ FixCapabilityControllerEntitlementsMigrationReporter = &fixEntitlementsMigrationReporter{}
var _ migrations.Reporter = &fixEntitlementsMigrationReporter{}

func (r *fixEntitlementsMigrationReporter) Migrated(
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

func (r *fixEntitlementsMigrationReporter) Error(err error) {

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

func (r *fixEntitlementsMigrationReporter) DictionaryKeyConflict(accountAddressPath interpreter.AddressPath) {
	r.reportWriter.Write(dictionaryKeyConflictEntry{
		AddressPath: accountAddressPath,
	})
}

func (r *fixEntitlementsMigrationReporter) MigratedCapability(
	key interpreter.StorageKey,
	value *interpreter.IDCapabilityValue,
) {
	// TODO:
}

func (r *fixEntitlementsMigrationReporter) MigratedCapabilityController(
	key interpreter.StorageKey,
	value *interpreter.StorageCapabilityControllerValue,
) {
	// TODO:
}

type AccountCapabilityControllerID struct {
	Address      common.Address
	CapabilityID uint64
}

// ReadLinkMigrationReport reads a link migration report from the given reader.
// The report is expected to be a JSON array of objects with the following structure:
//
// [
//
//	{"kind":"link-migration-success","account_address":"0x1","path":"/public/foo","capability_id":1},
//	{"kind":"link-migration-success","account_address":"0x2","path":"/private/bar","capability_id":2}
//
// ]
//
// The function returns a mapping from account capability controller IDs to paths.
func ReadLinkMigrationReport(reader io.Reader) (map[AccountCapabilityControllerID]string, error) {
	mapping := make(map[AccountCapabilityControllerID]string)

	dec := json.NewDecoder(reader)

	token, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}
	if token != json.Delim('[') {
		return nil, fmt.Errorf("expected start of array, got %s", token)
	}

	for dec.More() {
		var entry struct {
			Kind         string `json:"kind"`
			Address      string `json:"account_address"`
			Path         string `json:"path"`
			CapabilityID uint64 `json:"capability_id"`
		}
		err := dec.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("failed to decode entry: %w", err)
		}

		if entry.Kind != "link-migration-success" {
			continue
		}

		address, err := common.HexToAddress(entry.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to parse address: %w", err)
		}

		key := AccountCapabilityControllerID{
			Address:      address,
			CapabilityID: entry.CapabilityID,
		}
		mapping[key] = entry.Path
	}

	token, err = dec.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}
	if token != json.Delim(']') {
		return nil, fmt.Errorf("expected end of array, got %s", token)
	}

	return mapping, nil
}

func NewFixEntitlementsMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	opts FixEntitlementsMigrationOptions,
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
			Name: "fix-capability-controller-entitlements",
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]AccountBasedMigration{
					NewFixCapabilityControllerEntitlementsMigration(
						rwf,
						errorMessageHandler,
						programs,
						opts,
					),
				},
			),
		},
		{
			Name: "fix-capability-entitlements",
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]AccountBasedMigration{
					NewFixCapabilityEntitlementsMigration(
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
