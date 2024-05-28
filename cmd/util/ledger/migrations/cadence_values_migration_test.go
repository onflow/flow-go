package migrations

import (
	_ "embed"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	migrations2 "github.com/onflow/cadence/migrations"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/systemcontracts"
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

	registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
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
		t.TempDir(),
		rwf,
		Options{
			NWorker:              nWorker,
			ChainID:              chainID,
			EVMContractChange:    evmContractChange,
			BurnerContractChange: burnerContractChange,
			StagedContracts:      stagedContracts,
			VerboseErrorOutput:   true,
		},
	)

	for _, migration := range migrations {
		err = migration.Migrate(registersByAccount)
		require.NoError(
			t,
			err,
			"migration `%s` failed, logs: %v",
			migration.Name,
			logWriter.logs,
		)
	}

	// Assert the migrated payloads
	checkMigratedPayloads(t, address, registersByAccount, chainID)

	// Check reporters
	checkReporters(t, rwf, address)

	// Check error logs.
	require.Empty(t, logWriter.logs)

	checkMigratedState(t, address, registersByAccount, chainID)
}

type migrationVisit struct {
	storageKey    interpreter.StorageKey
	storageMapKey interpreter.StorageMapKey
	value         string
}

type visitMigration struct {
	visits []migrationVisit
}

var _ migrations2.ValueMigration = &visitMigration{}

func (*visitMigration) Name() string {
	return "visit"
}

func (m *visitMigration) Migrate(
	storageKey interpreter.StorageKey,
	storageMapKey interpreter.StorageMapKey,
	value interpreter.Value,
	_ *interpreter.Interpreter,
) (newValue interpreter.Value, err error) {

	m.visits = append(
		m.visits,
		migrationVisit{
			storageKey:    storageKey,
			storageMapKey: storageMapKey,
			value:         value.String(),
		},
	)

	return nil, nil
}

func (*visitMigration) CanSkip(_ interpreter.StaticType) bool {
	return false
}

func (*visitMigration) Domains() map[string]struct{} {
	return nil
}

