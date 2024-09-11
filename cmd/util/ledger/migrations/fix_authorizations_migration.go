package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

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
)

type AccountCapabilityID struct {
	Address      common.Address
	CapabilityID uint64
}

// FixAuthorizationsMigration

type FixAuthorizationsMigrationReporter interface {
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

type FixAuthorizationsMigration struct {
	Reporter          FixAuthorizationsMigrationReporter
	NewAuthorizations map[AccountCapabilityID]interpreter.Authorization
}

var _ migrations.ValueMigration = &FixAuthorizationsMigration{}

func (*FixAuthorizationsMigration) Name() string {
	return "FixAuthorizationsMigration"
}

func (*FixAuthorizationsMigration) Domains() map[string]struct{} {
	return nil
}

func (m *FixAuthorizationsMigration) Migrate(
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

		newAuthorization := m.NewAuthorizations[AccountCapabilityID{
			Address:      capabilityAddress,
			CapabilityID: capabilityID,
		}]
		if newAuthorization == nil {
			// Nothing to fix for this capability
			return nil, nil
		}

		oldBorrowType := value.BorrowType
		if oldBorrowType == nil {
			log.Warn().Msgf(
				"missing borrow type for capability with target %s#%d",
				capabilityAddress.HexWithPrefix(),
				capabilityID,
			)
		}

		oldBorrowReferenceType, ok := oldBorrowType.(*interpreter.ReferenceStaticType)
		if !ok {
			log.Warn().Msgf(
				"invalid non-reference borrow type for capability with target %s#%d: %s",
				capabilityAddress.HexWithPrefix(),
				capabilityID,
				oldBorrowType,
			)
			return nil, nil
		}

		newBorrowType := interpreter.NewReferenceStaticType(
			nil,
			newAuthorization,
			oldBorrowReferenceType.ReferencedType,
		)
		newCapabilityValue := interpreter.NewUnmeteredCapabilityValue(
			interpreter.UInt64Value(capabilityID),
			interpreter.AddressValue(capabilityAddress),
			newBorrowType,
		)

		m.Reporter.MigratedCapability(
			storageKey,
			capabilityAddress,
			capabilityID,
			newAuthorization,
		)

		return newCapabilityValue, nil

	case *interpreter.StorageCapabilityControllerValue:
		// The capability controller's address is implicitly
		// the address of the account in which it is stored
		capabilityAddress := storageKey.Address
		capabilityID := uint64(value.CapabilityID)

		newAuthorization := m.NewAuthorizations[AccountCapabilityID{
			Address:      capabilityAddress,
			CapabilityID: capabilityID,
		}]
		if newAuthorization == nil {
			// Nothing to fix for this capability controller
			return nil, nil
		}

		oldBorrowReferenceType := value.BorrowType

		newBorrowType := interpreter.NewReferenceStaticType(
			nil,
			newAuthorization,
			oldBorrowReferenceType.ReferencedType,
		)
		newStorageCapabilityControllerValue := interpreter.NewUnmeteredStorageCapabilityControllerValue(
			newBorrowType,
			interpreter.UInt64Value(capabilityID),
			value.TargetPath,
		)

		m.Reporter.MigratedCapabilityController(
			storageKey,
			capabilityID,
			newAuthorization,
		)

		return newStorageCapabilityControllerValue, nil
	}

	return nil, nil
}

func (*FixAuthorizationsMigration) CanSkip(valueType interpreter.StaticType) bool {
	return CanSkipFixAuthorizationsMigration(valueType)
}

