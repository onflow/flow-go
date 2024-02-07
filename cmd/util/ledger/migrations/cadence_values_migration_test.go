package migrations

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/onflow/cadence/runtime/tests/utils"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/flow-go/fvm/environment"

	"github.com/rs/zerolog"

	_ "github.com/glebarez/go-sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

const snapshotPath = "test-data/cadence_values_migration/snapshot_cadence_v0.42.6"

const updatedTestContract = "test-data/cadence_values_migration/test_contract_upgraded.cdc"

const testAccountAddress = "01cf0e2f2f715450"

type writer struct {
	logs []string
}

var _ io.Writer = &writer{}

func (w *writer) Write(p []byte) (n int, err error) {
	w.logs = append(w.logs, string(p))
	return len(p), nil
}

func TestCadenceValuesMigration(t *testing.T) {

	t.Parallel()

	//address, err := common.HexToAddress(testAccountAddress)
	//require.NoError(t, err)

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

	// TODO:
	var stagedContracts []StagedContract

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

	//	// Assert the migrated payloads
	//	rResourceType := checkMigratedPayloads(t, address, newPayloads)
	//
	//	// Check reporters
	//	checkReporters(t, valueMigration, address, rResourceType)

	// Check error logs.
	require.Equal(t, []string{}, logWriter.logs)
}

//func TestCadenceValuesMigrationWithSwappedOrder(t *testing.T) {
//
//	t.Parallel()
//
//	address, err := common.HexToAddress(testAccountAddress)
//	require.NoError(t, err)
//
//	// Get the old payloads
//	payloads, err := util.PayloadsFromEmulatorSnapshot(snapshotPath)
//	require.NoError(t, err)
//
//	// Update contracts to stable cadence.
//	payloads, err = updateContracts(payloads, address)
//	require.NoError(t, err)
//
//	// Migrate
//
//	rwf := &testReportWriterFactory{}
//	capabilityIDs := map[interpreter.AddressPath]interpreter.UInt64Value{}
//
//	// Run link values migration
//	payloads = runLinkMigration(t, address, payloads, capabilityIDs, rwf)
//
//	// Run remaining migrations
//	valueMigration := &CadenceBaseMigrator{
//		name:     "cadence-value-migration",
//		reporter: rwf.ReportWriter("cadence-value-migrator"),
//		valueMigrations: func(
//			inter *interpreter.Interpreter,
//			_ environment.Accounts,
//			reporter *cadenceValueMigrationReporter,
//		) []migrations.ValueMigration {
//			// All cadence migrations except the `capcons.LinkValueMigration`.
//			return []migrations.ValueMigration{
//				&capcons.CapabilityValueMigration{
//					CapabilityIDs: capabilityIDs,
//					Reporter:      reporter,
//				},
//				string_normalization.NewStringNormalizingMigration(),
//				statictypes.NewStaticTypeMigration().
//					WithCompositeTypeConverter(func(staticType *interpreter.CompositeStaticType) interpreter.StaticType {
//						// Returning `nil` indicates the type wasn't converted.
//						return nil
//					}).
//					WithInterfaceTypeConverter(func(staticType *interpreter.InterfaceStaticType) interpreter.StaticType {
//						// Returning `nil` indicates the type wasn't converted.
//						return nil
//					}),
//
//				// Run this at the end
//				entitlements.NewEntitlementsMigration(inter),
//			}
//		},
//	}
//
//	logWriter := &writer{}
//	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)
//	err = valueMigration.InitMigration(logger, nil, 0)
//	require.NoError(t, err)
//
//	ctx := context.Background()
//	newPayloads, err := valueMigration.MigrateAccount(ctx, address, payloads)
//	require.NoError(t, err)
//
//	err = valueMigration.Close()
//	require.NoError(t, err)
//
//	// Assert the migrated payloads
//	rResourceType := checkMigratedPayloads(t, address, newPayloads)
//
//	// Check reporters
//	checkReporters(t, valueMigration, address, rResourceType)
//
//	// Check error logs.
//	// Given entitlement migration was run at the end,
//	// we shouldn't get the deprecated-type errors
//	require.Empty(t, logWriter.logs)
//}