func checkMigratedState(
	t *testing.T,
	address common.Address,
	registersByAccount *registers.ByAccount,
	chainID flow.ChainID,
) {

	mr, err := NewInterpreterMigrationRuntime(
		registersByAccount,
		chainID,
		InterpreterMigrationRuntimeConfig{},
	)
	require.NoError(t, err)

	validationMigration, err := migrations2.NewStorageMigration(
		mr.Interpreter,
		mr.Storage,
		"validation",
		address,
	)
	require.NoError(t, err)

	visitMigration := &visitMigration{}

	validationMigration.Migrate(
		validationMigration.NewValueMigrationsPathMigrator(nil, visitMigration),
	)

	require.Equal(t,
		[]migrationVisit{
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_string_keys"),
				value:         `"H\u{e9}llo"`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_string_keys"),
				value:         `"Caf\u{e9}"`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_string_keys"),
				value:         `2`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_string_keys"),
				value:         `1`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_string_keys"),
				value:         `{"H\u{e9}llo": 2, "Caf\u{e9}": 1}`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("flowTokenVault"),
				value:         `0.00100000`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("flowTokenVault"),
				value:         `11240984669916758018`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("flowTokenVault"),
				value:         `A.0ae53cb6e3f42a79.FlowToken.Vault(balance: 0.00100000, uuid: 11240984669916758018)`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_auth_reference_typed_key"),
				value:         `Type<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_auth_reference_typed_key"),
				value:         `"auth_ref"`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_auth_reference_typed_key"),
				value:         `{Type<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(): "auth_ref"}`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_reference_typed_key"),
				value:         `Type<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_reference_typed_key"),
				value:         `"non_auth_ref"`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_reference_typed_key"),
				value:         `{Type<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(): "non_auth_ref"}`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("type_value"),
				value:         "Type<auth(Storage, Contracts, Keys, Inbox, Capabilities) &Account>()",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "Type<Account_StorageCapabilities>()",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "Type<Account_Keys>()",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "Type<Account_Contracts>()",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "Type<Account_Inbox>()",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "Type<&Account>()",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         `Type<Account_Capabilities>()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         `Type<Account_AccountCapabilities>()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         `Type<AccountKey>()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         `Type<auth(Storage, Contracts, Keys, Inbox, Capabilities) &Account>()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "4",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "6",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "5",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "7",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "8",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "2",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "3",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "9",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         "1",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				value:         `{Type<Account_StorageCapabilities>(): 4, Type<Account_Keys>(): 6, Type<Account_Contracts>(): 5, Type<Account_Inbox>(): 7, Type<&Account>(): 8, Type<Account_Capabilities>(): 2, Type<Account_AccountCapabilities>(): 3, Type<AccountKey>(): 9, Type<auth(Storage, Contracts, Keys, Inbox, Capabilities) &Account>(): 1}`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("capability"),
				value:         `Capability<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(address: 0x01cf0e2f2f715450, id: 2)`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("capability"),
				value:         `Capability<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(address: 0x01cf0e2f2f715450, id: 2)`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("string_value_1"),
				value:         `"Caf\u{e9}"`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("untyped_capability"),
				value:         `Capability<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(address: 0x01cf0e2f2f715450, id: 2)`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("r"),
				value:         `360287970189639680`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("r"),
				value:         "A.01cf0e2f2f715450.Test.R(uuid: 360287970189639680)",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("string_value_2"),
				value:         `"Caf\u{e9}"`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_restricted_typed_keys"),
				value:         `Type<{A.01cf0e2f2f715450.Test.Bar, A.01cf0e2f2f715450.Test.Foo}>()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_restricted_typed_keys"),
				value:         `Type<{A.01cf0e2f2f715450.Test.Foo, A.01cf0e2f2f715450.Test.Bar, A.01cf0e2f2f715450.Test.Baz}>()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_restricted_typed_keys"),
				value:         `1`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_restricted_typed_keys"),
				value:         `2`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("dictionary_with_restricted_typed_keys"),
				value:         `{Type<{A.01cf0e2f2f715450.Test.Bar, A.01cf0e2f2f715450.Test.Foo}>(): 1, Type<{A.01cf0e2f2f715450.Test.Foo, A.01cf0e2f2f715450.Test.Bar, A.01cf0e2f2f715450.Test.Baz}>(): 2}`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "public", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("flowTokenReceiver"),
				value:         `Capability<&A.0ae53cb6e3f42a79.FlowToken.Vault>(address: 0x01cf0e2f2f715450, id: 1)`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "public", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("linkR"),
				value:         `Capability<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(address: 0x01cf0e2f2f715450, id: 2)`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "public", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("flowTokenBalance"),
				value:         `Capability<&A.0ae53cb6e3f42a79.FlowToken.Vault>(address: 0x01cf0e2f2f715450, id: 3)`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "contract", Address: address},
				storageMapKey: interpreter.StringStorageMapKey("Test"),
				value:         `A.01cf0e2f2f715450.Test()`,
			},
			{
				storageKey:    interpreter.StorageKey{Key: "cap_con", Address: address},
				storageMapKey: interpreter.Uint64StorageMapKey(0x2),
				value:         "StorageCapabilityController(borrowType: Type<auth(A.01cf0e2f2f715450.Test.E) &A.01cf0e2f2f715450.Test.R>(), capabilityID: 2, target: /storage/r)",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "cap_con", Address: address},
				storageMapKey: interpreter.Uint64StorageMapKey(0x1),
				value:         "StorageCapabilityController(borrowType: Type<&A.0ae53cb6e3f42a79.FlowToken.Vault>(), capabilityID: 1, target: /storage/flowTokenVault)",
			},
			{
				storageKey:    interpreter.StorageKey{Key: "cap_con", Address: address},
				storageMapKey: interpreter.Uint64StorageMapKey(0x3),
				value:         "StorageCapabilityController(borrowType: Type<&A.0ae53cb6e3f42a79.FlowToken.Vault>(), capabilityID: 3, target: /storage/flowTokenVault)",
			},
		},
		visitMigration.visits,
	)

}

