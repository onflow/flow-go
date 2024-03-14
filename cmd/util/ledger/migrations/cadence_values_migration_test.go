package migrations

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const snapshotPath = "test-data/cadence_values_migration/snapshot_cadence_v0.42.6"

const testAccountAddress = "01cf0e2f2f715450"

type writer struct {
	logs []string
}

var _ io.Writer = &writer{}

func (w *writer) Write(p []byte) (n int, err error) {
	w.logs = append(w.logs, string(p))
	return len(p), nil
}

//go:embed test-data/cadence_values_migration/test_contract_upgraded.cdc
var testContractUpgraded []byte

func TestCadenceValuesMigration(t *testing.T) {

	t.Parallel()

	address, err := common.HexToAddress(testAccountAddress)
	require.NoError(t, err)

	// Get the old payloads
	payloads, err := util.PayloadsFromEmulatorSnapshot(snapshotPath)
	require.NoError(t, err)

	rwf := &testReportWriterFactory{}

	logWriter := &writer{}
	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

	const nWorker = 2

	const chainID = flow.Emulator
	// TODO: EVM contract is not deployed in snapshot yet, so can't update it
	const evmContractChange = EVMContractChangeNone

	const burnerContractChange = BurnerContractChangeDeploy

	stagedContracts := []StagedContract{
		{
			Contract: Contract{
				Name: "Test",
				Code: testContractUpgraded,
			},
			Address: address,
		},
	}

	migrations := NewCadence1Migrations(
		logger,
		rwf,
		nWorker,
		chainID,
		false,
		false,
		evmContractChange,
		burnerContractChange,
		stagedContracts,
		false,
		0,
	)

	for _, migration := range migrations {
		payloads, err = migration.Migrate(payloads)
		require.NoError(
			t,
			err,
			"migration `%s` failed, logs: %v",
			migration.Name,
			logWriter.logs,
		)
	}

	// Assert the migrated payloads
	checkMigratedPayloads(t, address, payloads)

	// Check reporters
	checkReporters(t, rwf, address)

	// Check error logs.
	require.Empty(t, logWriter.logs)
}

var flowTokenAddress = func() common.Address {
	address, _ := common.HexToAddress("0ae53cb6e3f42a79")
	return address
}()

