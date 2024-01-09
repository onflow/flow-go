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
)

const snapshotPath = "test-data/cadence_values_migration/snapshot"

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

	mr, _, err := newMigratorRuntime(address, newPayloads)
	require.NoError(t, err)

	storageMap := mr.Storage.GetStorageMap(address, common.PathDomainStorage.Identifier(), false)
	require.NotNil(t, storageMap)
	require.Equal(t, 4, int(storageMap.Count()))

	iterator := storageMap.Iterator(mr.Interpreter)

	fullyEntitledAccountReferenceType := interpreter.ConvertSemaToStaticType(nil, sema.FullyEntitledAccountReferenceType)

	var values []interpreter.Value
	for key, value := iterator.Next(); key != nil; key, value = iterator.Next() {
		// skip composite values. e.g: FlowToken etc.
		if _, isComposite := value.(*interpreter.CompositeValue); isComposite {
			continue
		}
		values = append(values, value)
	}

	// Order is non-deterministic, so use 'ElementsMatch'.
	assert.ElementsMatch(
		t,
		values,
		[]interpreter.Value{
			// Both string values should be in the normalized form.
			interpreter.NewUnmeteredStringValue("Caf\u00E9"),
			interpreter.NewUnmeteredStringValue("Caf\u00E9"),

			interpreter.NewUnmeteredTypeValue(fullyEntitledAccountReferenceType),
		},
	)

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
						Identifier: "type_value_1",
						Domain:     common.PathDomainStorage,
					},
				},
				Migration: "AccountTypeMigration",
			},
		},
	)

	// Check error logs - should be empty.
	assert.Equal(t, 0, buf.Len())
}

func TestLinkValueMigrationTypeErrors(t *testing.T) {

	t.Parallel()

	address, err := common.HexToAddress(testAccountAddress)
	require.NoError(t, err)

	// Get the old payloads

	payloads, err := util.PayloadsFromEmulatorSnapshot(snapshotPath)
	require.NoError(t, err)

	// Migrate

	rwf := &testReportWriterFactory{}
	capabilityIDs := map[interpreter.AddressPath]interpreter.UInt64Value{}
	valueMigration := NewCadenceLinkValueMigrator(rwf, capabilityIDs)

	buf := bytes.Buffer{}
	logger := zerolog.New(&buf).Level(zerolog.ErrorLevel)

	err = valueMigration.InitMigration(logger, nil, 0)
	require.NoError(t, err)

	_, err = valueMigration.MigrateAccount(nil, address, payloads)
	require.NoError(t, err)

	err = valueMigration.Close()
	require.NoError(t, err)

	// Check reporters. No values should have been migrated.
	reportWriter := valueMigration.reporter.(*testReportWriter)
	assert.Empty(t, reportWriter.entries)

	// Check error logs.
	// Should have two type loading errors for link value migration.
	lines := readLines(&buf)
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
