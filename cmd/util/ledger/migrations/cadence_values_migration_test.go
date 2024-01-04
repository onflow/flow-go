package migrations

import (
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"

	_ "github.com/glebarez/go-sqlite"
)

const snapshotPath string = "test-data/cadence_values_migration/snapshot"

func TestCadenceValuesMigration(t *testing.T) {

	address, err := common.HexToAddress("01cf0e2f2f715450")
	require.NoError(t, err)

	// Get the old payloads

	payloads := payloadsFromEmulatorSnapshot(t, snapshotPath)

	// Migrate

	rwf := &testReportWriterFactory{}
	valueMigration := NewCadenceValueMigrator(rwf)

	err = valueMigration.InitMigration(zerolog.Nop(), nil, 0)
	require.NoError(t, err)

	newPayloads, err := valueMigration.MigrateAccount(nil, address, payloads)
	require.NoError(t, err)

	// Assert the migrated payloads

	mr, err := newMigratorRuntime(address, newPayloads)
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
}

func payloadsFromEmulatorSnapshot(t *testing.T, snapshotPath string) []*ledger.Payload {
	db, err := sql.Open("sqlite", snapshotPath)
	require.NoError(t, err)

	rows, err := db.Query("SELECT key, value FROM ledger")
	require.NoError(t, err)

	var payloads []*ledger.Payload

	for rows.Next() {
		var hexKey, hexValue string

		err := rows.Scan(&hexKey, &hexValue)
		require.NoError(t, err)

		key, err := hex.DecodeString(hexKey)
		require.NoError(t, err)

		value, err := hex.DecodeString(hexValue)
		require.NoError(t, err)

		registerId := registerIDKeyFromString(string(key))

		// Type loading currently fails, because the core-contracts
		// in the emulator snapshot are not migrated yet.
		// So skip the values that get stored by default, and
		// keep only the explicitly stored values in 'storage' domain.
		if registerId.Key == "public" || registerId.Key == "private" {
			continue
		}

		ledgerKey := convert.RegisterIDToLedgerKey(registerId)

		payloads = append(
			payloads,
			ledger.NewPayload(
				ledgerKey,
				value,
			),
		)
	}

	return payloads
}

// registerIDKeyFromString is the inverse of `flow.RegisterID.String()` method
func registerIDKeyFromString(s string) flow.RegisterID {
	parts := strings.SplitN(s, "/", 2)

	owner := parts[0]
	key := parts[1]

	address, err := common.HexToAddress(owner)
	if err != nil {
		panic(err)
	}

	var decodedKey string

	switch key[0] {
	case '$':
		b := make([]byte, 9)
		b[0] = '$'

		int64Value, err := strconv.ParseInt(key[1:], 10, 64)
		if err != nil {
			panic(err)
		}

		binary.BigEndian.PutUint64(b[1:], uint64(int64Value))

		decodedKey = string(b)
	case '#':
		decoded, err := hex.DecodeString(key[1:])
		if err != nil {
			panic(err)
		}
		decodedKey = string(decoded)
	default:
		panic("Invalid register key")
	}

	return flow.RegisterID{
		Owner: string(address.Bytes()),
		Key:   decodedKey,
	}
}

type testReportWriterFactory struct{}

func (_m *testReportWriterFactory) ReportWriter(dataNamespace string) reporters.ReportWriter {
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