func checkMigratedPayloads(
	t *testing.T,
	address common.Address,
	newPayloads []*ledger.Payload,
) {
	mr, err := NewMigratorRuntime(
		address,
		newPayloads,
		util.RuntimeInterfaceConfig{},
	)
	require.NoError(t, err)

	storageMap := mr.Storage.GetStorageMap(address, common.PathDomainStorage.Identifier(), false)
	require.NotNil(t, storageMap)
	require.Equal(t, 12, int(storageMap.Count()))

	iterator := storageMap.Iterator(mr.Interpreter)

	// Check whether the account ID is properly incremented.
	checkAccountID(t, mr, address)

	fullyEntitledAccountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.FullyEntitledAccountReferenceType)
	accountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.AccountReferenceType)

	var values []interpreter.Value
	for key, value := iterator.Next(); key != nil; key, value = iterator.Next() {
		values = append(values, value)
	}

	testContractLocation := common.NewAddressLocation(
		nil,
		address,
		"Test",
	)

	flowTokenLocation := common.NewAddressLocation(
		nil,
		flowTokenAddress,
		"FlowToken",
	)

	fooInterfaceType := interpreter.NewInterfaceStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.Foo",
	)

	barInterfaceType := interpreter.NewInterfaceStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.Bar",
	)

	bazInterfaceType := interpreter.NewInterfaceStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.Baz",
	)

	rResourceType := interpreter.NewCompositeStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.R",
	)

	entitlementAuthorization := func() interpreter.EntitlementSetAuthorization {
		return interpreter.NewEntitlementSetAuthorization(
			nil,
			func() (entitlements []common.TypeID) {
				return []common.TypeID{
					testContractLocation.TypeID(nil, "Test.E"),
				}
			},
			1,
			sema.Conjunction,
		)
	}

	expectedValues := []interpreter.Value{
		// Both string values should be in the normalized form.
		interpreter.NewUnmeteredStringValue("Caf\u00E9"),
		interpreter.NewUnmeteredStringValue("Caf\u00E9"),

		interpreter.NewUnmeteredTypeValue(fullyEntitledAccountReferenceType),

		interpreter.NewDictionaryValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeString,
				interpreter.PrimitiveStaticTypeInt,
			),
			interpreter.NewUnmeteredStringValue("Caf\u00E9"),
			interpreter.NewUnmeteredIntValueFromInt64(1),
			interpreter.NewUnmeteredStringValue("H\u00E9llo"),
			interpreter.NewUnmeteredIntValueFromInt64(2),
		),

		interpreter.NewDictionaryValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeMetaType,
				interpreter.PrimitiveStaticTypeInt,
			),
			interpreter.NewUnmeteredTypeValue(
				&interpreter.IntersectionStaticType{
					Types: []*interpreter.InterfaceStaticType{
						fooInterfaceType,
						barInterfaceType,
					},
					LegacyType: interpreter.PrimitiveStaticTypeAnyStruct,
				},
			),
			interpreter.NewUnmeteredIntValueFromInt64(1),
			interpreter.NewUnmeteredTypeValue(
				&interpreter.IntersectionStaticType{
					Types: []*interpreter.InterfaceStaticType{
						fooInterfaceType,
						barInterfaceType,
						bazInterfaceType,
					},
					LegacyType: interpreter.PrimitiveStaticTypeAnyStruct,
				},
			),
			interpreter.NewUnmeteredIntValueFromInt64(2),
		),

		interpreter.NewCompositeValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			testContractLocation,
			"Test.R",
			common.CompositeKindResource,
			[]interpreter.CompositeField{
				{
					Value: interpreter.NewUnmeteredUInt64Value(360287970189639680),
					Name:  "uuid",
				},
			},
			address,
		),

		interpreter.NewUnmeteredSomeValueNonCopying(
			interpreter.NewUnmeteredCapabilityValue(
				interpreter.NewUnmeteredUInt64Value(2),
				interpreter.NewAddressValue(nil, address),
				interpreter.NewReferenceStaticType(nil, entitlementAuthorization(), rResourceType),
			),
		),

		interpreter.NewUnmeteredCapabilityValue(
			interpreter.NewUnmeteredUInt64Value(2),
			interpreter.NewAddressValue(nil, address),
			interpreter.NewReferenceStaticType(nil, entitlementAuthorization(), rResourceType),
		),

		interpreter.NewDictionaryValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeMetaType,
				interpreter.PrimitiveStaticTypeInt,
			),
			interpreter.NewUnmeteredTypeValue(fullyEntitledAccountReferenceType),
			interpreter.NewUnmeteredIntValueFromInt64(1),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_Capabilities),
			interpreter.NewUnmeteredIntValueFromInt64(2),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_AccountCapabilities),
			interpreter.NewUnmeteredIntValueFromInt64(3),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_StorageCapabilities),
			interpreter.NewUnmeteredIntValueFromInt64(4),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_Contracts),
			interpreter.NewUnmeteredIntValueFromInt64(5),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_Keys),
			interpreter.NewUnmeteredIntValueFromInt64(6),
			interpreter.NewUnmeteredTypeValue(interpreter.PrimitiveStaticTypeAccount_Inbox),
			interpreter.NewUnmeteredIntValueFromInt64(7),
			interpreter.NewUnmeteredTypeValue(accountReferenceType),
			interpreter.NewUnmeteredIntValueFromInt64(8),
			interpreter.NewUnmeteredTypeValue(interpreter.AccountKeyStaticType),
			interpreter.NewUnmeteredIntValueFromInt64(9),
		),

		interpreter.NewDictionaryValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeMetaType,
				interpreter.PrimitiveStaticTypeString,
			),
			interpreter.NewUnmeteredTypeValue(
				interpreter.NewReferenceStaticType(
					nil,
					entitlementAuthorization(),
					rResourceType,
				),
			),
			interpreter.NewUnmeteredStringValue("non_auth_ref"),
		),
		interpreter.NewDictionaryValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			interpreter.NewDictionaryStaticType(
				nil,
				interpreter.PrimitiveStaticTypeMetaType,
				interpreter.PrimitiveStaticTypeString,
			),
			interpreter.NewUnmeteredTypeValue(
				interpreter.NewReferenceStaticType(
					nil,
					entitlementAuthorization(),
					rResourceType,
				),
			),
			interpreter.NewUnmeteredStringValue("auth_ref"),
		),

		interpreter.NewCompositeValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			flowTokenLocation,
			"FlowToken.Vault",
			common.CompositeKindResource,
			[]interpreter.CompositeField{
				{
					Value: interpreter.NewUnmeteredUFix64Value(0.001 * sema.Fix64Factor),
					Name:  "balance",
				},
				{
					Value: interpreter.NewUnmeteredUInt64Value(11240984669916758018),
					Name:  "uuid",
				},
			},
			address,
		),
	}

	require.Equal(t, len(expectedValues), len(values))

	// Order is non-deterministic, so do a greedy compare.
	for _, value := range values {
		found := false
		actualValue := value.(interpreter.EquatableValue)
		for i, expectedValue := range expectedValues {
			if actualValue.Equal(mr.Interpreter, interpreter.EmptyLocationRange, expectedValue) {
				expectedValues = append(expectedValues[:i], expectedValues[i+1:]...)
				found = true
				break
			}

		}
		if !found {
			assert.Fail(t, fmt.Sprintf("extra item in actual values: %s", actualValue))
		}
	}

	if len(expectedValues) != 0 {
		assert.Fail(t, fmt.Sprintf("%d extra item(s) in expected values: %s", len(expectedValues), expectedValues))
	}
}