func checkMigratedPayloads(
	t *testing.T,
	address common.Address,
	newPayloads []*ledger.Payload,
) *interpreter.CompositeStaticType {
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

	entitlementType := interpreter.NewCompositeStaticTypeComputeTypeID(
		nil,
		testContractLocation,
		"Test.E",
	)

	entitlementAuthorization := func() interpreter.EntitlementSetAuthorization {
		return interpreter.NewEntitlementSetAuthorization(
			nil,
			func() (entitlements []common.TypeID) {
				return []common.TypeID{entitlementType.TypeID}
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
				interpreter.NewUnmeteredUInt64Value(1),
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
		assert.Fail(t, fmt.Sprintf("%d extra item(s) in expected values", len(expectedValues)))
	}
	return rResourceType
}

func checkAccountID(t *testing.T, mr *migratorRuntime, address common.Address) {
	id := flow.AccountStatusRegisterID(flow.Address(address))
	statusBytes, err := mr.Accounts.GetValue(id)
	require.NoError(t, err)

	accountStatus, err := environment.AccountStatusFromBytes(statusBytes)
	require.NoError(t, err)

	assert.Equal(t, uint64(1), accountStatus.AccountIdCounter())
}

func checkReporters(
	t *testing.T,
	valueMigration *CadenceBaseMigrator,
	address common.Address,
	rResourceType *interpreter.CompositeStaticType,
) {
	reportWriter := valueMigration.reporter.(*testReportWriter)

	reportEntry := func(migration, key string, domain common.PathDomain) cadenceValueMigrationReportEntry {
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

	acctTypedDictKeyMigrationReportEntry := reportEntry(
		"StaticTypeMigration",
		"dictionary_with_account_type_keys",
		common.PathDomainStorage)

	// Order is non-deterministic, so use 'ElementsMatch'.
	assert.ElementsMatch(
		t,
		reportWriter.entries,
		[]any{
			reportEntry("StringNormalizingMigration", "string_value_1", common.PathDomainStorage),
			reportEntry("StringNormalizingMigration", "string_value_2", common.PathDomainStorage),
			reportEntry("StaticTypeMigration", "type_value", common.PathDomainStorage),

			// String keys in dictionary
			reportEntry("StringNormalizingMigration", "dictionary_with_string_keys", common.PathDomainStorage),
			reportEntry("StringNormalizingMigration", "dictionary_with_string_keys", common.PathDomainStorage),

			// Restricted typed keys in dictionary
			reportEntry("StaticTypeMigration", "dictionary_with_restricted_typed_keys", common.PathDomainStorage),
			reportEntry("StaticTypeMigration", "dictionary_with_restricted_typed_keys", common.PathDomainStorage),

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
			capConsPathCapabilityMigration{
				AccountAddress: address,
				AddressPath: interpreter.AddressPath{
					Address: address,
					Path:    interpreter.NewUnmeteredPathValue(common.PathDomainPublic, "linkR"),
				},
				BorrowType: interpreter.NewReferenceStaticType(nil, interpreter.UnauthorizedAccess, rResourceType),
			},
			cadenceValueMigrationReportEntry{
				StorageMapKey: interpreter.Uint64StorageMapKey(1),
				StorageKey: interpreter.NewStorageKey(
					nil,
					address,
					stdlib.CapabilityControllerStorageDomain,
				),
				Migration: "EntitlementsMigration",
			},
			reportEntry("EntitlementsMigration", "capability", common.PathDomainStorage),
			reportEntry("EntitlementsMigration", "linkR", common.PathDomainPublic),

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
			reportEntry("EntitlementsMigration", "dictionary_with_auth_reference_typed_key", common.PathDomainStorage),
			reportEntry("StringNormalizingMigration", "dictionary_with_auth_reference_typed_key", common.PathDomainStorage),
			reportEntry("EntitlementsMigration", "dictionary_with_reference_typed_key", common.PathDomainStorage),
			reportEntry("StringNormalizingMigration", "dictionary_with_reference_typed_key", common.PathDomainStorage),
		},
	)
}

func updateContracts(payloads []*ledger.Payload, address common.Address) ([]*ledger.Payload, error) {
	testContractRegisterId := flow.ContractRegisterID(flow.ConvertAddress(address), "Test")

	updatedContractCode, err := os.ReadFile(updatedTestContract)
	if err != nil {
		return nil, err
	}

	for payloadIndex, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}

		registerID, err := convert.LedgerKeyToRegisterID(key)
		if err != nil {
			return nil, err
		}

		if registerID == testContractRegisterId {
			// change contract code
			payloads[payloadIndex] = ledger.NewPayload(key, updatedContractCode)
		}

	}

	return payloads, nil
}

func runLinkMigration(
	t *testing.T,
	address common.Address,
	payloads []*ledger.Payload,
	capabilityIDs map[interpreter.AddressPath]interpreter.UInt64Value,
	rwf *testReportWriterFactory,
) []*ledger.Payload {
	linkValueMigration := NewCadence1LinkValueMigrator(rwf, capabilityIDs)

	logWriter := &writer{}
	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

	err := linkValueMigration.InitMigration(logger, nil, 0)
	require.NoError(t, err)

	ctx := context.Background()
	payloads, err = linkValueMigration.MigrateAccount(ctx, address, payloads)
	require.NoError(t, err)

	linkMigrationReportWriter := linkValueMigration.reporter.(*testReportWriter)

	// Order is non-deterministic, so use 'ElementsMatch'.
	assert.ElementsMatch(
		t,
		linkMigrationReportWriter.entries,
		[]any{
			capConsLinkMigration{
				AccountAddressPath: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "linkR",
						Domain:     common.PathDomainPublic,
					},
				},
				CapabilityID: 1,
			},
			cadenceValueMigrationReportEntry{
				StorageMapKey: interpreter.StringStorageMapKey("linkR"),
				StorageKey: interpreter.NewStorageKey(
					nil,
					address,
					common.PathDomainPublic.Identifier(),
				),
				Migration: "LinkValueMigration",
			},
		},
	)

	// Check error logs.
	require.Equal(t, []string{}, logWriter.logs)

	return payloads
}

