package migrations

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/migrations"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

const metricsCollectingMigrationName = "metrics_collecting_migration"

type MetricsCollectingMigration struct {
	name              string
	chainID           flow.ChainID
	log               zerolog.Logger
	mutex             sync.RWMutex
	reporter          reporters.ReportWriter
	metricsCollector  *MigrationMetricsCollector
	migratedTypes     map[common.TypeID]struct{}
	migratedContracts map[common.Location]struct{}
	programs          map[common.Location]*interpreter.Program
}

var _ migrations.ValueMigration = &MetricsCollectingMigration{}
var _ AccountBasedMigration = &MetricsCollectingMigration{}

func NewMetricsCollectingMigration(
	log zerolog.Logger,
	chainID flow.ChainID,
	rwf reporters.ReportWriterFactory,
	programs map[common.Location]*interpreter.Program,
) *MetricsCollectingMigration {

	return &MetricsCollectingMigration{
		name:             metricsCollectingMigrationName,
		log:              log,
		reporter:         rwf.ReportWriter("metrics-collecting-migration"),
		metricsCollector: NewMigrationMetricsCollector(),
		chainID:          chainID,
		programs:         programs,
	}
}

func (*MetricsCollectingMigration) Name() string {
	return "MetricsCollectingMigration"
}

func (m *MetricsCollectingMigration) InitMigration(
	_ zerolog.Logger,
	_ *registers.ByAccount,
	_ int,
) error {
	m.migratedTypes = make(map[common.TypeID]struct{})
	m.migratedContracts = make(map[common.Location]struct{})

	// If the program is available, that means the associated contracts is compatible with Cadence 1.0.
	// i.e: the contract is either migrated to be compatible with 1.0 or existing contract already compatible.
	for _, program := range m.programs {
		var nestedDecls *ast.Members

		contract := program.Program.SoleContractDeclaration()
		if contract != nil {
			nestedDecls = contract.Members

			contractType := program.Elaboration.CompositeDeclarationType(contract)
			m.migratedTypes[contractType.ID()] = struct{}{}
			m.migratedContracts[contractType.Location] = struct{}{}
		} else {
			contractInterface := program.Program.SoleContractInterfaceDeclaration()
			if contractInterface == nil {
				panic(errors.NewUnreachableError())
			}
			nestedDecls = contractInterface.Members

			contractInterfaceType := program.Elaboration.InterfaceDeclarationType(contractInterface)
			m.migratedTypes[contractInterfaceType.ID()] = struct{}{}
			m.migratedContracts[contractInterfaceType.Location] = struct{}{}
		}

		for _, compositeDecl := range nestedDecls.Composites() {
			compositeType := program.Elaboration.CompositeDeclarationType(compositeDecl)
			if compositeType == nil {
				continue
			}
			m.migratedTypes[compositeType.ID()] = struct{}{}
		}

		for _, interfaceDecl := range nestedDecls.Interfaces() {
			interfaceType := program.Elaboration.InterfaceDeclarationType(interfaceDecl)
			if interfaceType == nil {
				continue
			}
			m.migratedTypes[interfaceType.ID()] = struct{}{}
		}

		for _, attachmentDecl := range nestedDecls.Attachments() {
			attachmentType := program.Elaboration.CompositeDeclarationType(attachmentDecl)
			if attachmentType == nil {
				continue
			}
			m.migratedTypes[attachmentType.ID()] = struct{}{}
		}

		// Entitlements are not needed, since old values won't have them.

		// TODO: Anything else? e.g: Enum cases?

	}

	return nil
}