func checkAccountID(t *testing.T, mr *migratorRuntime, address common.Address) {
	id := flow.AccountStatusRegisterID(flow.Address(address))
	statusBytes, err := mr.Accounts.GetValue(id)
	require.NoError(t, err)

	accountStatus, err := environment.AccountStatusFromBytes(statusBytes)
	require.NoError(t, err)

	assert.Equal(t, uint64(3), accountStatus.AccountIdCounter())
}

func checkReporters(
	t *testing.T,
	rwf *testReportWriterFactory,
	address common.Address,
) {

	testContractLocation := common.NewAddressLocation(
		nil,
		address,
		"Test",
	)

	rResourceType := interpreter.NewCompositeStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.R",
	)

	entitlementAuthorization := func() interpreter.EntitlementSetAuthorization {
		return interpreter.NewEntitlementSetAuthorization(
			nil,
			func() (entitlements []common.TypeID) {
				return []common.TypeID{
					testContractLocation.TypeID(nil, "Test.E"),
				}
			},
			1,
			sema.Conjunction,
		)
	}

	newCadenceValueMigrationReportEntry := func(
		migration, key string,
		domain common.PathDomain,
	) cadenceValueMigrationReportEntry {
		return cadenceValueMigrationReportEntry{
			StorageMapKey: interpreter.StringStorageMapKey(key),
			StorageKey: interpreter.NewStorageKey(
				nil,
				address,
				domain.Identifier(),
			),
			Migration: migration,
		}
	}

	var accountReportEntries []reportEntry

	for _, reportWriter := range rwf.reportWriters {
		for _, entry := range reportWriter.entries {

			e := entry.(reportEntry)
			if e.accountAddress() != address {
				continue
			}

			accountReportEntries = append(accountReportEntries, e)
		}
	}

	acctTypedDictKeyMigrationReportEntry := newCadenceValueMigrationReportEntry(
		"StaticTypeMigration",
		"dictionary_with_account_type_keys",
		common.PathDomainStorage)

	// Order is non-deterministic, so use 'ElementsMatch'.
	assert.ElementsMatch(
		t,
		[]reportEntry{
			newCadenceValueMigrationReportEntry(
				"StringNormalizingMigration",
				"string_value_1",
				common.PathDomainStorage,
			),
			newCadenceValueMigrationReportEntry(
				"StaticTypeMigration",
				"type_value",
				common.PathDomainStorage,
			),

			// String keys in dictionary
			newCadenceValueMigrationReportEntry(
				"StringNormalizingMigration",
				"dictionary_with_string_keys",
				common.PathDomainStorage,
			),

			// Restricted typed keys in dictionary
			newCadenceValueMigrationReportEntry(
				"StaticTypeMigration",
				"dictionary_with_restricted_typed_keys",
				common.PathDomainStorage,
			),
			newCadenceValueMigrationReportEntry(
				"StaticTypeMigration",
				"dictionary_with_restricted_typed_keys",
				common.PathDomainStorage,
			),

			// Capabilities and links
			cadenceValueMigrationReportEntry{
				StorageMapKey: interpreter.StringStorageMapKey("capability"),
				StorageKey: interpreter.NewStorageKey(
					nil,
					address,
					common.PathDomainStorage.Identifier(),
				),
				Migration: "CapabilityValueMigration",
			},
			capConsPathCapabilityMigrationEntry{
				AccountAddress: address,
				AddressPath: interpreter.AddressPath{
					Address: address,
					Path: interpreter.NewUnmeteredPathValue(
						common.PathDomainPublic,
						"linkR",
					),
				},
				BorrowType: interpreter.NewReferenceStaticType(
					nil,
					entitlementAuthorization(),
					rResourceType,
				),
			},
			newCadenceValueMigrationReportEntry(
				"EntitlementsMigration",
				"capability",
				common.PathDomainStorage,
			),
			newCadenceValueMigrationReportEntry(
				"EntitlementsMigration",
				"linkR",
				common.PathDomainPublic,
			),

			// untyped capability
			capConsPathCapabilityMigrationEntry{
				AccountAddress: address,
				AddressPath: interpreter.AddressPath{
					Address: address,
					Path: interpreter.NewUnmeteredPathValue(
						common.PathDomainPublic,
						"linkR",
					),
				},
				BorrowType: interpreter.NewReferenceStaticType(
					nil,
					entitlementAuthorization(),
					rResourceType,
				),
			},
			newCadenceValueMigrationReportEntry(
				"CapabilityValueMigration",
				"untyped_capability",
				common.PathDomainStorage,
			),

			// Account-typed keys in dictionary
			acctTypedDictKeyMigrationReportEntry,
			acctTypedDictKeyMigrationReportEntry,
			acctTypedDictKeyMigrationReportEntry,
			acctTypedDictKeyMigrationReportEntry,
			acctTypedDictKeyMigrationReportEntry,
			acctTypedDictKeyMigrationReportEntry,
			acctTypedDictKeyMigrationReportEntry,
			acctTypedDictKeyMigrationReportEntry,
			acctTypedDictKeyMigrationReportEntry,

			// Entitled typed keys in dictionary
			newCadenceValueMigrationReportEntry(
				"EntitlementsMigration",
				"dictionary_with_auth_reference_typed_key",
				common.PathDomainStorage,
			),
			newCadenceValueMigrationReportEntry(
				"EntitlementsMigration",
				"dictionary_with_reference_typed_key",
				common.PathDomainStorage,
			),

			// Entitlements in links
			newCadenceValueMigrationReportEntry(
				"EntitlementsMigration",
				"flowTokenReceiver",
				common.PathDomainPublic,
			),
			newCadenceValueMigrationReportEntry(
				"EntitlementsMigration",
				"flowTokenBalance",
				common.PathDomainPublic,
			),

			// Cap cons

			capConsLinkMigrationEntry{
				AccountAddressPath: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "flowTokenReceiver",
						Domain:     common.PathDomainPublic,
					},
				},
				CapabilityID: 1,
			},
			newCadenceValueMigrationReportEntry(
				"LinkValueMigration",
				"flowTokenReceiver",
				common.PathDomainPublic,
			),

			capConsLinkMigrationEntry{
				AccountAddressPath: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "linkR",
						Domain:     common.PathDomainPublic,
					},
				},
				CapabilityID: 2,
			},
			newCadenceValueMigrationReportEntry(
				"LinkValueMigration",
				"linkR",
				common.PathDomainPublic,
			),

			capConsLinkMigrationEntry{
				AccountAddressPath: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "flowTokenBalance",
						Domain:     common.PathDomainPublic,
					},
				},
				CapabilityID: 3,
			},
			newCadenceValueMigrationReportEntry(
				"LinkValueMigration",
				"flowTokenBalance",
				common.PathDomainPublic,
			),
		},
		accountReportEntries,
	)
}

