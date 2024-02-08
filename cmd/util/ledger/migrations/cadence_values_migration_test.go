package migrations

import (
	_ "embed"
	"fmt"
	"io"
	"testing"

	"github.com/onflow/flow-go/fvm/environment"

	"github.com/rs/zerolog"

	_ "github.com/glebarez/go-sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
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

	// TODO: >1 breaks
	const nWorker = 1

	const chainID = flow.Emulator
	// TODO: EVM contract is not deployed in snapshot yet, so can't update it
	const evmContractChange = EVMContractChangeNone

	stagedContracts := []StagedContract{
		{
			Contract: Contract{
				name: "Test",
				code: testContractUpgraded,
			},
			address: address,
		},
	}

	migrations := NewCadence1Migrations(
		logger,
		rwf,
		nWorker,
		chainID,
		evmContractChange,
		stagedContracts,
	)

	for _, migration := range migrations {
		payloads, err = migration(payloads)
		require.NoError(t, err)
	}

	// Assert the migrated payloads
	checkMigratedPayloads(t, address, payloads)

	// Check reporters
	checkReporters(t, rwf, address)

	// Check error logs.
	require.Nil(t, logWriter.logs)
}

// TODO:
//func TestCadenceValuesMigrationWithSwappedOrder(t *testing.T) {

func checkMigratedPayloads(
	t *testing.T,
	address common.Address,
	newPayloads []*ledger.Payload,
) {
	mr, err := newMigratorRuntime(
		address,
		newPayloads,
		util.RuntimeInterfaceConfig{},
	)
	require.NoError(t, err)

	storageMap := mr.Storage.GetStorageMap(address, common.PathDomainStorage.Identifier(), false)
	require.NotNil(t, storageMap)
	require.Equal(t, 11, int(storageMap.Count()))

	iterator := storageMap.Iterator(mr.Interpreter)

	// Check whether the account ID is properly incremented.
	checkAccountID(t, mr, address)

	fullyEntitledAccountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.FullyEntitledAccountReferenceType)
	accountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.AccountReferenceType)

	var values []interpreter.Value
	for key, value := iterator.Next(); key != nil; key, value = iterator.Next() {
		identifier := string(key.(interpreter.StringAtreeValue))
		if identifier == "flowTokenVault" || identifier == "flowTokenReceiver" {
			continue
		}
		values = append(values, value)
	}

	testContractLocation := common.NewAddressLocation(
		nil,
		address,
		"Test",
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
					Value: interpreter.NewUnmeteredUInt64Value(1369094286720630784),
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
				"StringNormalizingMigration",
				"string_value_2",
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
				"StringNormalizingMigration",
				"dictionary_with_auth_reference_typed_key",
				common.PathDomainStorage,
			),
			newCadenceValueMigrationReportEntry(
				"EntitlementsMigration",
				"dictionary_with_reference_typed_key",
				common.PathDomainStorage,
			),
			newCadenceValueMigrationReportEntry(
				"StringNormalizingMigration",
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
	reportWriters map[string]*testReportWriter
}

func (f *testReportWriterFactory) ReportWriter(dataNamespace string) reporters.ReportWriter {
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
	entries []any
}

var _ reporters.ReportWriter = &testReportWriter{}

func (r *testReportWriter) Write(entry any) {
	r.entries = append(r.entries, entry)
}

func (r *testReportWriter) Close() {}