func (m *MetricsCollectingMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {
	// Create all the runtime components we need for the migration
	migrationRuntime, err := NewInterpreterMigrationRuntime(
		accountRegisters,
		m.chainID,
		InterpreterMigrationRuntimeConfig{},
	)
	if err != nil {
		return fmt.Errorf("failed to create interpreter migration runtime: %w", err)
	}

	storage := migrationRuntime.Storage

	migration, err := migrations.NewStorageMigration(
		migrationRuntime.Interpreter,
		storage,
		m.name,
		address,
	)
	if err != nil {
		return fmt.Errorf("failed to create storage migration: %w", err)
	}

	migration.Migrate(
		migration.NewValueMigrationsPathMigrator(
			NewStorageVisitingErrorReporter(m.log),
			m,
		),
	)

	err = migration.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}

func (m *MetricsCollectingMigration) Migrate(
	_ interpreter.StorageKey,
	_ interpreter.StorageMapKey,
	value interpreter.Value,
	_ *interpreter.Interpreter,
	_ migrations.ValueMigrationPosition,
) (
	newValue interpreter.Value,
	err error,
) {

	if m.metricsCollector != nil {
		m.metricsCollector.RecordValue()
	}

	var migrated bool

	switch value := value.(type) {
	case interpreter.TypeValue:
		// Type is optional. nil represents "unknown"/"invalid" type
		ty := value.Type
		if ty == nil {
			return
		}

		migrated = m.isTypeMigrated(ty)

	case *interpreter.IDCapabilityValue:
		migrated = m.isTypeMigrated(value.BorrowType)

	case *interpreter.PathCapabilityValue: //nolint:staticcheck
		// Type is optional
		borrowType := value.BorrowType
		if borrowType == nil {
			return
		}
		migrated = m.isTypeMigrated(borrowType)

	case interpreter.PathLinkValue: //nolint:staticcheck
		migrated = m.isTypeMigrated(value.Type)

	case *interpreter.AccountCapabilityControllerValue:
		migrated = m.isTypeMigrated(value.BorrowType)

	case *interpreter.StorageCapabilityControllerValue:
		migrated = m.isTypeMigrated(value.BorrowType)

	case *interpreter.ArrayValue:
		migrated = m.isTypeMigrated(value.Type)

	case *interpreter.DictionaryValue:
		migrated = m.isTypeMigrated(value.Type)
	default:
		migrated = true
	}

	if !migrated && m.metricsCollector != nil {
		m.metricsCollector.RecordError()
	}

	return
}

func (m *MetricsCollectingMigration) isTypeMigrated(staticType interpreter.StaticType) bool {
	switch staticType := staticType.(type) {
	case *interpreter.ConstantSizedStaticType:
		return m.isTypeMigrated(staticType.Type)

	case *interpreter.VariableSizedStaticType:
		return m.isTypeMigrated(staticType.Type)

	case *interpreter.DictionaryStaticType:
		keyTypeMigrated := m.isTypeMigrated(staticType.KeyType)
		if !keyTypeMigrated {
			return false
		}
		return m.isTypeMigrated(staticType.ValueType)

	case *interpreter.CapabilityStaticType:
		borrowType := staticType.BorrowType
		if borrowType == nil {
			return true
		}
		return m.isTypeMigrated(borrowType)

	case *interpreter.IntersectionStaticType:
		for _, interfaceStaticType := range staticType.Types {
			migrated := m.isTypeMigrated(interfaceStaticType)
			if !migrated {
				return false
			}
		}
		return true

	case *interpreter.OptionalStaticType:
		return m.isTypeMigrated(staticType.Type)

	case *interpreter.ReferenceStaticType:
		return m.isTypeMigrated(staticType.ReferencedType)

	case interpreter.FunctionStaticType:
		// Non-storable
		return true

	case *interpreter.CompositeStaticType:
		primitiveType := interpreter.PrimitiveStaticTypeFromTypeID(staticType.TypeID)
		if primitiveType != interpreter.PrimitiveStaticTypeUnknown {
			return true
		}
		return m.checkAndRecordIsTypeMigrated(staticType.TypeID, staticType.Location)

	case *interpreter.InterfaceStaticType:
		return m.checkAndRecordIsTypeMigrated(staticType.TypeID, staticType.Location)

	case interpreter.PrimitiveStaticType:
		return true

	default:
		panic(errors.NewUnexpectedError("unexpected static type: %T", staticType))
	}
}

func (m *MetricsCollectingMigration) checkAndRecordIsTypeMigrated(typeID sema.TypeID, location common.Location) bool {
	// If a value related to a composite/interface type is found,
	// then count this value, to measure the total number of values/objects.
	m.metricsCollector.RecordValueForContract(location)

	_, ok := m.migratedTypes[typeID]
	if !ok {
		// If this type is not migrated/usable with cadence 1.0,
		// then record this as an erroneous value.
		m.metricsCollector.RecordErrorForContract(location)

		// If the type is not migrated, but the contract is migrated, then report an error.
		// This is more likely to be an implementation error, where the typeID haven't got added to the list.
		_, ok := m.migratedContracts[location]
		if ok {
			m.log.Error().Msgf(
				"contract `%s` is migrated, but cannot find the migrated type: `%s`",
				location,
				typeID,
			)
		}
	}

	return ok
}

func (m *MetricsCollectingMigration) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.reporter.Write(m.metricsCollector.metrics())

	// Close the report writer so it flushes to file.
	m.reporter.Close()
	return nil
}

