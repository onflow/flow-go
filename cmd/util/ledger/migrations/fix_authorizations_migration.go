package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/onflow/cadence/migrations"
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
	)
	MigratedCapabilityController(
		storageKey interpreter.StorageKey,
		capabilityID uint64,
	)
}

type FixAuthorizationsMigration struct {
	Reporter           FixAuthorizationsMigrationReporter
	AuthorizationFixes AuthorizationFixes
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

		_, ok := m.AuthorizationFixes[AccountCapabilityID{
			Address:      capabilityAddress,
			CapabilityID: capabilityID,
		}]
		if !ok {
			// This capability does not need to be fixed
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
			interpreter.UnauthorizedAccess,
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
		)

		return newCapabilityValue, nil

	case *interpreter.StorageCapabilityControllerValue:
		// The capability controller's address is implicitly
		// the address of the account in which it is stored
		capabilityAddress := storageKey.Address
		capabilityID := uint64(value.CapabilityID)

		_, ok := m.AuthorizationFixes[AccountCapabilityID{
			Address:      capabilityAddress,
			CapabilityID: capabilityID,
		}]
		if !ok {
			// This capability controller does not need to be fixed
			return nil, nil
		}

		oldBorrowReferenceType := value.BorrowType

		newBorrowType := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
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
	authorizationFixes AuthorizationFixes,
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
					AuthorizationFixes: authorizationFixes,
					Reporter: &fixAuthorizationsMigrationReporter{
						reportWriter:       reporter,
						verboseErrorOutput: opts.VerboseErrorOutput,
					},
				},
			}
		},
		chainID: opts.ChainID,
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
) {
	r.reportWriter.Write(capabilityControllerAuthorizationFixedEntry{
		StorageKey:   storageKey,
		CapabilityID: capabilityID,
	})
}

func (r *fixAuthorizationsMigrationReporter) MigratedCapability(
	storageKey interpreter.StorageKey,
	capabilityAddress common.Address,
	capabilityID uint64,
) {
	r.reportWriter.Write(capabilityAuthorizationFixedEntry{
		StorageKey:        storageKey,
		CapabilityAddress: capabilityAddress,
		CapabilityID:      capabilityID,
	})
}

// capabilityControllerAuthorizationFixedEntry
type capabilityControllerAuthorizationFixedEntry struct {
	StorageKey   interpreter.StorageKey
	CapabilityID uint64
}

var _ json.Marshaler = capabilityControllerAuthorizationFixedEntry{}

func (e capabilityControllerAuthorizationFixedEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		StorageDomain  string `json:"domain"`
		CapabilityID   uint64 `json:"capability_id"`
	}{
		Kind:           "capability-controller-authorizations-fixed",
		AccountAddress: e.StorageKey.Address.HexWithPrefix(),
		StorageDomain:  e.StorageKey.Key,
		CapabilityID:   e.CapabilityID,
	})
}

// capabilityAuthorizationFixedEntry
type capabilityAuthorizationFixedEntry struct {
	StorageKey        interpreter.StorageKey
	CapabilityAddress common.Address
	CapabilityID      uint64
}

var _ json.Marshaler = capabilityAuthorizationFixedEntry{}

func (e capabilityAuthorizationFixedEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind              string `json:"kind"`
		AccountAddress    string `json:"account_address"`
		StorageDomain     string `json:"domain"`
		CapabilityAddress string `json:"capability_address"`
		CapabilityID      uint64 `json:"capability_id"`
	}{
		Kind:              "capability-authorizations-fixed",
		AccountAddress:    e.StorageKey.Address.HexWithPrefix(),
		StorageDomain:     e.StorageKey.Key,
		CapabilityAddress: e.CapabilityAddress.HexWithPrefix(),
		CapabilityID:      e.CapabilityID,
	})
}

func NewFixAuthorizationsMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	authorizationFixes AuthorizationFixes,
	opts Options,
) []NamedMigration {

	return []NamedMigration{
		{
			Name: "fix-authorizations",
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]AccountBasedMigration{
					NewFixAuthorizationsMigration(
						rwf,
						authorizationFixes,
						opts,
					),
				},
			),
		},
	}
}

type AuthorizationFixes map[AccountCapabilityID]struct{}

// ReadAuthorizationFixes reads a report of authorization fixes from the given reader.
// The report is expected to be a JSON array of objects with the following structure:
//
//	[
//		{"capability_address":"0x1","capability_id":1}
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

		accountCapabilityID := AccountCapabilityID{
			Address:      address,
			CapabilityID: entry.CapabilityID,
		}

		fixes[accountCapabilityID] = struct{}{}
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
