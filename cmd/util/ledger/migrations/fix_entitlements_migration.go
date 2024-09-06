package migrations

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/onflow/cadence/migrations"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	cadenceErrors "github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

type AccountCapabilityControllerID struct {
	Address      common.Address
	CapabilityID uint64
}

// FixEntitlementsMigration

type FixEntitlementsMigrationReporter interface {
	MigratedCapability(
		storageKey interpreter.StorageKey,
		capabilityAddress common.Address,
		capabilityID uint64,
		newAuthorization interpreter.Authorization,
	)
	MigratedCapabilityController(
		storageKey interpreter.StorageKey,
		capabilityID uint64,
		newAuthorization interpreter.Authorization,
	)
}

type FixEntitlementsMigration struct {
	Reporter          FixEntitlementsMigrationReporter
	NewAuthorizations map[AccountCapabilityControllerID]interpreter.Authorization
}

var _ migrations.ValueMigration = &FixEntitlementsMigration{}

func (*FixEntitlementsMigration) Name() string {
	return "FixEntitlementsMigration"
}

func (*FixEntitlementsMigration) Domains() map[string]struct{} {
	return nil
}

func (m *FixEntitlementsMigration) Migrate(
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
		capabilityAddress := common.Address(value.Address())
		capabilityID := uint64(value.ID)

		newAuthorization := m.NewAuthorizations[AccountCapabilityControllerID{
			Address:      capabilityAddress,
			CapabilityID: capabilityID,
		}]
		if newAuthorization == nil {
			// Nothing to fix for this capability
			return nil, nil
		}

		borrowType := value.BorrowType
		if borrowType == nil {
			log.Warn().Msgf(
				"missing borrow type for capability with target %s#%d",
				capabilityAddress.HexWithPrefix(),
				capabilityID,
			)
		}

		borrowReferenceType, ok := borrowType.(*interpreter.ReferenceStaticType)
		if !ok {
			log.Warn().Msgf(
				"invalid non-reference borrow type for capability with target %s#%d: %s",
				capabilityAddress.HexWithPrefix(),
				capabilityID,
				borrowType,
			)
			return nil, nil
		}

		borrowReferenceType.Authorization = newAuthorization
		value.BorrowType = borrowReferenceType

		m.Reporter.MigratedCapability(
			storageKey,
			capabilityAddress,
			capabilityID,
			newAuthorization,
		)

		return value, nil

	case *interpreter.StorageCapabilityControllerValue:
		// The capability controller's address is implicitly
		// the address of the account in which it is stored
		capabilityAddress := storageKey.Address
		capabilityID := uint64(value.CapabilityID)

		newAuthorization := m.NewAuthorizations[AccountCapabilityControllerID{
			Address:      capabilityAddress,
			CapabilityID: capabilityID,
		}]
		if newAuthorization == nil {
			// Nothing to fix for this capability controller
			return nil, nil
		}

		value.BorrowType.Authorization = newAuthorization

		m.Reporter.MigratedCapabilityController(
			storageKey,
			capabilityID,
			newAuthorization,
		)

		return value, nil
	}

	return nil, nil
}

func (*FixEntitlementsMigration) CanSkip(valueType interpreter.StaticType) bool {
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

const fixEntitlementsMigrationReporterName = "fix-entitlements-migration"

func NewFixEntitlementsMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	programs map[runtime.Location]*interpreter.Program,
	newAuthorizations map[AccountCapabilityControllerID]interpreter.Authorization,
	opts FixEntitlementsMigrationOptions,
) *CadenceBaseMigration {
	var diffReporter reporters.ReportWriter
	if opts.DiffMigrations {
		diffReporter = rwf.ReportWriter("fix-entitlements-migration-diff")
	}

	reporter := rwf.ReportWriter(fixEntitlementsMigrationReporterName)

	return &CadenceBaseMigration{
		name:                              "fix_entitlements_migration",
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
				&FixEntitlementsMigration{
					NewAuthorizations: newAuthorizations,
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

var _ FixEntitlementsMigrationReporter = &fixEntitlementsMigrationReporter{}
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

func (r *fixEntitlementsMigrationReporter) MigratedCapabilityController(
	storageKey interpreter.StorageKey,
	capabilityID uint64,
	newAuthorization interpreter.Authorization,
) {
	r.reportWriter.Write(capabilityControllerEntitlementsFixedEntry{
		StorageKey:       storageKey,
		CapabilityID:     capabilityID,
		NewAuthorization: newAuthorization,
	})
}

func (r *fixEntitlementsMigrationReporter) MigratedCapability(
	storageKey interpreter.StorageKey,
	capabilityAddress common.Address,
	capabilityID uint64,
	newAuthorization interpreter.Authorization,
) {
	r.reportWriter.Write(capabilityEntitlementsFixedEntry{
		StorageKey:        storageKey,
		CapabilityAddress: capabilityAddress,
		CapabilityID:      capabilityID,
		NewAuthorization:  newAuthorization,
	})
}

func jsonEncodeAuthorization(authorization interpreter.Authorization) string {
	switch authorization {
	case interpreter.UnauthorizedAccess, interpreter.InaccessibleAccess:
		return ""
	default:
		return string(authorization.ID())
	}
}

// capabilityControllerEntitlementsFixedEntry
type capabilityControllerEntitlementsFixedEntry struct {
	StorageKey       interpreter.StorageKey
	CapabilityID     uint64
	NewAuthorization interpreter.Authorization
}

var _ json.Marshaler = capabilityControllerEntitlementsFixedEntry{}

func (e capabilityControllerEntitlementsFixedEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind             string `json:"kind"`
		AccountAddress   string `json:"account_address"`
		StorageDomain    string `json:"domain"`
		CapabilityID     uint64 `json:"capability_id"`
		NewAuthorization string `json:"new_authorization"`
	}{
		Kind:             "capability-controller-entitlements-fixed",
		AccountAddress:   e.StorageKey.Address.HexWithPrefix(),
		StorageDomain:    e.StorageKey.Key,
		CapabilityID:     e.CapabilityID,
		NewAuthorization: jsonEncodeAuthorization(e.NewAuthorization),
	})
}

// capabilityEntitlementsFixedEntry
type capabilityEntitlementsFixedEntry struct {
	StorageKey        interpreter.StorageKey
	CapabilityAddress common.Address
	CapabilityID      uint64
	NewAuthorization  interpreter.Authorization
}

var _ json.Marshaler = capabilityEntitlementsFixedEntry{}

func (e capabilityEntitlementsFixedEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind              string `json:"kind"`
		AccountAddress    string `json:"account_address"`
		StorageDomain     string `json:"domain"`
		CapabilityAddress string `json:"capability_address"`
		CapabilityID      uint64 `json:"capability_id"`
		NewAuthorization  string `json:"new_authorization"`
	}{
		Kind:              "capability-entitlements-fixed",
		AccountAddress:    e.StorageKey.Address.HexWithPrefix(),
		StorageDomain:     e.StorageKey.Key,
		CapabilityAddress: e.CapabilityAddress.HexWithPrefix(),
		CapabilityID:      e.CapabilityID,
		NewAuthorization:  jsonEncodeAuthorization(e.NewAuthorization),
	})
}

func NewFixEntitlementsMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	newAuthorizations map[AccountCapabilityControllerID]interpreter.Authorization,
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
		// TODO: unnecessary? remove?
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
			Name: "fix-entitlements",
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]AccountBasedMigration{
					NewFixEntitlementsMigration(
						rwf,
						errorMessageHandler,
						programs,
						newAuthorizations,
						opts,
					),
				},
			),
		},
	}
}
