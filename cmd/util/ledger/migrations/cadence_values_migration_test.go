package migrations

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

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
)

const snapshotPath = "test-data/cadence_values_migration/snapshot_cadence_v0.42.6"

const testAccountAddress = "01cf0e2f2f715450"

func TestCadenceValuesMigration(t *testing.T) {

	t.Parallel()

	address, err := common.HexToAddress(testAccountAddress)
	require.NoError(t, err)

	// Get the old payloads

	payloads, err := util.PayloadsFromEmulatorSnapshot(snapshotPath)
	require.NoError(t, err)

	// Migrate

	rwf := &testReportWriterFactory{}
	capabilityIDs := map[interpreter.AddressPath]interpreter.UInt64Value{}

	// Run link values migration
	payloads = runLinkMigration(t, address, payloads, capabilityIDs, rwf)

	// Run remaining migrations
	valueMigration := NewCadenceValueMigrator(rwf, capabilityIDs)

	buf := bytes.Buffer{}
	logger := zerolog.New(&buf).Level(zerolog.ErrorLevel)
	err = valueMigration.InitMigration(logger, nil, 0)
	require.NoError(t, err)

	newPayloads, err := valueMigration.MigrateAccount(nil, address, payloads)
	require.NoError(t, err)

	err = valueMigration.Close()
	require.NoError(t, err)

	// Assert the migrated payloads

	mr, err := newMigratorRuntime(address, newPayloads)
	require.NoError(t, err)

	storageMap := mr.Storage.GetStorageMap(address, common.PathDomainStorage.Identifier(), false)
	require.NotNil(t, storageMap)
	require.Equal(t, 8, int(storageMap.Count()))

	iterator := storageMap.Iterator(mr.Interpreter)

	fullyEntitledAccountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.FullyEntitledAccountReferenceType)

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
					Value: interpreter.NewUnmeteredUInt64Value(4179340454199820288),
					Name:  "uuid",
				},
			},
			address,
		),

		interpreter.NewUnmeteredSomeValueNonCopying(
			interpreter.NewUnmeteredCapabilityValue(
				interpreter.NewUnmeteredUInt64Value(1),
				interpreter.NewAddressValue(nil, address),
				interpreter.NewReferenceStaticType(nil, interpreter.UnauthorizedAccess, rResourceType),
			),
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

	// Check reporters

	reportWriter := valueMigration.reporter.(*testReportWriter)

	// Order is non-deterministic, so use 'ElementsMatch'.
	assert.ElementsMatch(
		t,
		reportWriter.entries,
		[]any{
			cadenceValueMigrationReportEntry{
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "string_value_1",
						Domain:     common.PathDomainStorage,
					},
				},
				Migration: "StringNormalizingMigration",
			},
			cadenceValueMigrationReportEntry{
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "string_value_2",
						Domain:     common.PathDomainStorage,
					},
				},
				Migration: "StringNormalizingMigration",
			},
			cadenceValueMigrationReportEntry{
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "type_value",
						Domain:     common.PathDomainStorage,
					},
				},
				Migration: "AccountTypeMigration",
			},
			cadenceValueMigrationReportEntry{
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "dictionary_with_string_keys",
						Domain:     common.PathDomainStorage,
					},
				},
				Migration: "StringNormalizingMigration",
			},
			cadenceValueMigrationReportEntry{
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "dictionary_with_string_keys",
						Domain:     common.PathDomainStorage,
					},
				},
				Migration: "StringNormalizingMigration",
			},
			cadenceValueMigrationReportEntry{
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "dictionary_with_restricted_typed_keys",
						Domain:     common.PathDomainStorage,
					},
				},
				Migration: "TypeValueMigration",
			},
			cadenceValueMigrationReportEntry{
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "dictionary_with_restricted_typed_keys",
						Domain:     common.PathDomainStorage,
					},
				},
				Migration: "TypeValueMigration",
			},
			cadenceValueMigrationReportEntry{
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "capability",
						Domain:     common.PathDomainStorage,
					},
				},
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
		},
	)

	// Check error logs - should be empty.
	assert.Equal(t, 0, buf.Len())
}

func runLinkMigration(
	t *testing.T,
	address common.Address,
	payloads []*ledger.Payload,
	capabilityIDs map[interpreter.AddressPath]interpreter.UInt64Value,
	rwf *testReportWriterFactory,
) []*ledger.Payload {
	linkValueMigration := NewCadenceLinkValueMigrator(rwf, capabilityIDs)

	linkMigrationBuf := bytes.Buffer{}
	linkMigrationLogger := zerolog.New(&linkMigrationBuf).Level(zerolog.ErrorLevel)

	err := linkValueMigration.InitMigration(linkMigrationLogger, nil, 0)
	require.NoError(t, err)

	payloads, err = linkValueMigration.MigrateAccount(nil, address, payloads)
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
				Address: interpreter.AddressPath{
					Address: address,
					Path: interpreter.PathValue{
						Identifier: "linkR",
						Domain:     common.PathDomainPublic,
					},
				},
				Migration: "LinkValueMigration",
			},
		},
	)

	// Check error logs.
	lines := readLines(&linkMigrationBuf)
	require.Len(t, lines, 2)

	assert.Contains(
		t,
		lines[0],
		fmt.Sprintf(
			"failed to run LinkValueMigration for path {%s /public/flowTokenReceiver}",
			testAccountAddress,
		),
	)

	assert.Contains(
		t,
		lines[1],
		fmt.Sprintf(
			"failed to run LinkValueMigration for path {%s /public/flowTokenBalance}",
			testAccountAddress,
		),
	)
	return payloads
}

func readLines(reader io.Reader) []string {
	lines := make([]string, 0)
	var line []byte
	var err error

	r := bufio.NewReader(reader)
	for {
		line, _, err = r.ReadLine()
		if err != nil {
			break
		}
		lines = append(lines, string(line))
	}
	return lines
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