func (m *MetricsCollectingMigration) CanSkip(interpreter.StaticType) bool {
	return false
}

func (m *MetricsCollectingMigration) Domains() map[string]struct{} {
	return nil
}

type MigrationMetricsCollector struct {
	mutex             sync.RWMutex
	TotalValues       int
	TotalErrors       int
	ValuesPerContract map[common.Location]int
	ErrorsPerContract map[common.Location]int
}

func NewMigrationMetricsCollector() *MigrationMetricsCollector {
	return &MigrationMetricsCollector{
		ErrorsPerContract: make(map[common.Location]int),
		ValuesPerContract: make(map[common.Location]int),
	}
}

func (c *MigrationMetricsCollector) RecordValue() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.TotalValues++
}

func (c *MigrationMetricsCollector) RecordError() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.TotalErrors++
}

func (c *MigrationMetricsCollector) RecordErrorForContract(location common.Location) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.ErrorsPerContract[location]++
}

func (c *MigrationMetricsCollector) RecordValueForContract(location common.Location) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.ValuesPerContract[location]++
}

func (c *MigrationMetricsCollector) metrics() Metrics {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	errorsPerContract := make(map[string]int, len(c.ErrorsPerContract))
	for location, count := range c.ErrorsPerContract {
		errorsPerContract[location.ID()] = count
	}

	valuesPerContract := make(map[string]int, len(c.ValuesPerContract))
	for location, count := range c.ValuesPerContract {
		valuesPerContract[location.ID()] = count
	}

	return Metrics{
		TotalValues:       c.TotalValues,
		TotalErrors:       c.TotalErrors,
		ErrorsPerContract: errorsPerContract,
		ValuesPerContract: valuesPerContract,
	}
}

type Metrics struct {
	// Total values in the storage
	TotalValues int `json:"totalValues"`

	// Total values with errors (un-migrated values)
	TotalErrors int `json:"TotalErrors"`

	// Values with errors (un-migrated) related to each contract
	ErrorsPerContract map[string]int `json:"errorsPerContract"`

	// Total values related to each contract
	ValuesPerContract map[string]int `json:"valuesPerContract"`
}

type storageVisitingErrorReporter struct {
	log zerolog.Logger
}

func NewStorageVisitingErrorReporter(log zerolog.Logger) *storageVisitingErrorReporter {
	return &storageVisitingErrorReporter{
		log: log,
	}
}

var _ migrations.Reporter = &storageVisitingErrorReporter{}

func (p *storageVisitingErrorReporter) Migrated(
	_ interpreter.StorageKey,
	_ interpreter.StorageMapKey,
	_ string,
) {
	// Ignore
}

func (p *storageVisitingErrorReporter) DictionaryKeyConflict(addressPath interpreter.AddressPath) {
	p.log.Error().Msgf("dictionary key conflict for %s", addressPath)
}

func (p *storageVisitingErrorReporter) Error(err error) {
	p.log.Error().Msgf("%s", err.Error())
}