type testReportWriterFactory struct {
	lock          sync.Mutex
	reportWriters map[string]*testReportWriter
}

func (f *testReportWriterFactory) ReportWriter(dataNamespace string) reporters.ReportWriter {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.reportWriters == nil {
		f.reportWriters = make(map[string]*testReportWriter)
	}
	reportWriter := &testReportWriter{}
	if _, ok := f.reportWriters[dataNamespace]; ok {
		panic(fmt.Sprintf("report writer already exists for namespace %s", dataNamespace))
	}
	f.reportWriters[dataNamespace] = reportWriter
	return reportWriter
}

type testReportWriter struct {
	lock    sync.Mutex
	entries []any
}

var _ reporters.ReportWriter = &testReportWriter{}

func (r *testReportWriter) Write(entry any) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.entries = append(r.entries, entry)
}

func (r *testReportWriter) Close() {}

func TestBootstrappedStateMigration(t *testing.T) {
	t.Parallel()

	rwf := &testReportWriterFactory{}

	logWriter := &writer{}
	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

	const nWorker = 2

	const chainID = flow.Emulator
	// TODO: EVM contract is not deployed in snapshot yet, so can't update it
	const evmContractChange = EVMContractChangeNone

	const burnerContractChange = BurnerContractChangeUpdate

	payloads, err := newBootstrapPayloads(chainID)
	require.NoError(t, err)

	migrations := NewCadence1Migrations(
		logger,
		rwf,
		nWorker,
		chainID,
		false,
		false,
		evmContractChange,
		burnerContractChange,
		nil,
		false,
		0,
	)

	for _, migration := range migrations {
		payloads, err = migration.Migrate(payloads)
		require.NoError(
			t,
			err,
			"migration `%s` failed, logs: %v",
			migration.Name,
			logWriter.logs,
		)
	}

	// Check error logs.
	require.Empty(t, logWriter.logs)
}