type testReportWriterFactory struct{}

func (_m *testReportWriterFactory) ReportWriter(_ string) reporters.ReportWriter {
	return &testReportWriter{}
}

type testReportWriter struct {
	entries []any
}

var _ reporters.ReportWriter = &testReportWriter{}

func (r *testReportWriter) Write(entry any) {
	r.entries = append(r.entries, entry)
}

func (r *testReportWriter) Close() {}

func TestStaticTypesMigrationRules(t *testing.T) {

	t.Parallel()

	programs := analysis.Programs{}
	config := analysis.NewSimpleConfig(
		analysis.NeedTypes,
		map[common.Location][]byte{
			utils.TestLocation: []byte(`
              access(all)
              contract Test {
                  access(all)
                  resource R {}

                  access(all)
                  resource interface RI {}
              }
            `),
		},
		nil,
		nil,
	)
	err := programs.Load(config, utils.TestLocation)
	require.NoError(t, err)

	csv := strings.NewReader(`"import ""test""",Test.R,{Test.RI}`)

	rules, err := ReadCSVStaticTypeMigrationRules(programs, csv)
	require.NoError(t, err)

	type ruleEntry struct {
		source, target interpreter.StaticType
	}

	var actual []ruleEntry

	for source, target := range rules {
		actual = append(
			actual,
			ruleEntry{
				source: source,
				target: target,
			},
		)
	}

	assert.ElementsMatch(t,
		[]ruleEntry{
			{
				source: &interpreter.CompositeStaticType{
					Location:            utils.TestLocation,
					QualifiedIdentifier: "Test.R",
					TypeID:              "S.test.Test.R",
				},
				target: &interpreter.IntersectionStaticType{
					Types: []*interpreter.InterfaceStaticType{
						{
							Location:            utils.TestLocation,
							QualifiedIdentifier: "Test.RI",
							TypeID:              "S.test.Test.RI",
						},
					},
				},
			},
		},
		actual,
	)
}