func CanSkipFixAuthorizationsMigration(valueType interpreter.StaticType) bool {
	switch valueType := valueType.(type) {
	case *interpreter.DictionaryStaticType:
		return CanSkipFixAuthorizationsMigration(valueType.KeyType) &&
			CanSkipFixAuthorizationsMigration(valueType.ValueType)

	case interpreter.ArrayStaticType:
		return CanSkipFixAuthorizationsMigration(valueType.ElementType())

	case *interpreter.OptionalStaticType:
		return CanSkipFixAuthorizationsMigration(valueType.Type)

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

const fixAuthorizationsMigrationReporterName = "fix-authorizations-migration"

func NewFixAuthorizationsMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	programs map[runtime.Location]*interpreter.Program,
	newAuthorizations AuthorizationFixes,
	opts Options,
) *CadenceBaseMigration {
	var diffReporter reporters.ReportWriter
	if opts.DiffMigrations {
		diffReporter = rwf.ReportWriter("fix-authorizations-migration-diff")
	}

	reporter := rwf.ReportWriter(fixAuthorizationsMigrationReporterName)

	return &CadenceBaseMigration{
		name:                              "fix_authorizations_migration",
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
				&FixAuthorizationsMigration{
					NewAuthorizations: newAuthorizations,
					Reporter: &fixAuthorizationsMigrationReporter{
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

type fixAuthorizationsMigrationReporter struct {
	reportWriter        reporters.ReportWriter
	errorMessageHandler *errorMessageHandler
	verboseErrorOutput  bool
}

var _ FixAuthorizationsMigrationReporter = &fixAuthorizationsMigrationReporter{}
var _ migrations.Reporter = &fixAuthorizationsMigrationReporter{}

func (r *fixAuthorizationsMigrationReporter) Migrated(
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

func (r *fixAuthorizationsMigrationReporter) Error(err error) {

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

func (r *fixAuthorizationsMigrationReporter) DictionaryKeyConflict(accountAddressPath interpreter.AddressPath) {
	r.reportWriter.Write(dictionaryKeyConflictEntry{
		AddressPath: accountAddressPath,
	})
}

func (r *fixAuthorizationsMigrationReporter) MigratedCapabilityController(
	storageKey interpreter.StorageKey,
	capabilityID uint64,
	newAuthorization interpreter.Authorization,
) {
	r.reportWriter.Write(capabilityControllerAuthorizationFixedEntry{
		StorageKey:       storageKey,
		CapabilityID:     capabilityID,
		NewAuthorization: newAuthorization,
	})
}

func (r *fixAuthorizationsMigrationReporter) MigratedCapability(
	storageKey interpreter.StorageKey,
	capabilityAddress common.Address,
	capabilityID uint64,
	newAuthorization interpreter.Authorization,
) {
	r.reportWriter.Write(capabilityAuthorizationFixedEntry{
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

// capabilityControllerAuthorizationFixedEntry
type capabilityControllerAuthorizationFixedEntry struct {
	StorageKey       interpreter.StorageKey
	CapabilityID     uint64
	NewAuthorization interpreter.Authorization
}

var _ json.Marshaler = capabilityControllerAuthorizationFixedEntry{}

func (e capabilityControllerAuthorizationFixedEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind             string `json:"kind"`
		AccountAddress   string `json:"account_address"`
		StorageDomain    string `json:"domain"`
		CapabilityID     uint64 `json:"capability_id"`
		NewAuthorization string `json:"new_authorization"`
	}{
		Kind:             "capability-controller-authorizations-fixed",
		AccountAddress:   e.StorageKey.Address.HexWithPrefix(),
		StorageDomain:    e.StorageKey.Key,
		CapabilityID:     e.CapabilityID,
		NewAuthorization: jsonEncodeAuthorization(e.NewAuthorization),
	})
}

// capabilityAuthorizationFixedEntry
type capabilityAuthorizationFixedEntry struct {
	StorageKey        interpreter.StorageKey
	CapabilityAddress common.Address
	CapabilityID      uint64
	NewAuthorization  interpreter.Authorization
}

var _ json.Marshaler = capabilityAuthorizationFixedEntry{}

func (e capabilityAuthorizationFixedEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind              string `json:"kind"`
		AccountAddress    string `json:"account_address"`
		StorageDomain     string `json:"domain"`
		CapabilityAddress string `json:"capability_address"`
		CapabilityID      uint64 `json:"capability_id"`
		NewAuthorization  string `json:"new_authorization"`
	}{
		Kind:              "capability-authorizations-fixed",
		AccountAddress:    e.StorageKey.Address.HexWithPrefix(),
		StorageDomain:     e.StorageKey.Key,
		CapabilityAddress: e.CapabilityAddress.HexWithPrefix(),
		CapabilityID:      e.CapabilityID,
		NewAuthorization:  jsonEncodeAuthorization(e.NewAuthorization),
	})
}

func NewFixAuthorizationsMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	newAuthorizations AuthorizationFixes,
	opts Options,
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
				nil,
				programs,
			),
		},
		{
			Name: "fix-authorizations",
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]AccountBasedMigration{
					NewFixAuthorizationsMigration(
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

type AuthorizationFixes map[AccountCapabilityID]interpreter.Authorization

// ReadAuthorizationFixes reads a report of authorization fixes from the given reader.
// The report is expected to be a JSON array of objects with the following structure:
//
//	[
//		{"address":"0x1","identifier":"foo","linkType":"&Foo","accessibleMembers":["foo"]}
//	]
func ReadAuthorizationFixes(
	reader io.Reader,
	filter map[common.Address]struct{},
) (AuthorizationFixes, error) {

	fixes := AuthorizationFixes{}

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
			CapabilityAddress string `json:"capability_address"`
			CapabilityID      uint64 `json:"capability_id"`
			NewAuthorization  string `json:"new_authorization"`
		}
		err := dec.Decode(&entry)
		if err != nil {
			return nil, fmt.Errorf("failed to decode entry: %w", err)
		}

		address, err := common.HexToAddress(entry.CapabilityAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to parse address: %w", err)
		}

		if filter != nil {
			if _, ok := filter[address]; !ok {
				continue
			}
		}

		newAuthorization, err := jsonDecodeAuthorization(entry.NewAuthorization)
		if err != nil {
			return nil, fmt.Errorf("failed to decode new authorization '%s': %w", entry.NewAuthorization, err)
		}

		accountCapabilityID := AccountCapabilityID{
			Address:      address,
			CapabilityID: entry.CapabilityID,
		}

		fixes[accountCapabilityID] = newAuthorization
	}

	token, err = dec.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}
	if token != json.Delim(']') {
		return nil, fmt.Errorf("expected end of array, got %s", token)
	}

	return fixes, nil
}

func jsonDecodeAuthorization(encoded string) (interpreter.Authorization, error) {
	if encoded == "" {
		return interpreter.UnauthorizedAccess, nil
	}

	if strings.Contains(encoded, "|") {
		return nil, fmt.Errorf("invalid disjunction entitlement set authorization: %s", encoded)
	}

	var typeIDs []common.TypeID
	for _, part := range strings.Split(encoded, ",") {
		typeIDs = append(typeIDs, common.TypeID(part))
	}

	entitlementSetAuthorization := interpreter.NewEntitlementSetAuthorization(
		nil,
		func() []common.TypeID {
			return typeIDs
		},
		len(typeIDs),
		sema.Conjunction,
	)

	return entitlementSetAuthorization, nil
}