func TestProgramParsingError(t *testing.T) {
	t.Parallel()

	rwf := &testReportWriterFactory{}

	logWriter := &writer{}
	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

	const nWorker = 2

	const chainID = flow.Emulator
	chain := chainID.Chain()

	testAddress := common.Address(chain.ServiceAddress())

	payloads, err := newBootstrapPayloads(chainID)
	require.NoError(t, err)

	runtime, err := NewMigratorRuntime(
		testAddress,
		payloads,
		util.RuntimeInterfaceConfig{},
	)
	require.NoError(t, err)

	storage := runtime.Storage

	storageMap := storage.GetStorageMap(
		testAddress,
		common.PathDomainStorage.Identifier(),
		true,
	)

	const contractName = "C"
	contractLocation := common.NewAddressLocation(nil, testAddress, contractName)

	const nonExistingStructQualifiedIdentifier = contractName + ".NonExistingStruct"

	capabilityValue := interpreter.NewUnmeteredCapabilityValue(
		0,
		interpreter.AddressValue(testAddress),
		interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewCompositeStaticType(
				nil,
				contractLocation,
				nonExistingStructQualifiedIdentifier,
				contractLocation.TypeID(nil, nonExistingStructQualifiedIdentifier),
			),
		),
	)

	storageMap.WriteValue(
		runtime.Interpreter,
		interpreter.StringStorageMapKey("test"),
		capabilityValue,
	)

	err = storage.Commit(runtime.Interpreter, false)
	require.NoError(t, err)

	// finalize the transaction
	result, err := runtime.TransactionState.FinalizeMainTransaction()
	require.NoError(t, err)

	// Merge the changes to the original payloads.

	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(testAddress): {},
	}

	payloads, err = MergeRegisterChanges(
		runtime.Snapshot.Payloads,
		result.WriteSet,
		expectedAddresses,
		nil,
		logger,
	)
	require.NoError(t, err)

	// Set the code for the old program

	payloads = append(
		payloads,
		newContractPayload(
			testAddress,
			contractName,
			[]byte(`pub contract C {}`),
		),
	)

	// Migrate

	// TODO: EVM contract is not deployed in snapshot yet, so can't update it
	const evmContractChange = EVMContractChangeNone

	const burnerContractChange = BurnerContractChangeUpdate

	migrations := NewCadence1Migrations(
		logger,
		rwf,
		nWorker,
		chainID,
		false,
		false,
		evmContractChange,
		burnerContractChange,
		nil,
		false,
		0,
	)

	for _, migration := range migrations {
		payloads, err = migration.Migrate(payloads)
		require.NoError(
			t,
			err,
			"migration `%s` failed, logs: %v",
			migration.Name,
			logWriter.logs,
		)
	}

	// Check error logs
	require.Len(t, logWriter.logs, 1)

	log := logWriter.logs[0]

	var entry struct {
		Message string `json:"message"`
	}

	err = json.Unmarshal([]byte(log), &entry)
	require.NoError(t, err)

	assert.Contains(t, entry.Message, "`pub` is no longer a valid access keyword")
	assert.NotContains(t, entry.Message, "runtime/debug.Stack()")
}

