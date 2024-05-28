package migrations

import (
	"context"
	"fmt"
	"github.com/onflow/cadence/runtime/sema"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/migrations"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

const metricsCollectingMigrationName = "metrics_collecting_migration"

type MetricsCollectingMigration struct {
	name             string
	chainID          flow.ChainID
	mutex            sync.RWMutex
	reporter         reporters.ReportWriter
	metricsCollector *MigrationMetricsCollector
	migratedTypes    map[common.TypeID]struct{}
	programs         map[common.Location]*interpreter.Program
}

var _ migrations.ValueMigration = &MetricsCollectingMigration{}
var _ AccountBasedMigration = &MetricsCollectingMigration{}

func NewMetricsCollectingMigration(
	chainID flow.ChainID,
	rwf reporters.ReportWriterFactory,
	programs map[common.Location]*interpreter.Program,
) *MetricsCollectingMigration {

	return &MetricsCollectingMigration{
		name:             metricsCollectingMigrationName,
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

	for _, program := range m.programs {
		if program.Program == nil {
			continue
		}

		var nestedDecls *ast.Members

		contract := program.Program.SoleContractDeclaration()
		if contract != nil {
			nestedDecls = contract.Members
		} else {
			contractInterface := program.Program.SoleContractInterfaceDeclaration()
			if contractInterface == nil {
				panic(errors.NewUnreachableError())
			}
			nestedDecls = contractInterface.Members
		}

		for _, composite := range nestedDecls.Composites() {
			typeID := program.Elaboration.CompositeDeclarationType(composite).ID()
			m.migratedTypes[typeID] = struct{}{}
		}

		for _, composite := range nestedDecls.Interfaces() {
			typeID := program.Elaboration.InterfaceDeclarationType(composite).ID()
			m.migratedTypes[typeID] = struct{}{}
		}

		// TODO: also add the contract type itself?
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
			nil, // No need to report
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
		return m.checkIsTypeMigrated(staticType.TypeID, staticType.Location)

	case *interpreter.InterfaceStaticType:
		return m.checkIsTypeMigrated(staticType.TypeID, staticType.Location)

	case interpreter.PrimitiveStaticType:
		return true

	default:
		panic(errors.NewUnexpectedError("unexpected static type: %T", staticType))
	}
}

func (m *MetricsCollectingMigration) checkIsTypeMigrated(typeID sema.TypeID, location common.Location) bool {
	m.metricsCollector.RecordValueForContract(location)

	_, ok := m.migratedTypes[typeID]
	if !ok {
		m.metricsCollector.RecordErrorForContract(location)
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
	TotalValues       int                     `json:"totalValues"`
	TotalErrors       int                     `json:"totalErrors"`
	ValuesPerContract map[common.Location]int `json:"valuesPerContract"`
	ErrorsPerContract map[common.Location]int `json:"errorsPerContract"`
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
	errorsPerContract := make([]ContractErrors, 0, len(c.ErrorsPerContract))

	for location, count := range c.ErrorsPerContract {
		errorsPerContract = append(
			errorsPerContract,
			ContractErrors{
				Contract: location.ID(),
				Count:    count,
			})
	}

	sort.SliceStable(errorsPerContract, func(i, j int) bool {
		this := errorsPerContract[i]
		that := errorsPerContract[j]
		return strings.Compare(this.Contract, that.Contract) < 0
	})

	valuesPerContract := make([]ContractValues, 0, len(c.ValuesPerContract))
	for location, count := range c.ValuesPerContract {
		valuesPerContract = append(
			valuesPerContract,
			ContractValues{
				Contract: location.ID(),
				Count:    count,
			})
	}

	sort.SliceStable(valuesPerContract, func(i, j int) bool {
		this := valuesPerContract[i]
		that := valuesPerContract[j]
		return strings.Compare(this.Contract, that.Contract) < 0
	})

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
	ErrorsPerContract []ContractErrors `json:"errorsPerContract"`

	// Total values related to each contract
	ValuesPerContract []ContractValues `json:"valuesPerContract"`
}

type ContractErrors struct {
	Contract string `json:"contract"`
	Count    int    `json:"count"`
}

type ContractValues struct {
	Contract string `json:"contract"`
	Count    int    `json:"count"`

	// TODO:
	// Size
}