var flowTokenAddress = func() common.Address {
	address, _ := common.HexToAddress("0ae53cb6e3f42a79")
	return address
}()

func checkMigratedPayloads(
	t *testing.T,
	address common.Address,
	registersByAccount *registers.ByAccount,
	chainID flow.ChainID,
) {
	mr, err := NewInterpreterMigrationRuntime(
		registersByAccount,
		chainID,
		InterpreterMigrationRuntimeConfig{},
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
		interpreter.NewUnmeteredTypeValue(fullyEntitledAccountReferenceType),
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
		interpreter.NewUnmeteredSomeValueNonCopying(
			interpreter.NewUnmeteredCapabilityValue(
				2,
				interpreter.NewAddressValue(nil, address),
				interpreter.NewReferenceStaticType(nil, entitlementAuthorization(), rResourceType),
			),
		),

		// String value should be in the normalized form.
		interpreter.NewUnmeteredStringValue("Caf\u00E9"),

		interpreter.NewUnmeteredCapabilityValue(
			2,
			interpreter.NewAddressValue(nil, address),
			interpreter.NewReferenceStaticType(nil, entitlementAuthorization(), rResourceType),
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

		// String value should be in the normalized form.
		interpreter.NewUnmeteredStringValue("Caf\u00E9"),

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
	}

	require.Equal(t, len(expectedValues), len(values))

	for index, value := range values {
		actualValue := value.(interpreter.EquatableValue)
		expectedValue := expectedValues[index]

		assert.True(t,
			actualValue.Equal(mr.Interpreter, interpreter.EmptyLocationRange, expectedValue),
			"values at index %d are not equal: %s != %s",
			index,
			actualValue,
			expectedValue,
		)
	}
}

func checkAccountID(t *testing.T, mr *InterpreterMigrationRuntime, address common.Address) {
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

	var reporterNames []string
	for reporterName := range rwf.reportWriters {
		reporterNames = append(reporterNames, reporterName)
	}
	sort.Strings(reporterNames)

	var accountReportEntries []valueMigrationReportEntry

	for _, reporterName := range reporterNames {
		reportWriter := rwf.reportWriters[reporterName]

		for _, entry := range reportWriter.entries {

			e, ok := entry.(valueMigrationReportEntry)
			if !ok || e.accountAddress() != address {
				continue
			}

			accountReportEntries = append(accountReportEntries, e)
		}
	}

	assert.Equal(
		t,
		[]valueMigrationReportEntry{
			capabilityMigrationEntry{
				AccountAddress: address,
				AddressPath: interpreter.AddressPath{
					Address: address,
					Path:    interpreter.PathValue{Identifier: "linkR", Domain: 0x3},
				},
				BorrowType: interpreter.NewReferenceStaticType(
					nil,
					entitlementAuthorization(),
					rResourceType,
				),
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("capability"),
				Migration:     "CapabilityValueMigration",
			},
			capabilityMigrationEntry{
				AccountAddress: address,
				AddressPath: interpreter.AddressPath{
					Address: address, Path: interpreter.PathValue{Identifier: "linkR", Domain: 0x3},
				},
				BorrowType: interpreter.NewReferenceStaticType(
					nil,
					entitlementAuthorization(),
					rResourceType,
				),
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("untyped_capability"),
				Migration:     "CapabilityValueMigration",
			},
			linkMigrationEntry{
				AccountAddressPath: interpreter.AddressPath{
					Address: address,
					Path:    interpreter.PathValue{Identifier: "flowTokenReceiver", Domain: 0x3},
				},
				CapabilityID: 0x1,
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "public", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("flowTokenReceiver"),
				Migration:     "LinkValueMigration",
			},
			linkMigrationEntry{
				AccountAddressPath: interpreter.AddressPath{
					Address: address,
					Path:    interpreter.PathValue{Identifier: "linkR", Domain: 0x3},
				},
				CapabilityID: 0x2,
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "public", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("linkR"),
				Migration:     "LinkValueMigration",
			},
			linkMigrationEntry{
				AccountAddressPath: interpreter.AddressPath{
					Address: address,
					Path:    interpreter.PathValue{Identifier: "flowTokenBalance", Domain: 0x3},
				},
				CapabilityID: 0x3,
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "public", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("flowTokenBalance"),
				Migration:     "LinkValueMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_string_keys"),
				Migration:     "StringNormalizingMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_auth_reference_typed_key"),
				Migration:     "EntitlementsMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_reference_typed_key"),
				Migration:     "EntitlementsMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("type_value"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_account_type_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("capability"),
				Migration:     "EntitlementsMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("string_value_1"),
				Migration:     "StringNormalizingMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_restricted_typed_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "storage", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("dictionary_with_restricted_typed_keys"),
				Migration:     "StaticTypeMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "public", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("flowTokenReceiver"),
				Migration:     "EntitlementsMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "public", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("linkR"),
				Migration:     "EntitlementsMigration",
			},
			cadenceValueMigrationEntry{
				StorageKey:    interpreter.StorageKey{Key: "public", Address: address},
				StorageMapKey: interpreter.StringStorageMapKey("flowTokenBalance"),
				Migration:     "EntitlementsMigration",
			},
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

	registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	migrations := NewCadence1Migrations(
		logger,
		t.TempDir(),
		rwf,
		Options{
			NWorker:              nWorker,
			ChainID:              chainID,
			EVMContractChange:    evmContractChange,
			BurnerContractChange: burnerContractChange,
			VerboseErrorOutput:   true,
		},
	)

	for _, migration := range migrations {
		err = migration.Migrate(registersByAccount)
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

	registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	runtime, err := NewInterpreterMigrationRuntime(
		registersByAccount,
		chainID,
		InterpreterMigrationRuntimeConfig{},
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
		1,
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

	err = storage.NondeterministicCommit(runtime.Interpreter, false)
	require.NoError(t, err)

	// finalize the transaction
	result, err := runtime.TransactionState.FinalizeMainTransaction()
	require.NoError(t, err)

	// Merge the changes into the registers

	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(testAddress): {},
	}

	err = registers.ApplyChanges(
		registersByAccount,
		result.WriteSet,
		expectedAddresses,
		logger,
	)
	require.NoError(t, err)

	// Set the code for the old program

	err = registersByAccount.Set(
		string(testAddress[:]),
		flow.ContractKey(contractName),
		[]byte(`pub contract C {}`),
	)
	require.NoError(t, err)

	// Migrate

	// TODO: EVM contract is not deployed in snapshot yet, so can't update it
	const evmContractChange = EVMContractChangeNone

	const burnerContractChange = BurnerContractChangeUpdate

	migrations := NewCadence1Migrations(
		logger,
		t.TempDir(),
		rwf,
		Options{
			NWorker:              nWorker,
			ChainID:              chainID,
			EVMContractChange:    evmContractChange,
			BurnerContractChange: burnerContractChange,
			VerboseErrorOutput:   true,
		},
	)

	for _, migration := range migrations {
		err = migration.Migrate(registersByAccount)
		require.NoError(
			t,
			err,
			"migration `%s` failed, logs: %v",
			migration.Name,
			logWriter.logs,
		)
	}

	reporter := rwf.reportWriters[contractCheckingReporterName]
	require.NotNil(t, reporter)

	var messages []string

	for _, entry := range reporter.entries {
		if errorEntry, isErrorEntry := entry.(contractCheckingFailure); isErrorEntry {
			messages = append(messages, errorEntry.Error)
			break
		}
	}

	require.Len(t, messages, 1)

	assert.Contains(t, messages[0], "`pub` is no longer a valid access keyword")
	assert.NotContains(t, messages[0], "runtime/debug.Stack()")
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

		registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
		require.NoError(t, err)

		runtime, err := NewInterpreterMigrationRuntime(
			registersByAccount,
			chainID,
			InterpreterMigrationRuntimeConfig{},
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
			1,
			interpreter.AddressValue(testAddress),
			staticType,
		)

		storageMap.WriteValue(
			runtime.Interpreter,
			storageMapKey,
			capabilityValue,
		)

		err = storage.NondeterministicCommit(runtime.Interpreter, false)
		require.NoError(t, err)

		// finalize the transaction
		result, err := runtime.TransactionState.FinalizeMainTransaction()
		require.NoError(t, err)

		// Merge the changes to the original payloads.

		expectedAddresses := map[flow.Address]struct{}{
			flow.Address(testAddress): {},
		}

		err = registers.ApplyChanges(
			registersByAccount,
			result.WriteSet,
			expectedAddresses,
			logger,
		)
		require.NoError(t, err)

		// Migrate

		// TODO: EVM contract is not deployed in snapshot yet, so can't update it
		const evmContractChange = EVMContractChangeNone

		const burnerContractChange = BurnerContractChangeUpdate

		migrations := NewCadence1Migrations(
			logger,
			t.TempDir(),
			rwf,
			Options{
				NWorker:              nWorker,
				ChainID:              chainID,
				EVMContractChange:    evmContractChange,
				BurnerContractChange: burnerContractChange,
				VerboseErrorOutput:   true,
			},
		)

		for _, migration := range migrations {
			err = migration.Migrate(registersByAccount)
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

		mr, err := NewInterpreterMigrationRuntime(
			registersByAccount,
			chainID,
			InterpreterMigrationRuntimeConfig{},
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

	t.Run("&NonFungibleToken.Collection => auth(Withdraw) &{NonFungibleToken.Collection}", func(t *testing.T) {
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
		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.NewEntitlementSetAuthorization(
				nil,
				func() []common.TypeID {
					return []common.TypeID{
						nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenWithdrawTypeQualifiedIdentifier),
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

	t.Run("&NonFungibleToken.Provider => auth(Withdraw) &{NonFungibleToken.Provider}", func(t *testing.T) {
		t.Parallel()

		systemContracts := systemcontracts.SystemContractsForChain(chainID)

		const nonFungibleTokenContractName = "NonFungibleToken"
		nonFungibleTokenContractLocation := common.NewAddressLocation(
			nil,
			common.Address(systemContracts.NonFungibleToken.Address),
			nonFungibleTokenContractName,
		)

		const nonFungibleTokenProviderTypeQualifiedIdentifier = nonFungibleTokenContractName + ".Provider"

		input := interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.NewInterfaceStaticType(
				nil,
				nonFungibleTokenContractLocation,
				nonFungibleTokenProviderTypeQualifiedIdentifier,
				nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenProviderTypeQualifiedIdentifier),
			),
		)

		const nonFungibleTokenWithdrawTypeQualifiedIdentifier = nonFungibleTokenContractName + ".Withdraw"
		expected := interpreter.NewReferenceStaticType(
			nil,
			interpreter.NewEntitlementSetAuthorization(
				nil,
				func() []common.TypeID {
					return []common.TypeID{
						nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenWithdrawTypeQualifiedIdentifier),
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
						nonFungibleTokenContractLocation,
						nonFungibleTokenProviderTypeQualifiedIdentifier,
						nonFungibleTokenContractLocation.TypeID(nil, nonFungibleTokenProviderTypeQualifiedIdentifier),
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

func TestDictionaryKeyConflictEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := dictionaryKeyConflictEntry{
		AddressPath: interpreter.AddressPath{
			Address: common.MustBytesToAddress([]byte{0x1}),
			Path: interpreter.PathValue{
				Domain:     common.PathDomainPublic,
				Identifier: "test",
			},
		},
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "dictionary-key-conflict",
          "account_address": "0x0000000000000001",
          "path": "/public/test"
        }`,
		string(actual),
	)
}

func TestLinkMissingTargetEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := linkMissingTargetEntry{
		AddressPath: interpreter.AddressPath{
			Address: common.MustBytesToAddress([]byte{0x1}),
			Path: interpreter.PathValue{
				Domain:     common.PathDomainPublic,
				Identifier: "test",
			},
		},
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "link-missing-target",
          "account_address": "0x0000000000000001",
          "path": "/public/test"
        }`,
		string(actual),
	)
}

func TestCapabilityMissingCapabilityIDEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := capabilityMissingCapabilityIDEntry{
		AccountAddress: common.MustBytesToAddress([]byte{0x2}),
		AddressPath: interpreter.AddressPath{
			Address: common.MustBytesToAddress([]byte{0x1}),
			Path: interpreter.PathValue{
				Domain:     common.PathDomainPublic,
				Identifier: "test",
			},
		},
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "capability-missing-capability-id",
          "account_address": "0x0000000000000002",
          "address": "0x0000000000000001",
          "path": "/public/test"
        }`,
		string(actual),
	)
}

func TestCapabilityMigrationEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := capabilityMigrationEntry{
		AccountAddress: common.MustBytesToAddress([]byte{0x2}),
		AddressPath: interpreter.AddressPath{
			Address: common.MustBytesToAddress([]byte{0x1}),
			Path: interpreter.PathValue{
				Domain:     common.PathDomainPublic,
				Identifier: "test",
			},
		},
		BorrowType: interpreter.NewReferenceStaticType(
			nil,
			interpreter.UnauthorizedAccess,
			interpreter.PrimitiveStaticTypeInt,
		),
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "capability-migration-success",
          "account_address": "0x0000000000000002",
          "address": "0x0000000000000001",
          "path": "/public/test",
          "borrow_type": "&Int"
        }`,
		string(actual),
	)
}

func TestLinkMigrationEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := linkMigrationEntry{
		AccountAddressPath: interpreter.AddressPath{
			Address: common.MustBytesToAddress([]byte{0x1}),
			Path: interpreter.PathValue{
				Domain:     common.PathDomainPublic,
				Identifier: "test",
			},
		},
		CapabilityID: 42,
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "link-migration-success",
          "account_address": "0x0000000000000001",
          "path": "/public/test",
          "capability_id": 42
        }`,
		string(actual),
	)
}

func TestCadenceValueMigrationFailureEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := cadenceValueMigrationFailureEntry{
		StorageKey: interpreter.StorageKey{
			Address: common.MustBytesToAddress([]byte{0x1}),
			Key:     "storage",
		},
		StorageMapKey: interpreter.StringStorageMapKey("test"),
		Migration:     "test-migration",
		Message:       "unknown",
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "cadence-value-migration-failure",
          "account_address": "0x0000000000000001",
          "domain": "storage",
          "key": "test",
          "migration": "test-migration",
          "message": "unknown"
        }`,
		string(actual),
	)
}

func TestCadenceValueMigrationEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := cadenceValueMigrationEntry{
		StorageKey: interpreter.StorageKey{
			Address: common.MustBytesToAddress([]byte{0x1}),
			Key:     "storage",
		},
		StorageMapKey: interpreter.StringStorageMapKey("test"),
		Migration:     "test-migration",
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "cadence-value-migration-success",
          "account_address": "0x0000000000000001",
          "domain": "storage",
          "key": "test",
          "migration": "test-migration"
        }`,
		string(actual),
	)
}