func TestCoreContractUsage(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator

	migrate := func(t *testing.T, staticType interpreter.StaticType) interpreter.StaticType {

		rwf := &testReportWriterFactory{}

		logWriter := &writer{}
		logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

		const nWorker = 2

		chain := chainID.Chain()

		testFlowAddress, err := chain.AddressAtIndex(1_000_000)
		require.NoError(t, err)

		testAddress := common.Address(testFlowAddress)

		payloads, err := newBootstrapPayloads(chainID)
		require.NoError(t, err)

		runtime, err := NewMigratorRuntime(
			testAddress,
			payloads,
			util.RuntimeInterfaceConfig{},
		)
		require.NoError(t, err)

		err = runtime.Accounts.Create(nil, testFlowAddress)
		require.NoError(t, err)

		storage := runtime.Storage

		storageDomain := common.PathDomainStorage.Identifier()
		storageMapKey := interpreter.StringStorageMapKey("test")

		storageMap := storage.GetStorageMap(
			testAddress,
			storageDomain,
			true,
		)

		capabilityValue := interpreter.NewUnmeteredCapabilityValue(
			0,
			interpreter.AddressValue(testAddress),
			staticType,
		)

		storageMap.WriteValue(
			runtime.Interpreter,
			storageMapKey,
			capabilityValue,
		)

		err = storage.Commit(runtime.Interpreter, false)
		require.NoError(t, err)

		// finalize the transaction
		result, err := runtime.TransactionState.FinalizeMainTransaction()
		require.NoError(t, err)

		// Merge the changes to the original payloads.

		expectedAddresses := map[flow.Address]struct{}{
			flow.Address(testAddress): {},
		}

		payloads, err = MergeRegisterChanges(
			runtime.Snapshot.Payloads,
			result.WriteSet,
			expectedAddresses,
			nil,
			logger,
		)
		require.NoError(t, err)

		// Migrate

		// TODO: EVM contract is not deployed in snapshot yet, so can't update it
		const evmContractChange = EVMContractChangeNone

		const burnerContractChange = BurnerContractChangeUpdate

		migrations := NewCadence1Migrations(
			logger,
			rwf,
			nWorker,
			chainID,
			false,
			false,
			evmContractChange,
			burnerContractChange,
			nil,
			false,
			0,
		)

		for _, migration := range migrations {
			payloads, err = migration.Migrate(payloads)
			require.NoError(
				t,
				err,
				"migration `%s` failed, logs: %v",
				migration.Name,
				logWriter.logs,
			)
		}

		// Check error logs
		require.Len(t, logWriter.logs, 0)

		// Get result

		mr, err := NewMigratorRuntime(
			testAddress,
			payloads,
			util.RuntimeInterfaceConfig{},
		)
		require.NoError(t, err)

		storageMap = mr.Storage.GetStorageMap(
			testAddress,
			storageDomain,
			false,
		)
		require.NotNil(t, storageMap)

		resultValue := storageMap.ReadValue(nil, storageMapKey)
		require.NotNil(t, resultValue)
		require.IsType(t, &interpreter.IDCapabilityValue{}, resultValue)

		resultCap := resultValue.(*interpreter.IDCapabilityValue)
		return resultCap.BorrowType
	}

	t.Run("&FungibleToken.Vault => auth(Withdraw) &{FungibleToken.Vault}", func(t *testing.T) {
		t.Parallel()

		systemContracts := systemcontracts.SystemContractsForChain(chainID)

		const fungibleTokenContractName = "FungibleToken"
		fungibleTokenContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.FungibleToken.Address),
			fungibleTokenContractName,
		)

		const fungibleTokenVaultTypeQualifiedIdentifier = fungibleTokenContractName + ".Vault"

		input := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewCompositeStaticType(
				nil,
				fungibleTokenContractLocation,
				fungibleTokenVaultTypeQualifiedIdentifier,
				fungibleTokenContractLocation.TypeID(nil, fungibleTokenVaultTypeQualifiedIdentifier),
			),
		)

		const fungibleTokenWithdrawTypeQualifiedIdentifier = fungibleTokenContractName + ".Withdraw"
		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.NewEntitlementSetAuthorization(
				nil,
				func() []common.TypeID {
					return []common.TypeID{
						fungibleTokenContractLocation.TypeID(nil, fungibleTokenWithdrawTypeQualifiedIdentifier),
					}
				},
				1,
				sema.Conjunction,
			),
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						fungibleTokenContractLocation,
						fungibleTokenVaultTypeQualifiedIdentifier,
						fungibleTokenContractLocation.TypeID(nil, fungibleTokenVaultTypeQualifiedIdentifier),
					),
				},
			),
		)

		actual := migrate(t, input)

		require.Equal(t, expected, actual)
	})

	t.Run("&FungibleToken.Vault{FungibleToken.Balance} => &{FungibleToken.Vault}", func(t *testing.T) {
		t.Parallel()

		systemContracts := systemcontracts.SystemContractsForChain(chainID)

		const fungibleTokenContractName = "FungibleToken"
		fungibleTokenContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.FungibleToken.Address),
			fungibleTokenContractName,
		)

		const fungibleTokenVaultTypeQualifiedIdentifier = fungibleTokenContractName + ".Vault"
		const fungibleTokenBalanceTypeQualifiedIdentifier = fungibleTokenContractName + ".Balance"

		inputIntersectionType := interpreter.NewIntersectionStaticType(
			nil,
			[]*interpreter.InterfaceStaticType{
				interpreter.NewInterfaceStaticType(
					nil,
					fungibleTokenContractLocation,
					fungibleTokenBalanceTypeQualifiedIdentifier,
					fungibleTokenContractLocation.TypeID(nil, fungibleTokenBalanceTypeQualifiedIdentifier),
				),
			},
		)
		inputIntersectionType.LegacyType = interpreter.NewCompositeStaticType(
			nil,
			fungibleTokenContractLocation,
			fungibleTokenVaultTypeQualifiedIdentifier,
			fungibleTokenContractLocation.TypeID(nil, fungibleTokenVaultTypeQualifiedIdentifier),
		)

		input := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			inputIntersectionType,
		)

		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						fungibleTokenContractLocation,
						fungibleTokenVaultTypeQualifiedIdentifier,
						fungibleTokenContractLocation.TypeID(nil, fungibleTokenVaultTypeQualifiedIdentifier),
					),
				},
			),
		)

		actual := migrate(t, input)

		require.Equal(t, expected, actual)
	})

	t.Run("&NonFungibleToken.NFT => &{NonFungibleToken.NFT}", func(t *testing.T) {
		t.Parallel()

		systemContracts := systemcontracts.SystemContractsForChain(chainID)

		const nonFungibleTokenContractName = "NonFungibleToken"
		nonFungibleTokenContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.NonFungibleToken.Address),
			nonFungibleTokenContractName,
		)

		const nonFungibleTokenNFTTypeQualifiedIdentifier = nonFungibleTokenContractName + ".NFT"

		input := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewCompositeStaticType(
				nil,
				nonFungibleTokenContractLocation,
				nonFungibleTokenNFTTypeQualifiedIdentifier,
				nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenNFTTypeQualifiedIdentifier),
			),
		)

		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						nonFungibleTokenContractLocation,
						nonFungibleTokenNFTTypeQualifiedIdentifier,
						nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenNFTTypeQualifiedIdentifier),
					),
				},
			),
		)

		actual := migrate(t, input)

		require.Equal(t, expected, actual)
	})

	t.Run("&NonFungibleToken.Collection => auth(Withdraw, Owner) &{NonFungibleToken.Collection}", func(t *testing.T) {
		t.Parallel()

		systemContracts := systemcontracts.SystemContractsForChain(chainID)

		const nonFungibleTokenContractName = "NonFungibleToken"
		nonFungibleTokenContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.NonFungibleToken.Address),
			nonFungibleTokenContractName,
		)

		const nonFungibleTokenCollectionTypeQualifiedIdentifier = nonFungibleTokenContractName + ".Collection"

		input := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewCompositeStaticType(
				nil,
				nonFungibleTokenContractLocation,
				nonFungibleTokenCollectionTypeQualifiedIdentifier,
				nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenCollectionTypeQualifiedIdentifier),
			),
		)

		const nonFungibleTokenWithdrawTypeQualifiedIdentifier = nonFungibleTokenContractName + ".Withdraw"
		const nonFungibleTokenOwnerTypeQualifiedIdentifier = nonFungibleTokenContractName + ".Owner"
		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.NewEntitlementSetAuthorization(
				nil,
				func() []common.TypeID {
					return []common.TypeID{
						nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenWithdrawTypeQualifiedIdentifier),
						nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenOwnerTypeQualifiedIdentifier),
					}
				},
				2,
				sema.Conjunction,
			),
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						nonFungibleTokenContractLocation,
						nonFungibleTokenCollectionTypeQualifiedIdentifier,
						nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenCollectionTypeQualifiedIdentifier),
					),
				},
			),
		)

		actual := migrate(t, input)

		require.Equal(t, expected, actual)
	})

	t.Run("&NonFungibleToken.Collection{NonFungibleToken.CollectionPublic} => &{NonFungibleToken.Collection}", func(t *testing.T) {
		t.Parallel()

		systemContracts := systemcontracts.SystemContractsForChain(chainID)

		const nonFungibleTokenContractName = "NonFungibleToken"
		nonFungibleTokenContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.NonFungibleToken.Address),
			nonFungibleTokenContractName,
		)

		const nonFungibleTokenVaultTypeQualifiedIdentifier = nonFungibleTokenContractName + ".Collection"
		const nonFungibleTokenCollectionPublicTypeQualifiedIdentifier = nonFungibleTokenContractName + ".CollectionPublic"

		inputIntersectionType := interpreter.NewIntersectionStaticType(
			nil,
			[]*interpreter.InterfaceStaticType{
				interpreter.NewInterfaceStaticType(
					nil,
					nonFungibleTokenContractLocation,
					nonFungibleTokenCollectionPublicTypeQualifiedIdentifier,
					nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenCollectionPublicTypeQualifiedIdentifier),
				),
			},
		)
		inputIntersectionType.LegacyType = interpreter.NewCompositeStaticType(
			nil,
			nonFungibleTokenContractLocation,
			nonFungibleTokenVaultTypeQualifiedIdentifier,
			nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenVaultTypeQualifiedIdentifier),
		)

		input := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			inputIntersectionType,
		)

		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						nonFungibleTokenContractLocation,
						nonFungibleTokenVaultTypeQualifiedIdentifier,
						nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenVaultTypeQualifiedIdentifier),
					),
				},
			),
		)

		actual := migrate(t, input)

		require.Equal(t, expected, actual)
	})

	t.Run("&{MetadataViews.Resolver} => &{ViewResolver.Resolver}", func(t *testing.T) {
		t.Parallel()

		systemContracts := systemcontracts.SystemContractsForChain(chainID)

		const metadataViewsContractName = "MetadataViews"
		metadataViewsContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.MetadataViews.Address),
			metadataViewsContractName,
		)

		const metadataViewsResolverTypeQualifiedIdentifier = metadataViewsContractName + ".Resolver"

		input := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						metadataViewsContractLocation,
						metadataViewsResolverTypeQualifiedIdentifier,
						metadataViewsContractLocation.TypeID(nil, metadataViewsResolverTypeQualifiedIdentifier),
					),
				},
			),
		)

		const viewResolverContractName = "ViewResolver"
		viewResolverContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.MetadataViews.Address),
			viewResolverContractName,
		)

		const viewResolverResolverTypeQualifiedIdentifier = viewResolverContractName + ".Resolver"

		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						viewResolverContractLocation,
						viewResolverResolverTypeQualifiedIdentifier,
						viewResolverContractLocation.TypeID(nil, viewResolverResolverTypeQualifiedIdentifier),
					),
				},
			),
		)

		actual := migrate(t, input)

		require.Equal(t, expected, actual)
	})

	t.Run("&{MetadataViews.ResolverCollection} => &{ViewResolver.ResolverCollection}", func(t *testing.T) {
		t.Parallel()

		systemContracts := systemcontracts.SystemContractsForChain(chainID)

		const metadataViewsContractName = "MetadataViews"
		metadataViewsContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.MetadataViews.Address),
			metadataViewsContractName,
		)

		const metadataViewsResolverTypeQualifiedIdentifier = metadataViewsContractName + ".ResolverCollection"

		input := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						metadataViewsContractLocation,
						metadataViewsResolverTypeQualifiedIdentifier,
						metadataViewsContractLocation.TypeID(nil, metadataViewsResolverTypeQualifiedIdentifier),
					),
				},
			),
		)

		const viewResolverContractName = "ViewResolver"
		viewResolverContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.MetadataViews.Address),
			viewResolverContractName,
		)

		const viewResolverResolverTypeQualifiedIdentifier = viewResolverContractName + ".ResolverCollection"

		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewIntersectionStaticType(
				nil,
				[]*interpreter.InterfaceStaticType{
					interpreter.NewInterfaceStaticType(
						nil,
						viewResolverContractLocation,
						viewResolverResolverTypeQualifiedIdentifier,
						viewResolverContractLocation.TypeID(nil, viewResolverResolverTypeQualifiedIdentifier),
					),
				},
			),
		)

		actual := migrate(t, input)

		require.Equal(t, expected, actual)
	})

}
